package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/impex"
	"github.com/pursuit/pursuit/internal/links"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

var webhookClient = &http.Client{Timeout: 15 * time.Second}

const (
	leaseTTL     = 2 * time.Minute
	maxArtifact  = 5 << 20
	workPrepare  = "prepare"
	workSubmit   = "submit"
)

func (s *Service) ensureDefaultCaps(ctx context.Context, userID string) error {
	defaults := []sqlstore.CompanyCap{
		{ID: s.NewID(), UserID: userID, Company: "Coinbase", Max: 3, WindowDays: 180, Mode: "active"},
		{ID: s.NewID(), UserID: userID, Company: "OpenAI", Max: 5, WindowDays: 180, Mode: "submitted"},
		{ID: s.NewID(), UserID: userID, Company: "Block", Max: 9, WindowDays: 60, Mode: "active"},
	}
	for _, c := range defaults {
		if err := s.DB.EnsureCap(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) recordEvent(ctx context.Context, ev domain.Event) error {
	if err := s.DB.CreateEvent(ctx, ev); err != nil {
		return err
	}
	if s.Hooks != nil && !sqlstore.IsDryRun(ctx) {
		cp := ev
		go s.Hooks(ev.UserID, cp)
	}
	return nil
}

func (s *Service) guardTransition(ctx context.Context, a domain.Actor, appID, company, from, to string) error {
	if to == "" || to == from {
		return nil
	}
	if to == domain.StatusApproved && !a.Can("approve") {
		return domain.Forbidden("approve")
	}
	if to == domain.StatusSubmitting || to == domain.StatusSubmitted {
		if appID == "" {
			return domain.Conflict("submitting requires a recorded human approval", "run pursuit approve <id> with a token that has the approve scope")
		}
		ok, err := s.DB.HasApproval(ctx, a.UserID, appID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.Conflict("submitting requires a recorded human approval", "run pursuit approve <id> with a token that has the approve scope")
		}
		if !domain.CountsAsSubmitted(from) {
			if err := s.enforceCaps(ctx, a.UserID, company, appID, "submit"); err != nil {
				return err
			}
		}
	}
	if from == "" && to != domain.StatusSaved && !domain.IsTerminalStatus(to) {
		if err := s.enforceCaps(ctx, a.UserID, company, appID, "create"); err != nil {
			return err
		}
		if domain.CountsAsSubmitted(to) {
			if err := s.enforceCaps(ctx, a.UserID, company, appID, "submit"); err != nil {
				return err
			}
		}
	}
	if to == domain.StatusApproved && appID != "" {
		if err := s.DB.CreateApproval(ctx, sqlstore.Approval{
			ID: s.NewID(), UserID: a.UserID, ApplicationID: appID, TokenID: a.TokenID,
			ActorName: a.Name, CreatedAt: domain.FormatTime(s.now()),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) enforceCaps(ctx context.Context, userID, company, excludeID, action string) error {
	company = strings.TrimSpace(company)
	if company == "" {
		return nil
	}
	caps, err := s.DB.ListCaps(ctx, userID)
	if err != nil {
		return err
	}
	for _, c := range caps {
		if !strings.EqualFold(c.Company, company) {
			continue
		}
		if action == "create" && c.Mode != "active" {
			continue
		}
		if action == "submit" && c.Mode != "submitted" {
			continue
		}
		since := domain.FormatTime(s.now().AddDate(0, 0, -c.WindowDays))
		n, err := s.DB.CountCap(ctx, userID, company, since, excludeID, c.Mode)
		if err != nil {
			return err
		}
		if n >= c.Max {
			return domain.CapExceeded(c.Company, n, c.Max, c.WindowDays)
		}
	}
	return nil
}

func (s *Service) ListLinks(ctx context.Context, q domain.ListQuery) (domain.Page[domain.LinkHealth], error) {
	if q.LinkStatus != "" {
		switch q.LinkStatus {
		case domain.LinkLive, domain.LinkClosed, domain.LinkUnknown:
		default:
			return domain.Page[domain.LinkHealth]{}, domain.Validation("invalid link status %q", q.LinkStatus).WithHint("status is live, closed, or unknown")
		}
	}
	return listPage(s, ctx, false, "link", q, s.DB.ListLinkHealth, func(row domain.LinkHealth) string {
		return linkSortKey(row, q.Sort)
	}, func(row domain.LinkHealth) string { return row.ApplicationID })
}

func linkSortKey(row domain.LinkHealth, sort string) string {
	switch strings.TrimPrefix(sort, "-") {
	case "company":
		return row.Company
	case "status":
		return row.Status
	default:
		return row.CheckedAt
	}
}

func (s *Service) ListCaps(ctx context.Context) ([]sqlstore.CompanyCap, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	return s.DB.ListCaps(ctx, a.UserID)
}

type WorkView struct {
	ID            string `json:"id"`
	ApplicationID string `json:"application_id"`
	Kind          string `json:"kind"`
	State         string `json:"state"`
	LeaseOwner    string `json:"lease_owner"`
	LeaseUntil    string `json:"lease_until"`
	Attempts      int    `json:"attempts"`
}

type ClaimResult struct {
	Work        *WorkView          `json:"work"`
	Application *domain.Application `json:"application,omitempty"`
}

func workView(w sqlstore.WorkItem) WorkView {
	return WorkView{ID: w.ID, ApplicationID: w.ApplicationID, Kind: w.Kind, State: w.State, LeaseOwner: w.LeaseOwner, LeaseUntil: w.LeaseUntil, Attempts: w.Attempts}
}

func (s *Service) Enqueue(ctx context.Context, applicationID, kind string) (WorkView, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return WorkView{}, err
	}
	if kind != workPrepare && kind != workSubmit {
		return WorkView{}, domain.Validation("invalid work kind %q", kind).WithHint("kind is prepare or submit")
	}
	var out WorkView
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		app, err := s.DB.GetApplication(ctx, a.UserID, applicationID)
		if err != nil {
			return err
		}
		if kind == workSubmit {
			if app.EffectiveStatus() != domain.StatusApproved && app.EffectiveStatus() != domain.StatusSubmitting {
				return domain.Conflict("submit work requires an approved application", "run pursuit approve <id> before queueing a submit")
			}
			if app.EffectiveStatus() != domain.StatusSubmitting {
				if err := s.guardTransition(ctx, a, app.ID, app.Company, app.EffectiveStatus(), domain.StatusSubmitting); err != nil {
					return err
				}
			}
		}
		now := domain.FormatTime(s.now())
		item, err := s.DB.EnqueueWork(ctx, sqlstore.WorkItem{
			ID: s.NewID(), UserID: a.UserID, ApplicationID: app.ID, Kind: kind, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		out = workView(item)
		return s.audit(ctx, a, "work.enqueue", "application", app.ID, kind+" queued", map[string]any{"work_id": item.ID})
	})
	return out, err
}

func (s *Service) Claim(ctx context.Context, worker string) (ClaimResult, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return ClaimResult{}, err
	}
	worker, err = domain.RequireText("worker", worker, 80)
	if err != nil {
		return ClaimResult{}, err
	}
	var result ClaimResult
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		now := s.now()
		item, ok, err := s.DB.ClaimWork(ctx, a.UserID, worker, domain.FormatTime(now), domain.FormatTime(now.Add(leaseTTL)))
		if err != nil || !ok {
			return err
		}
		app, err := s.DB.GetApplication(ctx, a.UserID, item.ApplicationID)
		if err != nil {
			return err
		}
		view := workView(item)
		result.Work = &view
		result.Application = &app
		return nil
	})
	return result, err
}

func (s *Service) Heartbeat(ctx context.Context, workID, worker string) (WorkView, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return WorkView{}, err
	}
	worker, err = domain.RequireText("worker", worker, 80)
	if err != nil {
		return WorkView{}, err
	}
	now := s.now()
	item, err := s.DB.HeartbeatWork(ctx, a.UserID, workID, worker, domain.FormatTime(now), domain.FormatTime(now.Add(leaseTTL)))
	if err != nil {
		return WorkView{}, err
	}
	return workView(item), nil
}

