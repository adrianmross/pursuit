package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/impex"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

type OptString struct {
	Set   bool
	Null  bool
	Value string
}

func (o *OptString) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}

type ApplicationInput struct {
	CompanyID     string   `json:"company_id"`
	Company       string   `json:"company"`
	Website       string   `json:"website"`
	RoleID        string   `json:"role_id"`
	Title         string   `json:"title"`
	Location      string   `json:"location"`
	Salary        string   `json:"salary"`
	Fit           string   `json:"fit"`
	PostingURL    string   `json:"posting_url"`
	Source        string   `json:"source"`
	JobType       string   `json:"job_type"`
	Status        string   `json:"status"`
	AppliedDate   string   `json:"applied_date"`
	NextStep      string   `json:"next_step"`
	Notes         string   `json:"notes"`
	ResumeVersion string   `json:"resume_version"`
	ResumeFileRef string   `json:"resume_file_ref"`
	ContactEmail  string   `json:"contact_email"`
	ExternalID    string   `json:"external_id"`
	Tags          []string `json:"tags"`
	OccurredAt    string   `json:"occurred_at"`
}

type ApplicationPatch struct {
	Version       int      `json:"version"`
	Status        *string  `json:"status"`
	AppliedDate   OptString `json:"applied_date"`
	NextStep      *string  `json:"next_step"`
	Notes         *string  `json:"notes"`
	ResumeVersion *string  `json:"resume_version"`
	ResumeFileRef *string  `json:"resume_file_ref"`
	ContactEmail  *string  `json:"contact_email"`
	Title         *string  `json:"title"`
	Location      *string  `json:"location"`
	Salary        *string  `json:"salary"`
	Fit           *string  `json:"fit"`
	PostingURL    *string  `json:"posting_url"`
	Source        *string  `json:"source"`
	JobType       *string  `json:"job_type"`
}

type TransitionInput struct {
	Version    int    `json:"version"`
	Status     string `json:"status"`
	NextStep   *string `json:"next_step"`
	Note       string `json:"note"`
	OccurredAt string `json:"occurred_at"`
}

type EventInput struct {
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	OccurredAt string `json:"occurred_at"`
}

type TagInput struct {
	Name string `json:"name"`
}

type AttachTagInput struct {
	Name  string `json:"name"`
	TagID string `json:"tag_id"`
}

func (s *Service) CreateApplication(ctx context.Context, in ApplicationInput) (domain.Application, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Application{}, err
	}
	var out domain.Application
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		app, err := s.insertApplication(ctx, a, in, s.now(), "")
		if err != nil {
			return err
		}
		out = app
		return nil
	})
	return out, err
}

func (s *Service) insertApplication(ctx context.Context, a domain.Actor, in ApplicationInput, created time.Time, eventKind string) (domain.Application, error) {
	if strings.TrimSpace(in.Status) == "" {
		in.Status = domain.StatusSaved
	}
	status, err := domain.ParseStatus(in.Status)
	if err != nil {
		return domain.Application{}, err
	}
	date, err := domain.ParseDate(in.AppliedDate)
	if err != nil {
		return domain.Application{}, err
	}
	nextStep, err := domain.BoundText("next_step", in.NextStep, 2000)
	if err != nil {
		return domain.Application{}, err
	}
	notes, err := domain.BoundText("notes", in.Notes, 20000)
	if err != nil {
		return domain.Application{}, err
	}
	resume, err := domain.BoundText("resume_version", in.ResumeVersion, 200)
	if err != nil {
		return domain.Application{}, err
	}
	fileRef, err := domain.BoundText("resume_file_ref", in.ResumeFileRef, 1000)
	if err != nil {
		return domain.Application{}, err
	}
	email, err := domain.ParseEmail(in.ContactEmail)
	if err != nil {
		return domain.Application{}, err
	}
	ext, err := domain.BoundText("external_id", in.ExternalID, 100)
	if err != nil {
		return domain.Application{}, err
	}
	var role domain.Role
	if in.RoleID != "" {
		role, err = s.DB.GetRole(ctx, a.UserID, in.RoleID)
		if err != nil {
			return domain.Application{}, err
		}
	} else {
		company, _, err := s.ensureCompany(ctx, a, in.CompanyID, in.Company, in.Website)
		if err != nil {
			return domain.Application{}, err
		}
		if company.Website == "" {
			if origin := domain.Origin(in.PostingURL); origin != "" {
				company.Website = origin
				company.UpdatedAt = s.now()
				if err := s.DB.UpdateCompany(ctx, company); err == nil {
					company.Version++
				}
			}
		}
		role, err = s.reuseOrCreateRole(ctx, a, company, RoleInput{
			Title: in.Title, Location: in.Location, Salary: in.Salary, Fit: in.Fit,
			PostingURL: in.PostingURL, Source: in.Source, JobType: in.JobType,
		})
		if err != nil {
			return domain.Application{}, err
		}
	}
	var applied *string
	if date != "" {
		applied = &date
	}
	if err := s.guardTransition(ctx, a, "", role.Company, "", status); err != nil {
		return domain.Application{}, err
	}
	app := domain.Application{
		ID: s.NewID(), UserID: a.UserID, RoleID: role.ID, Status: status, AppliedDate: applied,
		NextStep: nextStep, Notes: notes, ResumeVersion: resume, ResumeFileRef: fileRef,
		ContactEmail: email, ExternalID: ext, Version: 1, CreatedAt: created, UpdatedAt: created, Tags: []string{},
	}
	if err := s.DB.CreateApplication(ctx, app); err != nil {
		return domain.Application{}, err
	}
	for _, name := range in.Tags {
		if _, _, err := s.attachByName(ctx, a, app.ID, name, false); err != nil {
			return domain.Application{}, err
		}
	}
	occurred := created
	if strings.TrimSpace(in.OccurredAt) != "" {
		occurred, err = domain.ParseTime(in.OccurredAt)
		if err != nil {
			return domain.Application{}, domain.Validation("invalid occurred_at").WithHint("use RFC3339 or YYYY-MM-DD")
		}
	} else if date != "" {
		occurred, _ = domain.ParseTime(date)
	}
	kind := eventKind
	title := "Application created"
	if kind == "" {
		kind = domain.EventStatusChange
	}
	if kind == domain.EventImported {
		title = "Imported from CSV"
	}
	if err := s.recordEvent(ctx, domain.Event{
		ID: s.NewID(), UserID: a.UserID, ApplicationID: app.ID, Kind: kind, ToStatus: status,
		Title: title, Body: notes, OccurredAt: occurred, CreatedAt: s.now(),
		Actor: domain.ActorRef{TokenID: a.TokenID, Name: a.Name},
	}); err != nil {
		return domain.Application{}, err
	}
	if err := s.audit(ctx, a, "application.create", "application", app.ID, status+" "+role.Title, map[string]any{"company": role.Company, "external_id": ext}); err != nil {
		return domain.Application{}, err
	}
	return s.DB.GetApplication(ctx, a.UserID, app.ID)
}

