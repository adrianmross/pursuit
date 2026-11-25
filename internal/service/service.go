// Package service is the transport-agnostic use-case layer.
// HTTP and any later adapter should call Service methods rather than SQL.
package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/adrianmross/appkit/account"
	"github.com/pursuit/pursuit/internal/billing"
	"github.com/pursuit/pursuit/internal/blobstore"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/embed"
	"github.com/pursuit/pursuit/internal/links"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

type Service struct {
	DB         *sqlstore.Store
	Now        func() time.Time
	RateNow    func() time.Time // clock for rate limits; nil means Now
	NewID      func() string
	HTTP       *http.Client
	Links      *links.Client
	Profile    map[string]string
	ResumeFile string
	// Hooks is invoked after an event is stored. It must not block the caller for long.
	Hooks func(userID string, ev domain.Event)
	// Passkeys is nil unless passkeys are configured (NewPasskeys).
	Passkeys *account.Passkeys
	// Stripe is nil unless billing is configured; then every configured
	// feature is entitled (self-hosted). StripePrice is the plan's price.
	// PremiumOn (PURSUIT_PREMIUM_FEATURES=on) lets premium features and Stripe billing work.
	// Off, they are parked: shown as coming soon, never available, and Stripe stays unconfigured.
	PremiumOn           bool
	Stripe              *billing.Client
	StripePrice         string
	StripeWebhookSecret string
	// PublicURL is where Stripe sends the user back; empty uses the request's origin.
	PublicURL string
	// Capabilities is which Features this deployment is set up for, by key.
	Capabilities map[string]bool
	// Embedder powers smart search; nil means it isn't configured.
	Embedder embed.Embedder
	// ClientDoc fetches an OAuth client metadata document; nil uses FetchClientDoc.
	ClientDoc func(ctx context.Context, url string) ([]byte, error)
	// Blobs holds new document files when a bucket is configured; nil keeps them in the database.
	Blobs blobstore.Store
	// OnUserCreated runs right after a user row is inserted. Sign-in and
	// bootstrap call it inside their lock, so a failure there undoes the
	// user. It is where an app gives a new account its starting data. New
	// sets it to the default caps.
	OnUserCreated func(ctx context.Context, u domain.User) error
}

func New(db *sqlstore.Store) *Service {
	s := &Service{DB: db, Now: time.Now, NewID: uuid.NewString}
	s.OnUserCreated = func(ctx context.Context, u domain.User) error { return s.ensureDefaultCaps(ctx, u.ID) }
	return s
}

func (s *Service) now() time.Time { return s.Now().UTC() }

func (s *Service) actor(ctx context.Context, write bool) (domain.Actor, error) {
	a, ok := domain.ActorFrom(ctx)
	if !ok || a.UserID == "" {
		return domain.Actor{}, domain.Unauthorized()
	}
	need := "read"
	if write {
		need = "write"
	}
	if !a.Can(need) {
		return domain.Actor{}, domain.Forbidden(need)
	}
	return a, nil
}

func (s *Service) audit(ctx context.Context, a domain.Actor, action, entityType, entityID, summary string, meta map[string]any) error {
	var raw json.RawMessage
	if meta != nil {
		b, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		raw = b
	}
	return s.DB.CreateAudit(ctx, domain.Audit{
		ID: s.NewID(), UserID: a.UserID, TokenID: a.TokenID, ActorName: a.Name,
		Action: action, EntityType: entityType, EntityID: entityID, Summary: summary,
		Metadata: raw, CreatedAt: s.now(),
	})
}

// Bootstrap creates the owner and the first token when the database is empty.
// The returned secret is set only when a token was created.
func (s *Service) Bootstrap(ctx context.Context, ownerName, ownerEmail, tokenName, secret string) (string, bool, error) {
	if strings.TrimSpace(ownerName) == "" {
		ownerName = "Adrian"
	}
	if strings.TrimSpace(ownerEmail) == "" {
		ownerEmail = "adrian@localhost"
	}
	a := s.Accounts()
	issued, err := a.Bootstrap(ctx, ownerName, ownerEmail, tokenName, secret)
	if err != nil {
		return "", false, err
	}
	// An owner from before a default cap existed gets it on the next boot.
	owner, err := a.Store().FirstUser(ctx)
	if err != nil {
		return "", false, err
	}
	return issued, issued != "", s.ensureDefaultCaps(ctx, owner.ID)
}

// LoginTokenName names the token a dashboard sign-in mints (OIDC or Better
// Auth); sign-out revokes only tokens with this name.
const LoginTokenName = "login"