func (s *Service) CompleteWork(ctx context.Context, workID, worker, status, note string) (domain.Application, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Application{}, err
	}
	worker, err = domain.RequireText("worker", worker, 80)
	if err != nil {
		return domain.Application{}, err
	}
	if strings.TrimSpace(status) == "" {
		return domain.Application{}, domain.Validation("status is required").WithHint("complete with awaiting_review, needs_input, submitted, or failed")
	}
	st, err := domain.ParseStatus(status)
	if err != nil {
		return domain.Application{}, err
	}
	var out domain.Application
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		item, err := s.DB.GetWork(ctx, a.UserID, workID)
		if err != nil {
			return err
		}
		app, err := s.DB.GetApplication(ctx, a.UserID, item.ApplicationID)
		if err != nil {
			return err
		}
		if err := s.guardTransition(ctx, a, app.ID, app.Company, app.Status, st); err != nil {
			return err
		}
		now := domain.FormatTime(s.now())
		if err := s.DB.FinishWork(ctx, a.UserID, workID, worker, now); err != nil {
			return err
		}
		if app.Shown() != st {
			from := app.Shown()
			app.Status = st
			if st == domain.StatusSubmitted && app.AppliedDate == nil {
				d := s.now().Format("2006-01-02")
				app.AppliedDate = &d
			}
			app.UpdatedAt = s.now()
			if err := s.DB.UpdateApplication(ctx, app); err != nil {
				return err
			}
			if err := s.recordEvent(ctx, domain.Event{
				ID: s.NewID(), UserID: a.UserID, ApplicationID: app.ID, Kind: domain.EventStatusChange,
				FromStatus: from, ToStatus: st, Title: "Work completed", Body: note,
				OccurredAt: s.now(), CreatedAt: s.now(), Actor: domain.ActorRef{TokenID: a.TokenID, Name: a.Name},
			}); err != nil {
				return err
			}
		}
		out, err = s.DB.GetApplication(ctx, a.UserID, app.ID)
		return err
	})
	return out, err
}

