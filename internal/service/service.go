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

	"github.com/pursuit/pursuit/internal/auth"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/links"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

type Service struct {
	DB         *sqlstore.Store
	Now        func() time.Time
	NewID      func() string
	HTTP       *http.Client
	Links      *links.Client
	Profile    map[string]string
	ResumeFile string
	// Hooks is invoked after an event is stored. It must not block the caller for long.
	Hooks func(userID string, ev domain.Event)
}

func New(db *sqlstore.Store) *Service {
	return &Service{DB: db, Now: time.Now, NewID: uuid.NewString}
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

func (s *Service) Authenticate(ctx context.Context, secret string) (domain.Actor, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return domain.Actor{}, domain.Unauthorized()
	}
	tok, err := s.DB.GetTokenByHash(ctx, auth.Hash(secret))
	if domain.IsCode(err, "not_found") || (err == nil && tok.RevokedAt != nil) {
		return domain.Actor{}, domain.Unauthorized()
	}
	if err != nil {
		return domain.Actor{}, err
	}
	now := s.now()
	_ = s.DB.TouchToken(ctx, tok.ID, domain.FormatTime(now), domain.FormatTime(now.Add(-time.Minute)))
	return domain.Actor{UserID: tok.UserID, TokenID: tok.ID, Name: tok.Name, Scopes: tok.Scopes, Demo: tok.Name == demoTokenName}, nil
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
	if strings.TrimSpace(tokenName) == "" {
		tokenName = "owner"
	}
	var issued string
	var created bool
	err := s.DB.WithLock(ctx, func(ctx context.Context) error {
		n, err := s.DB.CountUsers(ctx)
		if err != nil {
			return err
		}
		var user domain.User
		if n == 0 {
			user = domain.User{ID: s.NewID(), Name: ownerName, Email: ownerEmail, CreatedAt: s.now()}
			if err := s.DB.CreateUser(ctx, user); err != nil {
				return err
			}
		} else {
			user, err = s.DB.FirstUser(ctx)
			if err != nil {
				return err
			}
		}
		tn, err := s.DB.CountTokens(ctx)
		if err != nil {
			return err
		}
		if err := s.ensureDefaultCaps(ctx, user.ID); err != nil {
			return err
		}
		if tn > 0 {
			return nil
		}
		if strings.TrimSpace(secret) == "" {
			secret, _, _, err = auth.Generate()
			if err != nil {
				return err
			}
		}
		scopes, err := auth.NormalizeScopes([]string{"read", "write", "approve"})
		if err != nil {
			return err
		}
		tok := domain.Token{
			ID: s.NewID(), UserID: user.ID, Name: tokenName, Hash: auth.Hash(secret),
			Prefix: auth.Prefix(secret), Scopes: scopes, CreatedAt: s.now(),
		}
		if err := s.DB.CreateToken(ctx, tok); err != nil {
			return err
		}
		actor := domain.Actor{UserID: user.ID, TokenID: tok.ID, Name: tok.Name, Scopes: scopes}
		if err := s.audit(domain.WithActor(ctx, actor), actor, "token.create", "token", tok.ID, "bootstrap token "+tok.Name, map[string]any{"prefix": tok.Prefix}); err != nil {
			return err
		}
		issued = secret
		created = true
		return nil
	})
	return issued, created, err
}

// demoTokenName marks a token as belonging to a throwaway tutorial tenant.
// Authenticate reads it back to set Actor.Demo on every later request, not
// just the one that minted the session — a plain token name, not a new
// schema column, since it's an informational flag rather than a security
// boundary (sqlstore already isolates all data by UserID regardless).
const demoTokenName = "demo"