func (s *Service) reuseOrCreateRole(ctx context.Context, a domain.Actor, company domain.Company, in RoleInput) (domain.Role, error) {
	url, err := domain.ParseURL(in.PostingURL)
	if err != nil {
		return domain.Role{}, err
	}
	if url != "" {
		if role, err := s.DB.FindRoleByURL(ctx, a.UserID, company.ID, url); err == nil {
			return role, nil
		} else if !domain.IsCode(err, "not_found") {
			return domain.Role{}, err
		}
	}
	if strings.TrimSpace(in.Title) != "" && url == "" {
		if role, err := s.DB.FindRoleByTitle(ctx, a.UserID, company.ID, strings.TrimSpace(in.Title)); err == nil {
			return role, nil
		} else if !domain.IsCode(err, "not_found") {
			return domain.Role{}, err
		}
	}
	return s.createRole(ctx, a, company, in)
}

func (s *Service) createRole(ctx context.Context, a domain.Actor, company domain.Company, in RoleInput) (domain.Role, error) {
	role, err := s.buildRole(a, company, in, s.now())
	if err != nil {
		return domain.Role{}, err
	}
	if err := s.DB.CreateRole(ctx, role); err != nil {
		return domain.Role{}, err
	}
	if err := s.audit(ctx, a, "role.create", "role", role.ID, role.Title+" at "+company.Name, nil); err != nil {
		return domain.Role{}, err
	}
	return s.DB.GetRole(ctx, a.UserID, role.ID)
}

func (s *Service) ensureCompany(ctx context.Context, a domain.Actor, id, name, website string) (domain.Company, bool, error) {
	if id != "" {
		c, err := s.DB.GetCompany(ctx, a.UserID, id)
		return c, false, err
	}
	name, err := domain.RequireText("company", name, 200)
	if err != nil {
		return domain.Company{}, false, err
	}
	if c, err := s.DB.GetCompanyByName(ctx, a.UserID, name); err == nil {
		return c, false, nil
	} else if !domain.IsCode(err, "not_found") {
		return domain.Company{}, false, err
	}
	c, err := s.newCompany(a, name, website, "")
	if err != nil {
		return domain.Company{}, false, err
	}
	if err := s.DB.CreateCompany(ctx, c); err != nil {
		return domain.Company{}, false, err
	}
	if err := s.audit(ctx, a, "company.create", "company", c.ID, "created "+c.Name, nil); err != nil {
		return domain.Company{}, false, err
	}
	return c, true, nil
}

func (s *Service) GetApplication(ctx context.Context, id string) (domain.Application, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return domain.Application{}, err
	}
	return s.DB.GetApplication(ctx, a.UserID, id)
}