// LoginIdleTTL is how long a dashboard sign-in survives without a request.
// Each request refreshes it, so an in-use session never lapses.
const LoginIdleTTL = 30 * 24 * time.Hour

// LoginMaxTTL caps a dashboard sign-in however busy it is. A fresh sign-in
// (including a step-up) starts a new session, so this is "days since you last
// proved who you are".
const LoginMaxTTL = 90 * 24 * time.Hour

// StepUpWindow is how recently a dashboard sign-in must have authenticated to
// approve, or to mint an approve token, when the user has a passkey.
const StepUpWindow = account.StepUpWindow

// CreateDemoSession provisions a throwaway tenant for the "try it without
// an account" tutorial: a synthetic, unroutable user plus a token scoped
// like the owner's, pre-loaded with the example seed data so it isn't
// empty. Demo mode is just a token minted without proving who you are
// first, not a separate auth path: discarding the token (or minting a new
// one, "replay tutorial") abandons this tenant.
func (s *Service) CreateDemoSession(ctx context.Context) (secret string, actor domain.Actor, err error) {
	return s.Accounts().CreateDemoSession(ctx)
}

// demoTTL is how long a throwaway demo tenant lives before SweepDemo removes it.
const demoTTL = 24 * time.Hour

// SweepDemo deletes demo tenants older than demoTTL and returns how many.
func (s *Service) SweepDemo(ctx context.Context) (int, error) {
	return s.DB.DeleteDemoUsers(ctx, s.now().Add(-demoTTL))
}

type CompanyInput struct {
	Name    string `json:"name"`
	Website string `json:"website"`
	IconURL string `json:"icon_url"`
	Notes   string `json:"notes"`
}

type CompanyPatch struct {
	Version int     `json:"version"`
	Name    *string `json:"name"`
	Website *string `json:"website"`
	IconURL *string `json:"icon_url"`
	Notes   *string `json:"notes"`
}

func (s *Service) CreateCompany(ctx context.Context, in CompanyInput) (domain.Company, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Company{}, err
	}
	c, err := s.newCompany(a, in.Name, in.Website, in.Notes)
	if err != nil {
		return domain.Company{}, err
	}
	if c.IconURL, err = domain.ParseURL(in.IconURL); err != nil {
		return domain.Company{}, err
	}
	if existing, err := s.DB.GetCompanyByName(ctx, a.UserID, c.Name); err == nil {
		return domain.Company{}, domain.Conflict("company already exists", "reuse company_id or update the company").WithDetails(map[string]any{"id": existing.ID, "version": existing.Version})
	} else if !domain.IsCode(err, "not_found") {
		return domain.Company{}, err
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.CreateCompany(ctx, c); err != nil {
			return err
		}
		return s.audit(ctx, a, "company.create", "company", c.ID, "created "+c.Name, nil)
	})
	return c, err
}

func (s *Service) newCompany(a domain.Actor, name, website, notes string) (domain.Company, error) {
	var err error
	if name, err = domain.RequireText("name", name, 200); err != nil {
		return domain.Company{}, err
	}
	if website, err = domain.ParseURL(website); err != nil {
		return domain.Company{}, err
	}
	if notes, err = domain.BoundText("notes", notes, 20000); err != nil {
		return domain.Company{}, err
	}
	now := s.now()
	return domain.Company{ID: s.NewID(), UserID: a.UserID, Name: name, Website: website, Notes: notes, Version: 1, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Service) GetCompany(ctx context.Context, id string) (domain.Company, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return domain.Company{}, err
	}
	return s.DB.GetCompany(ctx, a.UserID, id)
}

func (s *Service) ListCompanies(ctx context.Context, q domain.ListQuery) (domain.Page[domain.Company], error) {
	return listPage(s, ctx, false, "company", q, s.DB.ListCompanies, func(c domain.Company) string {
		switch strings.TrimPrefix(q.Sort, "-") {
		case "updated_at":
			return domain.FormatTime(c.UpdatedAt)
		case "created_at":
			return domain.FormatTime(c.CreatedAt)
		default:
			return c.Name
		}
	}, func(c domain.Company) string { return c.ID })
}