func (s *Service) Approve(ctx context.Context, ids []string, note string) ([]domain.Application, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	if !a.Can("approve") {
		return nil, domain.Forbidden("approve")
	}
	if len(ids) == 0 {
		return nil, domain.Validation("at least one application id is required").WithHint("pursuit approve <id> [<id>...]")
	}
	note, err = domain.BoundText("note", note, 2000)
	if err != nil {
		return nil, err
	}
	var out []domain.Application
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		for _, id := range ids {
			app, err := s.DB.GetApplication(ctx, a.UserID, id)
			if err != nil {
				return err
			}
			if app.EffectiveStatus() != domain.StatusAwaitingReview {
				return domain.Validation("application %s is %s", id, app.EffectiveStatus()).WithHint("only awaiting_review applications can be approved")
			}
			if err := s.guardTransition(ctx, a, app.ID, app.Company, app.Status, domain.StatusApproved); err != nil {
				return err
			}
			from := app.Status
			app.Status = domain.StatusApproved
			app.UpdatedAt = s.now()
			if err := s.DB.UpdateApplication(ctx, app); err != nil {
				return err
			}
			now := domain.FormatTime(s.now())
			if _, err := s.DB.EnqueueWork(ctx, sqlstore.WorkItem{
				ID: s.NewID(), UserID: a.UserID, ApplicationID: app.ID, Kind: workSubmit, CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				return err
			}
			if err := s.recordEvent(ctx, domain.Event{
				ID: s.NewID(), UserID: a.UserID, ApplicationID: app.ID, Kind: domain.EventApproval,
				FromStatus: from, ToStatus: domain.StatusApproved, Title: "Approved to submit", Body: note,
				OccurredAt: s.now(), CreatedAt: s.now(), Actor: domain.ActorRef{TokenID: a.TokenID, Name: a.Name},
			}); err != nil {
				return err
			}
			if err := s.audit(ctx, a, "application.approve", "application", app.ID, "approved "+app.Title, map[string]any{"token_id": a.TokenID}); err != nil {
				return err
			}
			got, err := s.DB.GetApplication(ctx, a.UserID, app.ID)
			if err != nil {
				return err
			}
			out = append(out, got)
		}
		return nil
	})
	return out, err
}

// Reject declines prepared applications. It requires the approve scope, moves
// awaiting_review work to withdrawn, and does not enqueue a submit.
func (s *Service) Reject(ctx context.Context, ids []string, note string) ([]domain.Application, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	if !a.Can("approve") {
		return nil, domain.Forbidden("approve")
	}
	if len(ids) == 0 {
		return nil, domain.Validation("at least one application id is required").WithHint("pursuit reject <id> [<id>...]")
	}
	note, err = domain.BoundText("note", note, 2000)
	if err != nil {
		return nil, err
	}
	var out []domain.Application
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		for _, id := range ids {
			app, err := s.DB.GetApplication(ctx, a.UserID, id)
			if err != nil {
				return err
			}
			if app.EffectiveStatus() != domain.StatusAwaitingReview {
				return domain.Validation("application %s is %s", id, app.EffectiveStatus()).WithHint("only awaiting_review applications can be rejected")
			}
			if err := s.guardTransition(ctx, a, app.ID, app.Company, app.Status, domain.StatusWithdrawn); err != nil {
				return err
			}
			from := app.Status
			app.Status = domain.StatusWithdrawn
			app.UpdatedAt = s.now()
			if err := s.DB.UpdateApplication(ctx, app); err != nil {
				return err
			}
			now := domain.FormatTime(s.now())
			if err := s.DB.CancelOpenWork(ctx, a.UserID, app.ID, now); err != nil {
				return err
			}
			if err := s.recordEvent(ctx, domain.Event{
				ID: s.NewID(), UserID: a.UserID, ApplicationID: app.ID, Kind: domain.EventApproval,
				FromStatus: from, ToStatus: domain.StatusWithdrawn, Title: "Approval rejected", Body: note,
				OccurredAt: s.now(), CreatedAt: s.now(), Actor: domain.ActorRef{TokenID: a.TokenID, Name: a.Name},
			}); err != nil {
				return err
			}
			if err := s.audit(ctx, a, "application.reject", "application", app.ID, "rejected "+app.Title, map[string]any{"token_id": a.TokenID}); err != nil {
				return err
			}
			got, err := s.DB.GetApplication(ctx, a.UserID, app.ID)
			if err != nil {
				return err
			}
			out = append(out, got)
		}
		return nil
	})
	return out, err
}