func (s *Service) ListApplications(ctx context.Context, q domain.ListQuery) (domain.Page[domain.Application], error) {
	if len(q.Status) > 0 {
		for i, st := range q.Status {
			parsed, err := domain.ParseStatus(st)
			if err != nil {
				return domain.Page[domain.Application]{}, err
			}
			q.Status[i] = parsed
		}
	}
	if q.Fit != "" {
		fit, err := domain.ParseFit(q.Fit)
		if err != nil {
			return domain.Page[domain.Application]{}, err
		}
		q.Fit = fit
	}
	if q.Source != "" {
		src, err := domain.ParseSource(q.Source)
		if err != nil {
			return domain.Page[domain.Application]{}, err
		}
		q.Source = src
	}
	return listPage(s, ctx, false, "application", q, s.DB.ListApplications, func(a domain.Application) string {
		return appSortKey(a, q.Sort)
	}, func(a domain.Application) string { return a.ID })
}

func appSortKey(a domain.Application, sort string) string {
	switch strings.TrimPrefix(sort, "-") {
	case "created_at":
		return domain.FormatTime(a.CreatedAt)
	case "applied_date":
		return a.AppliedDateValue()
	case "company":
		return a.Company
	case "status":
		return a.Status
	case "title":
		return a.Title
	default:
		return domain.FormatTime(a.UpdatedAt)
	}
}

func (s *Service) UpdateApplication(ctx context.Context, id string, p ApplicationPatch) (domain.Application, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Application{}, err
	}
	if p.Version < 1 {
		return domain.Application{}, versionRequired("application")
	}
	var out domain.Application
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		cur, err := s.DB.GetApplication(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		if cur.Version != p.Version {
			return domain.VersionConflict("application", cur.Version)
		}
		next := cur
		changed := false
		var from string
		if p.Status != nil {
			st, err := domain.ParseStatus(*p.Status)
			if err != nil {
				return err
			}
			if st != cur.Shown() {
				if err := s.guardTransition(ctx, a, cur.ID, cur.Company, cur.Status, st); err != nil {
					return err
				}
				from = cur.Shown()
				next.Status = st
				changed = true
			}
		}
		if p.AppliedDate.Set {
			if p.AppliedDate.Null || strings.TrimSpace(p.AppliedDate.Value) == "" {
				if cur.AppliedDate != nil {
					next.AppliedDate = nil
					changed = true
				}
			} else {
				d, err := domain.ParseDate(p.AppliedDate.Value)
				if err != nil {
					return err
				}
				if cur.AppliedDate == nil || *cur.AppliedDate != d {
					next.AppliedDate = &d
					changed = true
				}
			}
		}
		if p.NextStep != nil && strings.TrimSpace(*p.NextStep) != cur.NextStep {
			n, err := domain.BoundText("next_step", *p.NextStep, 2000)
			if err != nil {
				return err
			}
			next.NextStep = n
			changed = true
		}
		if p.Notes != nil && *p.Notes != cur.Notes {
			n, err := domain.BoundText("notes", *p.Notes, 20000)
			if err != nil {
				return err
			}
			next.Notes = n
			changed = true
		}
		if p.ResumeVersion != nil && strings.TrimSpace(*p.ResumeVersion) != cur.ResumeVersion {
			n, err := domain.BoundText("resume_version", *p.ResumeVersion, 200)
			if err != nil {
				return err
			}
			next.ResumeVersion = n
			changed = true
		}
		if p.ResumeFileRef != nil && strings.TrimSpace(*p.ResumeFileRef) != cur.ResumeFileRef {
			n, err := domain.BoundText("resume_file_ref", *p.ResumeFileRef, 1000)
			if err != nil {
				return err
			}
			next.ResumeFileRef = n
			changed = true
		}
		if p.ContactEmail != nil && strings.TrimSpace(*p.ContactEmail) != cur.ContactEmail {
			n, err := domain.ParseEmail(*p.ContactEmail)
			if err != nil {
				return err
			}
			next.ContactEmail = n
			changed = true
		}
		roleChanged, err := s.patchLinkedRole(ctx, a, &cur, p)
		if err != nil {
			return err
		}
		if !changed && !roleChanged {
			out = cur
			return nil
		}
		next.UpdatedAt = s.now()
		if err := s.DB.UpdateApplication(ctx, next); err != nil {
			return err
		}
		if from != "" {
			if err := s.recordEvent(ctx, domain.Event{
				ID: s.NewID(), UserID: a.UserID, ApplicationID: id, Kind: domain.EventStatusChange,
				FromStatus: from, ToStatus: next.Status, Title: "Status changed", OccurredAt: s.now(), CreatedAt: s.now(),
				Actor: domain.ActorRef{TokenID: a.TokenID, Name: a.Name},
			}); err != nil {
				return err
			}
		}
		action := "application.update"
		if from != "" {
			action = "application.transition"
		}
		if err := s.audit(ctx, a, action, "application", id, next.Status+" "+cur.Title, map[string]any{"from": from, "to": next.Status}); err != nil {
			return err
		}
		out, err = s.DB.GetApplication(ctx, a.UserID, id)
		return err
	})
	return out, err
}