func (s *Service) UpdateCompany(ctx context.Context, id string, p CompanyPatch) (domain.Company, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Company{}, err
	}
	if p.Version < 1 {
		return domain.Company{}, versionRequired("company")
	}
	var out domain.Company
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		cur, err := s.DB.GetCompany(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		if cur.Version != p.Version {
			return domain.VersionConflict("company", cur.Version)
		}
		next := cur
		changed := false
		if p.Name != nil && *p.Name != cur.Name {
			name, err := domain.RequireText("name", *p.Name, 200)
			if err != nil {
				return err
			}
			next.Name = name
			changed = true
		}
		if p.Website != nil && *p.Website != cur.Website {
			w, err := domain.ParseURL(*p.Website)
			if err != nil {
				return err
			}
			next.Website = w
			changed = true
		}
		if p.IconURL != nil && *p.IconURL != cur.IconURL {
			u, err := domain.ParseURL(*p.IconURL)
			if err != nil {
				return err
			}
			next.IconURL = u
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
		if !changed {
			out = cur
			return nil
		}
		next.UpdatedAt = s.now()
		if err := s.DB.UpdateCompany(ctx, next); err != nil {
			return err
		}
		if err := s.audit(ctx, a, "company.update", "company", id, "updated "+next.Name, nil); err != nil {
			return err
		}
		out, err = s.DB.GetCompany(ctx, a.UserID, id)
		return err
	})
	return out, err
}

func (s *Service) DeleteCompany(ctx context.Context, id string, version int) (domain.Deleted, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Deleted{}, err
	}
	if version < 1 {
		return domain.Deleted{}, versionRequired("company")
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		cur, err := s.DB.GetCompany(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		if err := s.DB.DeleteCompany(ctx, a.UserID, id, version); err != nil {
			return err
		}
		return s.audit(ctx, a, "company.delete", "company", id, "deleted "+cur.Name, nil)
	})
	if err != nil {
		return domain.Deleted{}, err
	}
	return domain.Deleted{ID: id, Deleted: true}, nil
}

type RoleInput struct {
	CompanyID  string `json:"company_id"`
	Company    string `json:"company"`
	Website    string `json:"website"`
	Title      string `json:"title"`
	Location   string `json:"location"`
	Salary     string `json:"salary"`
	Fit        string `json:"fit"`
	PostingURL string `json:"posting_url"`
	Source     string `json:"source"`
	JobType    string `json:"job_type"`
}

type RolePatch struct {
	Version    int     `json:"version"`
	Title      *string `json:"title"`
	Location   *string `json:"location"`
	Salary     *string `json:"salary"`
	Fit        *string `json:"fit"`
	PostingURL *string `json:"posting_url"`
	Source     *string `json:"source"`
	JobType    *string `json:"job_type"`
}

func (s *Service) CreateRole(ctx context.Context, in RoleInput) (domain.Role, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Role{}, err
	}
	var out domain.Role
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		company, _, err := s.ensureCompany(ctx, a, in.CompanyID, in.Company, in.Website)
		if err != nil {
			return err
		}
		role, err := s.buildRole(a, company, in, s.now())
		if err != nil {
			return err
		}
		if err := s.DB.CreateRole(ctx, role); err != nil {
			return err
		}
		if err := s.audit(ctx, a, "role.create", "role", role.ID, role.Title+" at "+company.Name, nil); err != nil {
			return err
		}
		out, err = s.DB.GetRole(ctx, a.UserID, role.ID)
		return err
	})
	return out, err
}

func (s *Service) buildRole(a domain.Actor, company domain.Company, in RoleInput, now time.Time) (domain.Role, error) {
	title, err := domain.RequireText("title", in.Title, 300)
	if err != nil {
		return domain.Role{}, err
	}
	loc, err := domain.BoundText("location", in.Location, 300)
	if err != nil {
		return domain.Role{}, err
	}
	salary, err := domain.BoundText("salary", in.Salary, 200)
	if err != nil {
		return domain.Role{}, err
	}
	fit, err := domain.ParseFit(in.Fit)
	if err != nil {
		return domain.Role{}, err
	}
	rawURL, err := domain.ParseURL(in.PostingURL)
	if err != nil {
		return domain.Role{}, err
	}
	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = domain.InferSource(rawURL)
	} else {
		source, err = domain.ParseSource(source)
		if err != nil {
			return domain.Role{}, err
		}
	}
	job := strings.TrimSpace(in.JobType)
	if job == "" {
		job = "Full-time"
	}
	if job, err = domain.BoundText("job_type", job, 80); err != nil {
		return domain.Role{}, err
	}
	min, max := domain.ParseSalary(salary)
	return domain.Role{
		ID: s.NewID(), UserID: a.UserID, CompanyID: company.ID, Company: company.Name,
		Title: title, Location: loc, Salary: salary, SalaryMinCents: min, SalaryMaxCents: max,
		Fit: fit, PostingURL: rawURL, Source: source, JobType: job, Version: 1, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *Service) GetRole(ctx context.Context, id string) (domain.Role, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return domain.Role{}, err
	}
	return s.DB.GetRole(ctx, a.UserID, id)
}