type QuestionView struct {
	ID       string `json:"id"`
	Prompt   string `json:"prompt"`
	Answer   string `json:"answer"`
	Required bool   `json:"required"`
	Unknown  bool   `json:"unknown"`
	Source   string `json:"source,omitempty"`
}

type PrepareResult struct {
	Application domain.Application `json:"application"`
	Questions   []QuestionView     `json:"questions"`
	Warnings    []string           `json:"warnings,omitempty"`
	Unknowns    int                `json:"unknowns"`
}

func (s *Service) Prepare(ctx context.Context, id, worker string) (PrepareResult, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return PrepareResult{}, err
	}
	var result PrepareResult
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		app, err := s.DB.GetApplication(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		switch app.EffectiveStatus() {
		case domain.StatusToApply, domain.StatusPreparing, domain.StatusNeedsInput, domain.StatusSaved, domain.StatusAwaitingReview:
		default:
			return domain.Validation("cannot prepare an application in status %s", app.EffectiveStatus()).WithHint("prepare is for to_apply, preparing, needs_input, saved, or awaiting_review")
		}
		if strings.TrimSpace(worker) != "" {
			// A lease held by someone else blocks a second prepare.
			// Claim is optional for the synchronous path.
		}
		if warn, err := s.capWarning(ctx, a.UserID, app.Company, app.ID); err != nil {
			return err
		} else if warn != "" {
			result.Warnings = append(result.Warnings, warn)
		}
		from := app.Status
		if from != domain.StatusPreparing {
			if err := s.guardTransition(ctx, a, app.ID, app.Company, from, domain.StatusPreparing); err != nil {
				return err
			}
			app.Status = domain.StatusPreparing
			app.UpdatedAt = s.now()
			if err := s.DB.UpdateApplication(ctx, app); err != nil {
				return err
			}
			app.Version++
		}
		form, raw, err := s.links().FetchForm(ctx, app.PostingURL)
		if err != nil {
			result.Warnings = append(result.Warnings, "form fetch failed: "+err.Error())
		}
		standing, err := s.standingMap(ctx, a.UserID)
		if err != nil {
			return err
		}
		now := domain.FormatTime(s.now())
		qs := make([]sqlstore.Question, 0, len(form))
		views := make([]QuestionView, 0, len(form))
		unknowns := 0
		for _, fq := range form {
			answer, ok := matchStanding(fq.Prompt, standing)
			unknown := fq.Required && !ok
			if unknown {
				unknowns++
			}
			q := sqlstore.Question{
				ID: s.NewID(), Prompt: fq.Prompt, Answer: answer, Required: fq.Required,
				Unknown: unknown, Source: "form", CreatedAt: now,
			}
			if ok {
				q.Source = "standing"
			}
			qs = append(qs, q)
			views = append(views, QuestionView{ID: q.ID, Prompt: q.Prompt, Answer: q.Answer, Required: q.Required, Unknown: q.Unknown, Source: q.Source})
		}
		if err := s.DB.ReplaceQuestions(ctx, a.UserID, app.ID, qs); err != nil {
			return err
		}
		snap := map[string]any{"posting_url": app.PostingURL, "questions": views}
		if json.Valid(raw) && len(raw) < maxArtifact {
			snap["raw"] = json.RawMessage(raw)
		}
		snapBody, _ := json.Marshal(snap)
		if err := s.DB.CreateArtifact(ctx, sqlstore.Artifact{
			ID: s.NewID(), UserID: a.UserID, ApplicationID: app.ID, Kind: "form_snapshot",
			Filename: "form.json", ContentType: "application/json", Body: string(snapBody), CreatedAt: now,
		}); err != nil {
			return err
		}
		answersBody, _ := json.Marshal(map[string]any{"questions": views, "standing_keys": keysOf(standing)})
		if err := s.DB.CreateArtifact(ctx, sqlstore.Artifact{
			ID: s.NewID(), UserID: a.UserID, ApplicationID: app.ID, Kind: "answers_json",
			Filename: "answers.json", ContentType: "application/json", Body: string(answersBody), CreatedAt: now,
		}); err != nil {
			return err
		}
		if letter := standing["cover_letter"]; letter != "" {
			if err := s.DB.CreateArtifact(ctx, sqlstore.Artifact{
				ID: s.NewID(), UserID: a.UserID, ApplicationID: app.ID, Kind: "cover_letter",
				Filename: "cover-letter.txt", ContentType: "text/plain", Body: letter, CreatedAt: now,
			}); err != nil {
				return err
			}
		}
		if err := s.attachResume(ctx, a.UserID, app.ID, now); err != nil {
			return err
		}
		next := domain.StatusAwaitingReview
		if unknowns > 0 {
			next = domain.StatusNeedsInput
		}
		if err := s.guardTransition(ctx, a, app.ID, app.Company, app.Status, next); err != nil {
			return err
		}
		app.Status = next
		app.UpdatedAt = s.now()
		if err := s.DB.UpdateApplication(ctx, app); err != nil {
			return err
		}
		if err := s.recordEvent(ctx, domain.Event{
			ID: s.NewID(), UserID: a.UserID, ApplicationID: app.ID, Kind: domain.EventStatusChange,
			FromStatus: from, ToStatus: next, Title: "Prepared application",
			Body: noteForPrepare(unknowns), OccurredAt: s.now(), CreatedAt: s.now(),
			Actor: domain.ActorRef{TokenID: a.TokenID, Name: a.Name},
		}); err != nil {
			return err
		}
		got, err := s.DB.GetApplication(ctx, a.UserID, app.ID)
		if err != nil {
			return err
		}
		result.Application = got
		result.Questions = views
		result.Unknowns = unknowns
		if result.Questions == nil {
			result.Questions = []QuestionView{}
		}
		return nil
	})
	return result, err
}