func (s *Service) patchLinkedRole(ctx context.Context, a domain.Actor, app *domain.Application, p ApplicationPatch) (bool, error) {
	rp := RolePatch{Title: p.Title, Location: p.Location, Salary: p.Salary, Fit: p.Fit, PostingURL: p.PostingURL, Source: p.Source, JobType: p.JobType}
	if rp.Title == nil && rp.Location == nil && rp.Salary == nil && rp.Fit == nil && rp.PostingURL == nil && rp.Source == nil && rp.JobType == nil {
		return false, nil
	}
	role, err := s.DB.GetRole(ctx, a.UserID, app.RoleID)
	if err != nil {
		return false, err
	}
	next, changed, err := applyRolePatch(role, rp)
	if err != nil || !changed {
		return false, err
	}
	next.UpdatedAt = s.now()
	if err := s.DB.UpdateRole(ctx, next); err != nil {
		return false, err
	}
	return true, s.audit(ctx, a, "role.update", "role", role.ID, "updated "+next.Title, map[string]any{"via": "application"})
}

func (s *Service) Transition(ctx context.Context, id string, in TransitionInput) (domain.Application, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Application{}, err
	}
	if in.Version < 1 {
		return domain.Application{}, versionRequired("application")
	}
	status, err := domain.ParseStatus(in.Status)
	if err != nil {
		return domain.Application{}, err
	}
	var occurred time.Time
	if strings.TrimSpace(in.OccurredAt) == "" {
		occurred = s.now()
	} else {
		occurred, err = domain.ParseTime(in.OccurredAt)
		if err != nil {
			return domain.Application{}, domain.Validation("invalid occurred_at").WithHint("use RFC3339 or YYYY-MM-DD")
		}
	}
	note, err := domain.BoundText("note", in.Note, 20000)
	if err != nil {
		return domain.Application{}, err
	}
	var out domain.Application
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		cur, err := s.DB.GetApplication(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		if cur.Version != in.Version {
			return domain.VersionConflict("application", cur.Version)
		}
		// Requesting the stage an outcome sits on clears the outcome.
		if cur.Shown() == status {
			return domain.Validation("status is unchanged").WithHint("choose a different status, or update next_step with apps update")
		}
		if err := s.guardTransition(ctx, a, cur.ID, cur.Company, cur.Status, status); err != nil {
			return err
		}
		next := cur
		next.Status = status
		if in.NextStep != nil {
			step, err := domain.BoundText("next_step", *in.NextStep, 2000)
			if err != nil {
				return err
			}
			next.NextStep = step
		}
		next.UpdatedAt = s.now()
		if err := s.DB.UpdateApplication(ctx, next); err != nil {
			return err
		}
		if err := s.recordEvent(ctx, domain.Event{
			ID: s.NewID(), UserID: a.UserID, ApplicationID: id, Kind: domain.EventStatusChange,
			FromStatus: cur.Shown(), ToStatus: status, Title: "Status changed", Body: note,
			OccurredAt: occurred, CreatedAt: s.now(), Actor: domain.ActorRef{TokenID: a.TokenID, Name: a.Name},
		}); err != nil {
			return err
		}
		if err := s.audit(ctx, a, "application.transition", "application", id, cur.Shown()+" -> "+status, map[string]any{"from": cur.Shown(), "to": status}); err != nil {
			return err
		}
		out, err = s.DB.GetApplication(ctx, a.UserID, id)
		return err
	})
	return out, err
}

func (s *Service) DeleteApplication(ctx context.Context, id string, version int) (domain.Deleted, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Deleted{}, err
	}
	if version < 1 {
		return domain.Deleted{}, versionRequired("application")
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		cur, err := s.DB.GetApplication(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		if err := s.DB.DeleteApplication(ctx, a.UserID, id, version); err != nil {
			return err
		}
		return s.audit(ctx, a, "application.delete", "application", id, "deleted "+cur.Title, nil)
	})
	if err != nil {
		return domain.Deleted{}, err
	}
	return domain.Deleted{ID: id, Deleted: true}, nil
}

// ArchiveApplication hides an application from the default list view
// without changing its status. Archiving an already-archived application
// is a no-op that returns it unchanged.
func (s *Service) ArchiveApplication(ctx context.Context, id string, version int) (domain.Application, error) {
	return s.setArchived(ctx, id, version, true)
}

// UnarchiveApplication restores an archived application to the default
// list view. Unarchiving an application that isn't archived is a no-op.
func (s *Service) UnarchiveApplication(ctx context.Context, id string, version int) (domain.Application, error) {
	return s.setArchived(ctx, id, version, false)
}

func (s *Service) setArchived(ctx context.Context, id string, version int, archived bool) (domain.Application, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Application{}, err
	}
	if version < 1 {
		return domain.Application{}, versionRequired("application")
	}
	var out domain.Application
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		cur, err := s.DB.GetApplication(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		if cur.Version != version {
			return domain.VersionConflict("application", cur.Version)
		}
		if (cur.ArchivedAt != nil) == archived {
			out = cur
			return nil
		}
		if err := s.DB.SetApplicationArchived(ctx, a.UserID, id, version, archived, s.now()); err != nil {
			return err
		}
		action := "application.unarchive"
		if archived {
			action = "application.archive"
		}
		if err := s.audit(ctx, a, action, "application", id, action+" "+cur.Title, nil); err != nil {
			return err
		}
		out, err = s.DB.GetApplication(ctx, a.UserID, id)
		return err
	})
	return out, err
}