// CreateDemoSession provisions a throwaway tenant for the "try it without
// an account" tutorial: a synthetic, unroutable user plus a token scoped
// like the owner's, pre-loaded with the example seed data so it isn't
// empty. The caller hands the returned secret back to the browser exactly
// like any other token (localStorage, same sign-in flow) — demo mode is
// just a token minted without proving who you are first, not a separate
// auth path. Nothing here is persisted beyond that token: discarding it
// (or minting a new one — e.g. "replay tutorial") abandons this tenant.
func (s *Service) CreateDemoSession(ctx context.Context) (secret string, actor domain.Actor, err error) {
	user := domain.User{ID: s.NewID(), Name: "Demo visitor", Email: "demo-" + s.NewID() + "@pursuit.invalid", CreatedAt: s.now()}
	if err := s.DB.CreateUser(ctx, user); err != nil {
		return "", domain.Actor{}, err
	}
	if err := s.ensureDefaultCaps(ctx, user.ID); err != nil {
		return "", domain.Actor{}, err
	}
	secret, prefix, hash, err := auth.Generate()
	if err != nil {
		return "", domain.Actor{}, err
	}
	scopes, err := auth.NormalizeScopes([]string{"read", "write", "approve"})
	if err != nil {
		return "", domain.Actor{}, err
	}
	tok := domain.Token{ID: s.NewID(), UserID: user.ID, Name: demoTokenName, Hash: hash, Prefix: prefix, Scopes: scopes, CreatedAt: s.now()}
	if err := s.DB.CreateToken(ctx, tok); err != nil {
		return "", domain.Actor{}, err
	}
	actor = domain.Actor{UserID: user.ID, TokenID: tok.ID, Name: tok.Name, Scopes: scopes, Demo: true}
	if _, err := s.ImportSeed(domain.WithActor(ctx, actor)); err != nil {
		return "", domain.Actor{}, err
	}
	return secret, actor, nil
}

// ResolveTenantUser maps a verified Better Auth identity to a Pursuit
// actor on every request (unlike LoginIdentity, which runs once at
// login): it reads the existing user without locking on the hot path,
// and only takes the lock to create one on a first-ever verified email.
func (s *Service) ResolveTenantUser(ctx context.Context, email, name string, allowCreate bool) (domain.Actor, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return domain.Actor{}, domain.Unauthorized()
	}
	user, err := s.DB.FindUserByEmail(ctx, email)
	if err != nil && !domain.IsCode(err, "not_found") {
		return domain.Actor{}, err
	}
	if domain.IsCode(err, "not_found") {
		if !allowCreate {
			return domain.Actor{}, domain.Unauthorized()
		}
		if err := s.DB.WithLock(ctx, func(ctx context.Context) error {
			if existing, ferr := s.DB.FindUserByEmail(ctx, email); ferr == nil {
				user = existing
				return nil
			} else if !domain.IsCode(ferr, "not_found") {
				return ferr
			}
			if strings.TrimSpace(name) == "" {
				name = email
			}
			user = domain.User{ID: s.NewID(), Name: name, Email: email, CreatedAt: s.now()}
			if err := s.DB.CreateUser(ctx, user); err != nil {
				return err
			}
			return s.ensureDefaultCaps(ctx, user.ID)
		}); err != nil {
			return domain.Actor{}, err
		}
	}
	scopes, err := auth.NormalizeScopes([]string{"read", "write", "approve"})
	if err != nil {
		return domain.Actor{}, err
	}
	return domain.Actor{UserID: user.ID, Name: "betterauth", Scopes: scopes}, nil
}

// LoginIdentity exchanges a verified external identity (from an OIDC
// callback) for a fresh Pursuit API token. allowCreate is false in
// single-tenant mode, where the email must already belong to the owner;
// it is true in multi-tenant mode, where a first-time verified email
// provisions a new tenant.
func (s *Service) LoginIdentity(ctx context.Context, email, name string, allowCreate bool) (secret string, err error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", domain.Unauthorized()
	}
	err = s.DB.WithLock(ctx, func(ctx context.Context) error {
		user, err := s.DB.FindUserByEmail(ctx, email)
		if domain.IsCode(err, "not_found") {
			if !allowCreate {
				return domain.Unauthorized()
			}
			if strings.TrimSpace(name) == "" {
				name = email
			}
			user = domain.User{ID: s.NewID(), Name: name, Email: email, CreatedAt: s.now()}
			if err := s.DB.CreateUser(ctx, user); err != nil {
				return err
			}
			if err := s.ensureDefaultCaps(ctx, user.ID); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		scopes, err := auth.NormalizeScopes([]string{"read", "write", "approve"})
		if err != nil {
			return err
		}
		raw, prefix, hash, err := auth.Generate()
		if err != nil {
			return err
		}
		tok := domain.Token{
			ID: s.NewID(), UserID: user.ID, Name: "login", Hash: hash,
			Prefix: prefix, Scopes: scopes, CreatedAt: s.now(),
		}
		if err := s.DB.CreateToken(ctx, tok); err != nil {
			return err
		}
		actor := domain.Actor{UserID: user.ID, TokenID: tok.ID, Name: tok.Name, Scopes: scopes}
		if err := s.audit(domain.WithActor(ctx, actor), actor, "token.create", "token", tok.ID, "oidc login token", map[string]any{"prefix": tok.Prefix}); err != nil {
			return err
		}
		secret = raw
		return nil
	})
	return secret, err
}