func noteForPrepare(unknowns int) string {
	if unknowns == 0 {
		return "draft answers attached; waiting for approval"
	}
	return "required questions have no standing answer"
}

func (s *Service) capWarning(ctx context.Context, userID, company, appID string) (string, error) {
	err := s.enforceCaps(ctx, userID, company, appID, "submit")
	if err == nil {
		return "", nil
	}
	if de, ok := domain.As(err); ok && strings.Contains(de.Message, "cap reached") {
		return de.Message, nil
	}
	return "", err
}

func (s *Service) links() *links.Client {
	if s.Links != nil {
		return s.Links
	}
	s.Links = &links.Client{HTTP: s.HTTP, Now: s.Now, Gap: 0}
	return s.Links
}

func (s *Service) standingMap(ctx context.Context, userID string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range s.Profile {
		if strings.TrimSpace(v) != "" {
			out[impex.NormalizeKey(k)] = v
		}
	}
	rows, err := s.DB.ListStanding(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.Key] = row.Answer
	}
	return out, nil
}

func (s *Service) attachResume(ctx context.Context, userID, appID, now string) error {
	path := strings.TrimSpace(s.ResumeFile)
	if path == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return domain.Validation("resume file: %s", err.Error()).WithHint("set PURSUIT_RESUME_FILE to a readable PDF, or leave it unset")
	}
	if len(b) > maxArtifact {
		return domain.Validation("resume file is too large").WithHint("keep the resume under 5MB")
	}
	return s.DB.CreateArtifact(ctx, sqlstore.Artifact{
		ID: s.NewID(), UserID: userID, ApplicationID: appID, Kind: "resume_pdf",
		Filename: "resume.pdf", ContentType: "application/pdf",
		Body: base64.StdEncoding.EncodeToString(b), CreatedAt: now,
	})
}

func matchStanding(prompt string, standing map[string]string) (string, bool) {
	if v, ok := standing[impex.NormalizeKey(prompt)]; ok && strings.TrimSpace(v) != "" {
		return v, true
	}
	low := strings.ToLower(prompt)
	aliases := []struct{ needle, key string }{
		{"e-mail", "email"}, {"email", "email"}, {"phone", "phone"},
		{"linkedin", "linkedin"}, {"github", "github"}, {"website", "website"},
		{"authorized to work", "work_authorization"}, {"work authorization", "work_authorization"},
		{"sponsorship", "visa_sponsorship"}, {"resume", "resume"}, {"curriculum vitae", "resume"},
		{"cover letter", "cover_letter"}, {"pronouns", "pronouns"},
	}
	for _, alias := range aliases {
		if strings.Contains(low, alias.needle) {
			if v, ok := standing[alias.key]; ok && strings.TrimSpace(v) != "" {
				return v, true
			}
		}
	}
	return "", false
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

type BatchResult struct {
	IDs      []string `json:"ids"`
	Prepared int      `json:"prepared"`
	Queued   int      `json:"queued"`
	Errors   []string `json:"errors,omitempty"`
}

func (s *Service) ApplyBatch(ctx context.Context, q domain.ListQuery, mode string) (BatchResult, error) {
	if mode != "prepare" && mode != "async" {
		return BatchResult{}, domain.Validation("invalid batch mode").WithHint("use --prepare or --async")
	}
	q.Limit = 200
	page, err := s.ListApplications(ctx, q)
	if err != nil {
		return BatchResult{}, err
	}
	var result BatchResult
	for _, app := range page.Data {
		result.IDs = append(result.IDs, app.ID)
		if mode == "async" {
			kind := workPrepare
			if app.EffectiveStatus() == domain.StatusApproved {
				kind = workSubmit
			}
			if _, err := s.Enqueue(ctx, app.ID, kind); err != nil {
				result.Errors = append(result.Errors, app.ID+": "+err.Error())
				continue
			}
			result.Queued++
			continue
		}
		if _, err := s.Prepare(ctx, app.ID, ""); err != nil {
			result.Errors = append(result.Errors, app.ID+": "+err.Error())
			continue
		}
		result.Prepared++
	}
	if result.IDs == nil {
		result.IDs = []string{}
	}
	return result, nil
}

func (s *Service) ImportPostings(ctx context.Context, rows []impex.Posting) (ImportResult, error) {
	actor, err := s.actor(ctx, true)
	if err != nil {
		return ImportResult{}, err
	}
	var result ImportResult
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		for _, row := range rows {
			result.Rows++
			url, err := domain.ParseURL(row.URL)
			if err != nil {
				return impex.RowError(row.Line, err)
			}
			if url != "" {
				if _, err := s.DB.FindRoleByPostingURL(ctx, actor.UserID, url); err == nil {
					result.Skipped++
					continue
				} else if !domain.IsCode(err, "not_found") {
					return err
				}
			}
			status := row.Status
			if strings.TrimSpace(status) == "" {
				status = domain.StatusToApply
			}
			company, createdCo, err := s.ensureCompany(ctx, actor, "", row.Company, domain.Origin(url))
			if err != nil {
				return impex.RowError(row.Line, err)
			}
			if createdCo {
				result.CompaniesCreated++
			} else {
				result.CompaniesExisting++
			}
			in := ApplicationInput{
				CompanyID: company.ID, Title: row.Title, Location: row.Location, Salary: row.Salary,
				Fit: row.Fit, PostingURL: url, Status: status, AppliedDate: row.AppliedDate,
				NextStep: row.NextStep, Notes: row.Notes, ExternalID: row.ExternalID,
			}
			if _, err := s.insertApplication(ctx, actor, in, s.now(), domain.EventImported); err != nil {
				return impex.RowError(row.Line, err)
			}
			result.ApplicationsCreated++
			result.RolesCreated++
		}
		return s.audit(ctx, actor, "import.postings", "import", "", "imported postings", map[string]any{"rows": result.Rows, "skipped": result.Skipped})
	})
	return result, err
}