func (s *Service) AddEvent(ctx context.Context, appID string, in EventInput) (domain.Event, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Event{}, err
	}
	kind, err := domain.ParseEventKind(in.Kind)
	if err != nil {
		return domain.Event{}, err
	}
	title, err := domain.BoundText("title", in.Title, 300)
	if err != nil {
		return domain.Event{}, err
	}
	if title == "" {
		title = domain.DefaultEventTitle(kind)
	}
	body, err := domain.BoundText("body", in.Body, 20000)
	if err != nil {
		return domain.Event{}, err
	}
	occurred := s.now()
	if strings.TrimSpace(in.OccurredAt) != "" {
		occurred, err = domain.ParseTime(in.OccurredAt)
		if err != nil {
			return domain.Event{}, domain.Validation("invalid occurred_at").WithHint("use RFC3339 or YYYY-MM-DD")
		}
	}
	ev := domain.Event{
		ID: s.NewID(), UserID: a.UserID, ApplicationID: appID, Kind: kind, Title: title, Body: body,
		OccurredAt: occurred, CreatedAt: s.now(), Actor: domain.ActorRef{TokenID: a.TokenID, Name: a.Name},
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.DB.GetApplication(ctx, a.UserID, appID); err != nil {
			return err
		}
		ev.UserID = a.UserID
		if err := s.recordEvent(ctx, ev); err != nil {
			return err
		}
		return s.audit(ctx, a, "event.create", "event", ev.ID, kind, map[string]any{"application_id": appID})
	})
	return ev, err
}

func (s *Service) ListEvents(ctx context.Context, q domain.ListQuery) (domain.Page[domain.Event], error) {
	return listPage(s, ctx, false, "event", q, s.DB.ListEvents, func(e domain.Event) string {
		if strings.TrimPrefix(q.Sort, "-") == "created_at" {
			return domain.FormatTime(e.CreatedAt)
		}
		return domain.FormatTime(e.OccurredAt)
	}, func(e domain.Event) string { return e.ID })
}

func (s *Service) CreateTag(ctx context.Context, in TagInput) (domain.Tag, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Tag{}, err
	}
	var out domain.Tag
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		tag, _, err := s.ensureTag(ctx, a, in.Name)
		out = tag
		return err
	})
	return out, err
}

func (s *Service) ensureTag(ctx context.Context, a domain.Actor, name string) (domain.Tag, bool, error) {
	name, err := domain.RequireText("name", name, 80)
	if err != nil {
		return domain.Tag{}, false, err
	}
	if tag, err := s.DB.GetTagByName(ctx, a.UserID, name); err == nil {
		return tag, false, nil
	} else if !domain.IsCode(err, "not_found") {
		return domain.Tag{}, false, err
	}
	tag := domain.Tag{ID: s.NewID(), UserID: a.UserID, Name: name, CreatedAt: s.now()}
	if err := s.DB.CreateTag(ctx, tag); err != nil {
		return domain.Tag{}, false, err
	}
	if err := s.audit(ctx, a, "tag.create", "tag", tag.ID, "created tag "+name, nil); err != nil {
		return domain.Tag{}, false, err
	}
	return tag, true, nil
}

func (s *Service) ListTags(ctx context.Context, q domain.ListQuery) (domain.Page[domain.Tag], error) {
	return listPage(s, ctx, false, "tag", q, s.DB.ListTags, func(t domain.Tag) string {
		if strings.TrimPrefix(q.Sort, "-") == "created_at" {
			return domain.FormatTime(t.CreatedAt)
		}
		return t.Name
	}, func(t domain.Tag) string { return t.ID })
}

func (s *Service) DeleteTag(ctx context.Context, id string) (domain.Deleted, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Deleted{}, err
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		tag, err := s.DB.GetTag(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		if err := s.DB.DeleteTag(ctx, a.UserID, id); err != nil {
			return err
		}
		return s.audit(ctx, a, "tag.delete", "tag", id, "deleted tag "+tag.Name, nil)
	})
	if err != nil {
		return domain.Deleted{}, err
	}
	return domain.Deleted{ID: id, Deleted: true}, nil
}

type FieldInput struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type SetFieldValueInput struct {
	Value string `json:"value"`
}

func (s *Service) CreateField(ctx context.Context, in FieldInput) (domain.CustomField, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.CustomField{}, err
	}
	name, err := domain.RequireText("name", in.Name, 80)
	if err != nil {
		return domain.CustomField{}, err
	}
	typ, err := domain.ParseFieldType(in.Type)
	if err != nil {
		return domain.CustomField{}, err
	}
	field := domain.CustomField{ID: s.NewID(), UserID: a.UserID, Name: name, Type: typ, CreatedAt: s.now()}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.CreateField(ctx, field); err != nil {
			return err
		}
		return s.audit(ctx, a, "field.create", "field", field.ID, "created field "+name, map[string]any{"type": typ})
	})
	return field, err
}