func (s *Service) ListRoles(ctx context.Context, q domain.ListQuery) (domain.Page[domain.Role], error) {
	if q.Fit != "" {
		fit, err := domain.ParseFit(q.Fit)
		if err != nil {
			return domain.Page[domain.Role]{}, err
		}
		q.Fit = fit
	}
	if q.Source != "" {
		src, err := domain.ParseSource(q.Source)
		if err != nil {
			return domain.Page[domain.Role]{}, err
		}
		q.Source = src
	}
	return listPage(s, ctx, false, "role", q, s.DB.ListRoles, func(r domain.Role) string {
		switch strings.TrimPrefix(q.Sort, "-") {
		case "company":
			return r.Company
		case "updated_at":
			return domain.FormatTime(r.UpdatedAt)
		case "created_at":
			return domain.FormatTime(r.CreatedAt)
		default:
			return r.Title
		}
	}, func(r domain.Role) string { return r.ID })
}

func (s *Service) UpdateRole(ctx context.Context, id string, p RolePatch) (domain.Role, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Role{}, err
	}
	if p.Version < 1 {
		return domain.Role{}, versionRequired("role")
	}
	var out domain.Role
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		cur, err := s.DB.GetRole(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		if cur.Version != p.Version {
			return domain.VersionConflict("role", cur.Version)
		}
		next, changed, err := applyRolePatch(cur, p)
		if err != nil {
			return err
		}
		if !changed {
			out = cur
			return nil
		}
		next.UpdatedAt = s.now()
		if err := s.DB.UpdateRole(ctx, next); err != nil {
			return err
		}
		if err := s.audit(ctx, a, "role.update", "role", id, "updated "+next.Title, nil); err != nil {
			return err
		}
		out, err = s.DB.GetRole(ctx, a.UserID, id)
		return err
	})
	return out, err
}

func applyRolePatch(cur domain.Role, p RolePatch) (domain.Role, bool, error) {
	next := cur
	changed := false
	var err error
	if p.Title != nil && strings.TrimSpace(*p.Title) != cur.Title {
		if next.Title, err = domain.RequireText("title", *p.Title, 300); err != nil {
			return domain.Role{}, false, err
		}
		changed = true
	}
	if p.Location != nil && strings.TrimSpace(*p.Location) != cur.Location {
		if next.Location, err = domain.BoundText("location", *p.Location, 300); err != nil {
			return domain.Role{}, false, err
		}
		changed = true
	}
	if p.Salary != nil && strings.TrimSpace(*p.Salary) != cur.Salary {
		if next.Salary, err = domain.BoundText("salary", *p.Salary, 200); err != nil {
			return domain.Role{}, false, err
		}
		next.SalaryMinCents, next.SalaryMaxCents = domain.ParseSalary(next.Salary)
		changed = true
	}
	if p.Fit != nil && strings.ToLower(strings.TrimSpace(*p.Fit)) != cur.Fit {
		if next.Fit, err = domain.ParseFit(*p.Fit); err != nil {
			return domain.Role{}, false, err
		}
		changed = true
	}
	if p.PostingURL != nil && strings.TrimSpace(*p.PostingURL) != cur.PostingURL {
		if next.PostingURL, err = domain.ParseURL(*p.PostingURL); err != nil {
			return domain.Role{}, false, err
		}
		changed = true
	}
	if p.Source != nil && strings.ToLower(strings.TrimSpace(*p.Source)) != cur.Source {
		if next.Source, err = domain.ParseSource(*p.Source); err != nil {
			return domain.Role{}, false, err
		}
		changed = true
	}
	if p.JobType != nil && strings.TrimSpace(*p.JobType) != cur.JobType {
		if next.JobType, err = domain.BoundText("job_type", *p.JobType, 80); err != nil {
			return domain.Role{}, false, err
		}
		if next.JobType == "" {
			next.JobType = "Full-time"
		}
		changed = true
	}
	return next, changed, nil
}

func (s *Service) DeleteRole(ctx context.Context, id string, version int) (domain.Deleted, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Deleted{}, err
	}
	if version < 1 {
		return domain.Deleted{}, versionRequired("role")
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		cur, err := s.DB.GetRole(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		if err := s.DB.DeleteRole(ctx, a.UserID, id, version); err != nil {
			return err
		}
		return s.audit(ctx, a, "role.delete", "role", id, "deleted "+cur.Title, nil)
	})
	if err != nil {
		return domain.Deleted{}, err
	}
	return domain.Deleted{ID: id, Deleted: true}, nil
}

func versionRequired(entity string) *domain.Error {
	return domain.Validation("version is required").WithHint("pass the version from the latest " + entity + " get or list response")
}