type ArtifactView struct {
	ID            string `json:"id"`
	ApplicationID string `json:"application_id"`
	Kind          string `json:"kind"`
	Filename      string `json:"filename"`
	ContentType   string `json:"content_type"`
	CreatedAt     string `json:"created_at"`
}

func (s *Service) AddArtifact(ctx context.Context, appID, kind, filename, contentType string, body []byte) (ArtifactView, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return ArtifactView{}, err
	}
	kind, err = artifactKind(kind)
	if err != nil {
		return ArtifactView{}, err
	}
	if len(body) == 0 {
		return ArtifactView{}, domain.Validation("artifact body is empty").WithHint("pass a file")
	}
	if len(body) > maxArtifact {
		return ArtifactView{}, domain.Validation("artifact is too large").WithHint("keep artifacts under 5MB")
	}
	if _, err := s.DB.GetApplication(ctx, a.UserID, appID); err != nil {
		return ArtifactView{}, err
	}
	stored := string(body)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if kind == "resume_pdf" || kind == "confirmation" || !isText(contentType) {
		stored = base64.StdEncoding.EncodeToString(body)
	}
	now := domain.FormatTime(s.now())
	art := sqlstore.Artifact{
		ID: s.NewID(), UserID: a.UserID, ApplicationID: appID, Kind: kind,
		Filename: filename, ContentType: contentType, Body: stored, CreatedAt: now,
	}
	if err := s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.CreateArtifact(ctx, art); err != nil {
			return err
		}
		return s.audit(ctx, a, "artifact.create", "application", appID, kind, map[string]any{"artifact_id": art.ID})
	}); err != nil {
		return ArtifactView{}, err
	}
	return ArtifactView{ID: art.ID, ApplicationID: appID, Kind: kind, Filename: filename, ContentType: contentType, CreatedAt: now}, nil
}

func (s *Service) ListArtifacts(ctx context.Context, appID string) ([]ArtifactView, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	if _, err := s.DB.GetApplication(ctx, a.UserID, appID); err != nil {
		return nil, err
	}
	rows, err := s.DB.ListArtifacts(ctx, a.UserID, appID)
	if err != nil {
		return nil, err
	}
	out := make([]ArtifactView, len(rows))
	for i, row := range rows {
		out[i] = ArtifactView{ID: row.ID, ApplicationID: row.ApplicationID, Kind: row.Kind, Filename: row.Filename, ContentType: row.ContentType, CreatedAt: row.CreatedAt}
	}
	return out, nil
}

func (s *Service) GetArtifact(ctx context.Context, id string) (sqlstore.Artifact, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return sqlstore.Artifact{}, err
	}
	return s.DB.GetArtifact(ctx, a.UserID, id)
}

func artifactKind(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "resume_pdf", "cover_letter", "answers_json", "form_snapshot", "confirmation":
		return strings.ToLower(strings.TrimSpace(kind)), nil
	default:
		return "", domain.Validation("invalid artifact kind %q", kind).WithHint("kind is resume_pdf, cover_letter, answers_json, form_snapshot, or confirmation")
	}
}