func (s *Service) ListFields(ctx context.Context, q domain.ListQuery) (domain.Page[domain.CustomField], error) {
	return listPage(s, ctx, false, "field", q, s.DB.ListFields, func(f domain.CustomField) string {
		if strings.TrimPrefix(q.Sort, "-") == "created_at" {
			return domain.FormatTime(f.CreatedAt)
		}
		return f.Name
	}, func(f domain.CustomField) string { return f.ID })
}

func (s *Service) DeleteField(ctx context.Context, id string) (domain.Deleted, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Deleted{}, err
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		field, err := s.DB.GetField(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		if err := s.DB.DeleteField(ctx, a.UserID, id); err != nil {
			return err
		}
		return s.audit(ctx, a, "field.delete", "field", id, "deleted field "+field.Name, nil)
	})
	if err != nil {
		return domain.Deleted{}, err
	}
	return domain.Deleted{ID: id, Deleted: true}, nil
}

// SetFieldValue validates value against the field's type and stores it.
// An empty value clears it (leaves no row, so FieldValues won't report it).
func (s *Service) SetFieldValue(ctx context.Context, appID, fieldID string, in SetFieldValueInput) (domain.Application, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Application{}, err
	}
	var out domain.Application
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.DB.GetApplication(ctx, a.UserID, appID); err != nil {
			return err
		}
		field, err := s.DB.GetField(ctx, a.UserID, fieldID)
		if err != nil {
			return err
		}
		value := strings.TrimSpace(in.Value)
		var num *float64
		switch field.Type {
		case domain.FieldNumber:
			if value != "" {
				n, err := strconv.ParseFloat(value, 64)
				if err != nil {
					return domain.Validation("invalid value").WithHint("this field is numeric")
				}
				num = &n
			}
		case domain.FieldDate:
			if value != "" {
				d, err := domain.ParseDate(value)
				if err != nil {
					return err
				}
				value = d
			}
		}
		if err := s.DB.SetFieldValue(ctx, appID, fieldID, value, num); err != nil {
			return err
		}
		if err := s.audit(ctx, a, "field.set_value", "application", appID, "set "+field.Name, map[string]any{"field": field.Name}); err != nil {
			return err
		}
		out, err = s.DB.GetApplication(ctx, a.UserID, appID)
		return err
	})
	return out, err
}

func (s *Service) AttachTag(ctx context.Context, appID string, in AttachTagInput) (domain.Application, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Application{}, err
	}
	var out domain.Application
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.DB.GetApplication(ctx, a.UserID, appID); err != nil {
			return err
		}
		if in.TagID != "" {
			tag, err := s.DB.GetTag(ctx, a.UserID, in.TagID)
			if err != nil {
				return err
			}
			if err := s.DB.AttachTag(ctx, appID, tag.ID); err != nil {
				return err
			}
			if err := s.audit(ctx, a, "tag.attach", "application", appID, "tagged "+tag.Name, map[string]any{"tag_id": tag.ID}); err != nil {
				return err
			}
		} else if _, _, err := s.attachByName(ctx, a, appID, in.Name, false); err != nil {
			return err
		}
		out, err = s.DB.GetApplication(ctx, a.UserID, appID)
		return err
	})
	return out, err
}

func (s *Service) attachByName(ctx context.Context, a domain.Actor, appID, name string, idOnly bool) (domain.Tag, bool, error) {
	if strings.TrimSpace(name) == "" {
		if idOnly {
			return domain.Tag{}, false, nil
		}
		return domain.Tag{}, false, domain.Validation("name or tag_id is required").WithHint("pass a tag name to create-or-attach, or a tag_id")
	}
	tag, created, err := s.ensureTag(ctx, a, name)
	if err != nil {
		return domain.Tag{}, false, err
	}
	if err := s.DB.AttachTag(ctx, appID, tag.ID); err != nil {
		return domain.Tag{}, false, err
	}
	if err := s.audit(ctx, a, "tag.attach", "application", appID, "tagged "+tag.Name, map[string]any{"tag_id": tag.ID}); err != nil {
		return domain.Tag{}, false, err
	}
	return tag, created, nil
}

func (s *Service) DetachTag(ctx context.Context, appID, tagID string) (domain.Application, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Application{}, err
	}
	var out domain.Application
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.DB.GetApplication(ctx, a.UserID, appID); err != nil {
			return err
		}
		tag, err := s.DB.GetTag(ctx, a.UserID, tagID)
		if err != nil {
			return err
		}
		if err := s.DB.DetachTag(ctx, appID, tagID); err != nil {
			return err
		}
		if err := s.audit(ctx, a, "tag.detach", "application", appID, "removed tag "+tag.Name, map[string]any{"tag_id": tagID}); err != nil {
			return err
		}
		out, err = s.DB.GetApplication(ctx, a.UserID, appID)
		return err
	})
	return out, err
}

type ImportResult struct {
	Rows                 int `json:"rows"`
	CompaniesCreated     int `json:"companies_created"`
	CompaniesExisting    int `json:"companies_existing"`
	RolesCreated         int `json:"roles_created"`
	RolesUpdated         int `json:"roles_updated"`
	ApplicationsCreated  int      `json:"applications_created"`
	ApplicationsUpdated  int      `json:"applications_updated"`
	Skipped              int      `json:"skipped"`
	Warnings             []string `json:"warnings,omitempty"`
}