type TokenInput struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

func (s *Service) CreateToken(ctx context.Context, in TokenInput) (domain.IssuedToken, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.IssuedToken{}, err
	}
	name, err := domain.RequireText("name", in.Name, 80)
	if err != nil {
		return domain.IssuedToken{}, err
	}
	scopes, err := auth.NormalizeScopes(in.Scopes)
	if err != nil {
		return domain.IssuedToken{}, domain.Validation("%s", err.Error()).WithHint("scopes are read and write; write implies read")
	}
	secret, prefix, hash, err := auth.Generate()
	if err != nil {
		return domain.IssuedToken{}, err
	}
	tok := domain.Token{ID: s.NewID(), UserID: a.UserID, Name: name, Hash: hash, Prefix: prefix, Scopes: scopes, CreatedAt: s.now()}
	var out domain.IssuedToken
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.CreateToken(ctx, tok); err != nil {
			return err
		}
		if err := s.audit(ctx, a, "token.create", "token", tok.ID, "created token "+name, map[string]any{"prefix": prefix, "scopes": scopes}); err != nil {
			return err
		}
		tok.Hash = ""
		out = domain.IssuedToken{Token: tok, Secret: secret}
		return nil
	})
	return out, err
}

func (s *Service) ListTokens(ctx context.Context, q domain.ListQuery) (domain.Page[domain.Token], error) {
	return listPage(s, ctx, false, "token", q, func(ctx context.Context, user string, q domain.ListQuery) ([]domain.Token, int, error) {
		items, total, err := s.DB.ListTokens(ctx, user, q)
		for i := range items {
			items[i].Hash = ""
		}
		return items, total, err
	}, func(t domain.Token) string {
		if strings.TrimPrefix(q.Sort, "-") == "name" || strings.HasSuffix(q.Sort, "name") && !strings.Contains(q.Sort, "created") {
			return t.Name
		}
		return domain.FormatTime(t.CreatedAt)
	}, func(t domain.Token) string { return t.ID })
}

func (s *Service) RevokeToken(ctx context.Context, id string) (domain.Token, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return domain.Token{}, err
	}
	var out domain.Token
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		changed, err := s.DB.RevokeToken(ctx, a.UserID, id, domain.FormatTime(s.now()))
		if err != nil {
			return err
		}
		out, err = s.DB.GetToken(ctx, a.UserID, id)
		if err != nil {
			return err
		}
		out.Hash = ""
		if changed {
			return s.audit(ctx, a, "token.revoke", "token", id, "revoked token "+out.Name, nil)
		}
		return nil
	})
	return out, err
}

func (s *Service) Me(ctx context.Context) (domain.Me, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return domain.Me{}, err
	}
	user, err := s.DB.GetUser(ctx, a.UserID)
	if err != nil {
		return domain.Me{}, err
	}
	if strings.TrimSpace(a.TokenID) == "" {
		return domain.Me{User: user, Token: domain.Token{UserID: user.ID, Name: a.Name, Scopes: append([]string(nil), a.Scopes...)}}, nil
	}
	tok, err := s.DB.GetToken(ctx, a.UserID, a.TokenID)
	if err != nil {
		return domain.Me{}, err
	}
	tok.Hash = ""
	return domain.Me{User: user, Token: tok}, nil
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