func isText(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.HasPrefix(ct, "text/") || strings.Contains(ct, "json") || ct == ""
}

func (s *Service) ListQuestions(ctx context.Context, appID string) ([]QuestionView, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.ListQuestions(ctx, a.UserID, appID)
	if err != nil {
		return nil, err
	}
	out := make([]QuestionView, len(rows))
	for i, q := range rows {
		out[i] = QuestionView{ID: q.ID, Prompt: q.Prompt, Answer: q.Answer, Required: q.Required, Unknown: q.Unknown, Source: q.Source}
	}
	return out, nil
}

func (s *Service) SetStanding(ctx context.Context, key, answer string) (sqlstore.StandingAnswer, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return sqlstore.StandingAnswer{}, err
	}
	key = impex.NormalizeKey(key)
	if key == "" {
		return sqlstore.StandingAnswer{}, domain.Validation("key is required").WithHint("standing keys are words such as work_authorization or email")
	}
	answer, err = domain.RequireText("answer", answer, 20000)
	if err != nil {
		return sqlstore.StandingAnswer{}, err
	}
	row := sqlstore.StandingAnswer{ID: s.NewID(), UserID: a.UserID, Key: key, Answer: answer, UpdatedAt: domain.FormatTime(s.now())}
	if err := s.DB.UpsertStanding(ctx, row); err != nil {
		return sqlstore.StandingAnswer{}, err
	}
	return row, nil
}

func (s *Service) ListStanding(ctx context.Context) ([]sqlstore.StandingAnswer, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	return s.DB.ListStanding(ctx, a.UserID)
}

type LinkCheckView struct {
	RoleID     string `json:"role_id"`
	PostingURL string `json:"posting_url"`
	Status     string `json:"status"`
	Detail     string `json:"detail"`
	Changed    bool   `json:"changed"`
	HTTPStatus int    `json:"http_status"`
}

type CheckReport struct {
	Checked int             `json:"checked"`
	Results []LinkCheckView `json:"results"`
}

func (s *Service) CheckApplicationLink(ctx context.Context, applicationID string) (LinkCheckView, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return LinkCheckView{}, err
	}
	if a.Demo {
		// ponytail: a flat deny, not a real SSRF filter — a demo actor
		// can't make the server fetch an attacker-chosen URL at all.
		return LinkCheckView{}, domain.Forbidden("write")
	}
	app, err := s.DB.GetApplication(ctx, a.UserID, applicationID)
	if err != nil {
		return LinkCheckView{}, err
	}
	return s.checkRole(ctx, a, app.RoleID, app.PostingURL)
}

// SweepAsOwner runs the link sweep as the database owner. The daemon uses
// this so a replica can check postings without a separate token on disk.
func (s *Service) SweepAsOwner(ctx context.Context, owner string) (CheckReport, error) {
	user, err := s.DB.FirstUser(ctx)
	if err != nil {
		return CheckReport{}, err
	}
	ctx = domain.WithActor(ctx, domain.Actor{UserID: user.ID, Name: owner, Scopes: []string{"read", "write"}})
	return s.SweepOnce(ctx, owner)
}

func (s *Service) SweepOnce(ctx context.Context, owner string) (CheckReport, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return CheckReport{}, err
	}
	owner, err = domain.RequireText("owner", owner, 80)
	if err != nil {
		return CheckReport{}, err
	}
	now := s.now()
	leader, ok, err := s.DB.AcquireLeader(ctx, "link-sweep", owner, now, now.Add(2*time.Minute))
	if err != nil {
		return CheckReport{}, err
	}
	if !ok {
		return CheckReport{}, domain.Conflict("link sweep is already running", "another replica holds the sweep lock; retry after it finishes")
	}
	defer leader.Release()
	return s.checkAll(ctx, a)
}

func (s *Service) checkAll(ctx context.Context, a domain.Actor) (CheckReport, error) {
	ids, err := s.DB.ListRoleIDsForCheck(ctx, a.UserID)
	if err != nil {
		return CheckReport{}, err
	}
	var report CheckReport
	for _, id := range ids {
		role, err := s.DB.GetRole(ctx, a.UserID, id)
		if err != nil {
			return report, err
		}
		view, err := s.checkRole(ctx, a, role.ID, role.PostingURL)
		if err != nil {
			return report, err
		}
		report.Results = append(report.Results, view)
		report.Checked++
	}
	if report.Results == nil {
		report.Results = []LinkCheckView{}
	}
	return report, nil
}