func (s *Service) ImportCSV(ctx context.Context, r io.Reader) (ImportResult, error) {
	actor, err := s.actor(ctx, true)
	if err != nil {
		return ImportResult{}, err
	}
	rows, err := impex.ParseApplicationsCSV(r)
	if err != nil {
		return ImportResult{}, err
	}
	var result ImportResult
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		result = ImportResult{Rows: len(rows)}
		for i, row := range rows {
			if err := s.importRow(ctx, actor, row, i, &result); err != nil {
				return impex.RowError(row.Line, err)
			}
		}
		return s.audit(ctx, actor, "import.csv", "import", "", "imported csv", map[string]any{
			"rows": result.Rows, "applications_created": result.ApplicationsCreated, "applications_updated": result.ApplicationsUpdated,
		})
	})
	return result, err
}

func (s *Service) importRow(ctx context.Context, a domain.Actor, row impex.Row, index int, result *ImportResult) error {
	if _, err := domain.RequireText("company", row.Company, 200); err != nil {
		return err
	}
	if _, err := domain.RequireText("role", row.Title, 300); err != nil {
		return err
	}
	base := s.now()
	if strings.TrimSpace(row.LastUpdated) != "" {
		if t, err := time.Parse("2006-01-02", row.LastUpdated); err == nil {
			base = t.UTC()
		}
	}
	created := base.Add(time.Duration(index) * time.Millisecond)
	if row.ExternalID != "" {
		existing, err := s.DB.GetApplicationByExternalID(ctx, a.UserID, row.ExternalID)
		if err == nil {
			return s.updateImported(ctx, a, existing, row, result)
		}
		if !domain.IsCode(err, "not_found") {
			return err
		}
	}
	if url, err := domain.ParseURL(row.PostingURL); err != nil {
		return err
	} else if url != "" {
		existing, err := s.DB.GetApplicationByPostingURL(ctx, a.UserID, url)
		if err == nil {
			return s.updateImported(ctx, a, existing, row, result)
		}
		if !domain.IsCode(err, "not_found") {
			return err
		}
	}
	company, createdCo, err := s.ensureCompany(ctx, a, "", row.Company, domain.Origin(row.PostingURL))
	if err != nil {
		return err
	}
	if createdCo {
		result.CompaniesCreated++
	} else {
		result.CompaniesExisting++
	}
	in := ApplicationInput{
		CompanyID: company.ID, Title: row.Title, Location: row.Location, Salary: row.Salary,
		Fit: row.Fit, PostingURL: row.PostingURL, Status: row.Status, AppliedDate: row.AppliedDate,
		NextStep: row.NextStep, Notes: row.Notes, ExternalID: row.ExternalID,
	}
	// Count company as existing when ensure did not create. ensureCompany doesn't return a reliable
	// created flag when the company was just created in this call... it does.
	app, err := s.insertImported(ctx, a, company, in, created, result)
	if err != nil {
		return err
	}
	_ = app
	result.ApplicationsCreated++
	return nil
}

func (s *Service) insertImported(ctx context.Context, a domain.Actor, company domain.Company, in ApplicationInput, created time.Time, result *ImportResult) (domain.Application, error) {
	url, err := domain.ParseURL(in.PostingURL)
	if err != nil {
		return domain.Application{}, err
	}
	var createdRole bool
	if url != "" {
		if _, err := s.DB.FindRoleByURL(ctx, a.UserID, company.ID, url); err == nil {
			createdRole = false
		} else if domain.IsCode(err, "not_found") {
			createdRole = true
		} else if err != nil {
			return domain.Application{}, err
		}
	} else if _, err := s.DB.FindRoleByTitle(ctx, a.UserID, company.ID, in.Title); err == nil {
		createdRole = false
	} else if domain.IsCode(err, "not_found") {
		createdRole = true
	} else if err != nil {
		return domain.Application{}, err
	}
	app, err := s.insertApplication(ctx, a, in, created, domain.EventImported)
	if err != nil {
		return domain.Application{}, err
	}
	if createdRole {
		result.RolesCreated++
	}
	return app, nil
}