func (s *Service) checkRole(ctx context.Context, a domain.Actor, roleID, postingURL string) (LinkCheckView, error) {
	prev, found, err := s.DB.GetPostingCheck(ctx, roleID)
	if err != nil {
		return LinkCheckView{}, err
	}
	res := s.links().Check(ctx, postingURL)
	checked := domain.FormatTime(res.CheckedAt)
	// The response reports a change on the first observation. The stored flag
	// is only set when a previous result existed and the status moved.
	responseChanged := !found || prev.Status != res.Status
	storedChanged := found && prev.Status != res.Status
	if err := s.DB.UpsertPostingCheck(ctx, sqlstore.PostingCheck{
		RoleID: roleID, UserID: a.UserID, Status: res.Status, Detail: res.Detail,
		HTTPStatus: res.HTTPStatus, CheckedAt: checked, Changed: storedChanged,
	}); err != nil {
		return LinkCheckView{}, err
	}
	changed := responseChanged
	view := LinkCheckView{RoleID: roleID, PostingURL: postingURL, Status: res.Status, Detail: res.Detail, Changed: changed, HTTPStatus: res.HTTPStatus}
	if !changed {
		return view, nil
	}
	apps, err := s.DB.ApplicationsForRole(ctx, a.UserID, roleID)
	if err != nil {
		return view, err
	}
	for _, app := range apps {
		if res.Status == domain.LinkClosed && !strings.Contains(strings.ToLower(app.NextStep), "posting closed") {
			app.NextStep = strings.TrimSpace("Posting closed. " + app.NextStep)
			app.UpdatedAt = s.now()
			if err := s.DB.UpdateApplication(ctx, app); err != nil {
				return view, err
			}
		}
		if err := s.recordEvent(ctx, domain.Event{
			ID: s.NewID(), UserID: a.UserID, ApplicationID: app.ID, Kind: domain.EventLinkCheck,
			Title: "Posting " + res.Status, Body: res.Detail, ToStatus: res.Status,
			OccurredAt: res.CheckedAt, CreatedAt: s.now(),
			Actor: domain.ActorRef{TokenID: a.TokenID, Name: a.Name},
		}); err != nil {
			return view, err
		}
	}
	return view, nil
}

type WebhookView struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
}

func (s *Service) CreateWebhook(ctx context.Context, rawURL, secret string) (WebhookView, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return WebhookView{}, err
	}
	u, err := domain.ParseURL(rawURL)
	if err != nil || u == "" {
		if err == nil {
			err = domain.Validation("url is required")
		}
		return WebhookView{}, err
	}
	secret, err = domain.BoundText("secret", secret, 200)
	if err != nil {
		return WebhookView{}, err
	}
	now := domain.FormatTime(s.now())
	w := sqlstore.Webhook{ID: s.NewID(), UserID: a.UserID, URL: u, Secret: secret, CreatedAt: now}
	if err := s.DB.CreateWebhook(ctx, w); err != nil {
		return WebhookView{}, err
	}
	return WebhookView{ID: w.ID, URL: w.URL, CreatedAt: w.CreatedAt}, nil
}

func (s *Service) ListWebhooks(ctx context.Context) ([]WebhookView, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.ListWebhooks(ctx, a.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]WebhookView, len(rows))
	for i, w := range rows {
		out[i] = WebhookView{ID: w.ID, URL: w.URL, CreatedAt: w.CreatedAt}
	}
	return out, nil
}

func (s *Service) DeleteWebhook(ctx context.Context, id string) (domain.Deleted, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Deleted{}, err
	}
	if err := s.DB.DeleteWebhook(ctx, a.UserID, id); err != nil {
		return domain.Deleted{}, err
	}
	return domain.Deleted{ID: id, Deleted: true}, nil
}

// DeliverWebhooks posts one event to every webhook for the user.
// The HTTP call uses s.HTTP when set.
func (s *Service) DeliverWebhooks(ctx context.Context, userID string, ev domain.Event) {
	rows, err := s.DB.ListWebhooks(ctx, userID)
	if err != nil || len(rows) == 0 {
		return
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	client := s.HTTP
	if client == nil {
		client = webhookClient
	}
	for _, w := range rows {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(payload))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "pursuit-webhook")
		if w.Secret != "" {
			req.Header.Set("X-Pursuit-Secret", w.Secret)
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
	}
}

func (s *Service) ListEventsFollow(ctx context.Context, q domain.ListQuery, wait time.Duration) (domain.Page[domain.Event], error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return domain.Page[domain.Event]{}, err
	}
	if q.AfterID != "" {
		ev, err := s.DB.GetEvent(ctx, a.UserID, q.AfterID)
		if err != nil {
			return domain.Page[domain.Event]{}, err
		}
		q.AfterTime = domain.FormatTime(ev.OccurredAt)
		if q.Sort == "" {
			q.Sort = "occurred_at"
		}
	}
	if wait < 0 {
		wait = 0
	}
	if wait > 60*time.Second {
		wait = 60 * time.Second
	}
	deadline := time.Now().Add(wait)
	for {
		page, err := s.ListEvents(ctx, q)
		if err != nil || len(page.Data) > 0 || wait == 0 || time.Now().After(deadline) {
			return page, err
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return page, nil
		case <-timer.C:
		}
	}
}