func (s *Service) updateImported(ctx context.Context, a domain.Actor, cur domain.Application, row impex.Row, result *ImportResult) error {
	role, err := s.DB.GetRole(ctx, a.UserID, cur.RoleID)
	if err != nil {
		return err
	}
	nextRole, changedRole, err := applyRolePatch(role, RolePatch{
		Title: &row.Title, Location: &row.Location, Salary: &row.Salary, Fit: &row.Fit, PostingURL: &row.PostingURL,
	})
	if err != nil {
		return err
	}
	if changedRole {
		nextRole.UpdatedAt = s.now()
		if err := s.DB.UpdateRole(ctx, nextRole); err != nil {
			return err
		}
		result.RolesUpdated++
	}
	status, err := domain.ParseStatus(row.Status)
	if err != nil {
		return err
	}
	date, err := domain.ParseDate(row.AppliedDate)
	if err != nil {
		return err
	}
	next := cur
	from := ""
	if status != cur.Shown() {
		from = cur.Shown()
		next.Status = status
	}
	if date == "" {
		next.AppliedDate = nil
	} else {
		next.AppliedDate = &date
	}
	if next.NextStep, err = domain.BoundText("next_step", row.NextStep, 2000); err != nil {
		return err
	}
	if next.Notes, err = domain.BoundText("notes", row.Notes, 20000); err != nil {
		return err
	}
	next.UpdatedAt = s.now()
	if err := s.DB.UpdateApplication(ctx, next); err != nil {
		return err
	}
	if from != "" {
		if err := s.recordEvent(ctx, domain.Event{
			ID: s.NewID(), UserID: a.UserID, ApplicationID: cur.ID, Kind: domain.EventStatusChange,
			FromStatus: from, ToStatus: status, Title: "Status changed", Body: "imported from CSV",
			OccurredAt: s.now(), CreatedAt: s.now(), Actor: domain.ActorRef{TokenID: a.TokenID, Name: a.Name},
		}); err != nil {
			return err
		}
	}
	if err := s.audit(ctx, a, "application.update", "application", cur.ID, "imported from csv", map[string]any{"external_id": row.ExternalID}); err != nil {
		return err
	}
	result.ApplicationsUpdated++
	return nil
}

func (s *Service) ExportSimplify(ctx context.Context) ([]byte, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	apps, err := s.DB.ListApplicationsForExport(ctx, a.UserID)
	if err != nil {
		return nil, err
	}
	if len(apps) > 10000 {
		return nil, domain.Validation("too many applications to export").WithHint("export is capped at 10000 rows")
	}
	rows := make([]impex.SimplifyRow, len(apps))
	for i, app := range apps {
		rows[i] = impex.FromApplication(app)
	}
	var buf bytes.Buffer
	if err := impex.WriteSimplify(&buf, rows); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type ExportJSON struct {
	ExportedAt   time.Time            `json:"exported_at"`
	Applications []domain.Application `json:"applications"`
}

func (s *Service) ExportJSON(ctx context.Context) (ExportJSON, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return ExportJSON{}, err
	}
	apps, err := s.DB.ListApplicationsForExport(ctx, a.UserID)
	if err != nil {
		return ExportJSON{}, err
	}
	if apps == nil {
		apps = []domain.Application{}
	}
	return ExportJSON{ExportedAt: s.now(), Applications: apps}, nil
}

func (s *Service) ListAudit(ctx context.Context, q domain.ListQuery) (domain.Page[domain.Audit], error) {
	return listPage(s, ctx, false, "audit", q, s.DB.ListAudit, func(a domain.Audit) string {
		return domain.FormatTime(a.CreatedAt)
	}, func(a domain.Audit) string { return a.ID })
}

func listPage[T any](s *Service, ctx context.Context, write bool, kind string, q domain.ListQuery, load func(context.Context, string, domain.ListQuery) ([]T, int, error), key func(T) string, id func(T) string) (domain.Page[T], error) {
	a, err := s.actor(ctx, write)
	if err != nil {
		return domain.Page[T]{}, err
	}
	if len(q.Q) > 200 {
		return domain.Page[T]{}, domain.Validation("q is too long").WithHint("keep q under 200 characters")
	}
	limit := q.Limit
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 {
		return domain.Page[T]{}, domain.Validation("invalid limit").WithHint("limit is an integer from 1 to 200")
	}
	// A "field:<name>" sort targets a custom field, not a built-in column —
	// PrepareSort's column allowlist doesn't (and shouldn't) know about
	// those, so it's left as-is and the store layer resolves it. Field
	// sorts don't support cursor pagination (see listApplicationsByField),
	// so there's no canonical sort key to compute here either.
	fieldSort := strings.HasPrefix(strings.TrimPrefix(q.Sort, "-"), "field:")
	var canon string
	if fieldSort {
		if q.Cursor != nil && q.Cursor.ID != "" {
			return domain.Page[T]{}, domain.Validation("field sorts don't support cursor pagination").WithHint("drop the cursor and re-run with a higher limit")
		}
	} else {
		var err error
		_, _, canon, err = sqlstorePrepare(kind, q.Sort, q.Cursor)
		if err != nil {
			return domain.Page[T]{}, err
		}
		q.Sort = canon
	}
	q.Limit = limit + 1
	items, total, err := load(ctx, a.UserID, q)
	if err != nil {
		return domain.Page[T]{}, err
	}
	has := len(items) > limit
	if has {
		items = items[:limit]
	}
	next := ""
	if has && len(items) > 0 && !fieldSort {
		last := items[len(items)-1]
		next = domain.EncodeCursor(canon, key(last), id(last))
	}
	if items == nil {
		items = []T{}
	}
	return domain.Page[T]{Data: items, Page: domain.PageMeta{Limit: limit, HasMore: has, NextCursor: next, Total: total}}, nil
}

func sqlstorePrepare(kind, sort string, cur *domain.Cursor) (string, bool, string, error) {
	return sqlstore.PrepareSort(kind, sort, cur)
}
