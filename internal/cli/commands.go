package cli

import (
	"bytes"
	"cmp"
	"encoding/json"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/apiclient"
	"github.com/pursuit/pursuit/internal/app"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/service"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

func rootCommand() *Command {
	return &Command{
		Name: "pursuit", Summary: "job application tracker",
		Usage: "pursuit <command> [flags]",
		Example: "pursuit apps list --status to_apply --fields id,company,title,status",
		Children: []*Command{
			cmdServe(),
			{Name: "version", Summary: "print the CLI version", Run: func(e *exec) error { localVersion(e.Stdout, e.JSON); return nil }},
			{Name: "health", Summary: "check liveness", Run: func(e *exec) error { return e.Send("GET", "/healthz", nil, nil, "", false, false) }},
			{Name: "ready", Summary: "check readiness", Run: func(e *exec) error { return e.Send("GET", "/readyz", nil, nil, "", false, false) }},
			cmdApps(),
			{Name: "list", Summary: "list applications", Usage: "pursuit list [flags]", Example: "pursuit list --stale --fields id,company,title,link_status", Flags: appListFlags(), Run: runAppList},
			cmdRoles(),
			cmdCompanies(),
			cmdEvents(),
			cmdTags(),
			cmdFields(),
			{Name: "rounds", Summary: "rounds within any of an application's stages", Children: roundCommands()},
			{Name: "interviews", Summary: "alias of rounds", Children: roundCommands()},
			cmdTasks(),
			cmdNotes(),
			{Name: "questions", Summary: "the question bank: every question in your round notes", Children: []*Command{
				{Name: "list", Summary: "list questions across applications", Usage: "pursuit questions list [--q text]", Example: "pursuit questions list --q \"why snowflake\" --fields prompt,answer,company", Flags: []Flag{{Name: "q", Type: "string", Help: "filter prompt, answer, company and title"}, {Name: "limit", Type: "string", Help: "page size"}, {Name: "cursor", Type: "string", Help: "cursor"}}, Run: func(e *exec) error {
					q := url.Values{}
					for _, k := range []string{"q", "limit", "cursor"} {
						if v := e.Flag(k); v != "" {
							q.Set(k, v)
						}
					}
					return e.Send("GET", "/v1/questions", q, nil, "", false, true)
				}},
			}},
			cmdAuth(),
			cmdTokens(),
			cmdAudit(),
			cmdImport(),
			cmdExport(),
			cmdApply(),
			cmdApprove(),
			cmdReject(),
			cmdQueue(),
			cmdArtifacts(),
			cmdStanding(),
			cmdDocuments(),
			cmdCaps(),
			cmdCheckLinks(),
			cmdLinks(),
			cmdWebhooks(),
			cmdBatch(),
		},
	}
}

func cmdServe() *Command {
	return &Command{
		Name: "serve", Summary: "run the daemon",
		Usage: "pursuit serve [flags]",
		Example: "pursuit serve --database-url sqlite://$HOME/.local/share/pursuit/pursuit.db",
		Flags: []Flag{
			{Name: "addr", Type: "string", Help: "listen address"},
			{Name: "database-url", Type: "string", Help: "postgres or sqlite URL"},
			{Name: "log-level", Type: "string", Help: "debug, info, warn, or error"},
			{Name: "log-format", Type: "string", Help: "json or text"},
			{Name: "owner-name", Type: "string", Help: "owner name created on an empty database"},
			{Name: "owner-email", Type: "string", Help: "owner email created on an empty database"},
			{Name: "bootstrap-token", Type: "string", Help: "token secret to install if none exist"},
			{Name: "bootstrap-name", Type: "string", Help: "name of the bootstrap token"},
			{Name: "cors-origins", Type: "string", Help: "comma-separated browser origins"},
			{Name: "shutdown-timeout", Type: "string", Help: "graceful shutdown seconds"},
			{Name: "check-interval", Type: "string", Help: "link sweep interval seconds"},
			{Name: "profile", Type: "string", Help: "JSON file of standing answers"},
			{Name: "resume-file", Type: "string", Help: "resume PDF copied onto prepare"},
		},
		Run: func(e *exec) error {
			cfg := e.cfg
			if v := e.Flag("addr"); v != "" {
				cfg.Addr = v
			}
			if v := e.Flag("database-url"); v != "" {
				cfg.DatabaseURL = v
			}
			if v := e.Flag("log-level"); v != "" {
				cfg.LogLevel = v
			}
			if v := e.Flag("log-format"); v != "" {
				cfg.LogFormat = v
			}
			if v := e.Flag("owner-name"); v != "" {
				cfg.OwnerName = v
			}
			if v := e.Flag("owner-email"); v != "" {
				cfg.OwnerEmail = v
			}
			if v := e.Flag("bootstrap-token"); v != "" {
				cfg.BootstrapToken = v
			}
			if v := e.Flag("bootstrap-name"); v != "" {
				cfg.BootstrapName = v
			}
			if v := e.Flag("cors-origins"); v != "" {
				cfg.CORSOrigins = strings.Split(v, ",")
			}
			if v := e.Flag("shutdown-timeout"); v != "" {
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 {
					return &exitError{Code: 2, Message: "invalid --shutdown-timeout", Hint: "pass seconds"}
				}
				cfg.ShutdownTimeout = time.Duration(n) * time.Second
			}
			if v := e.Flag("check-interval"); v != "" {
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 {
					return &exitError{Code: 2, Message: "invalid --check-interval", Hint: "pass seconds"}
				}
				cfg.CheckInterval = time.Duration(n) * time.Second
			}
			if v := e.Flag("profile"); v != "" {
				cfg.ProfilePath = v
			}
			if v := e.Flag("resume-file"); v != "" {
				cfg.ResumeFile = v
			}
			if err := app.Run(context.Background(), cfg); err != nil {
				return &exitError{Code: 1, Message: err.Error(), Hint: "check the database URL and address"}
			}
			return nil
		},
	}
}

func appListFlags() []Flag {
	return []Flag{
		{Name: "status", Type: "string", Help: "filter by status", Repeatable: true},
		{Name: "status-not", Type: "string", Help: "exclude a status", Repeatable: true},
		{Name: "company", Type: "string", Help: "filter by company name"},
		{Name: "fit", Type: "string", Help: "strong, reach, or maybe"},
		{Name: "source", Type: "string", Help: "greenhouse, ashby, lever, workday, company_site, other"},
		{Name: "tag", Type: "string", Help: "filter by tag"},
		{Name: "q", Type: "string", Help: "search company, title, notes, location, next step"},
		{Name: "stale", Type: "bool", Help: "only postings the link checker marked closed"},
		{Name: "archived", Type: "bool", Help: "list archived applications instead of the default view"},
		{Name: "limit", Type: "string", Help: "page size, 1-200, default 50"},
		{Name: "cursor", Type: "string", Help: "next_cursor from the previous page"},
		{Name: "sort", Type: "string", Help: "field or -field, default -updated_at"},
	}
}

func cmdApps() *Command {
	return &Command{
		Name: "apps", Summary: "applications",
		Children: []*Command{
			{Name: "list", Summary: "list applications", Usage: "pursuit apps list [flags]", Example: "pursuit apps list --status applied --fields id,company,title,status", Flags: appListFlags(), Run: runAppList},
			{Name: "get", Summary: "get one application", Usage: "pursuit apps get <id>", Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
				return e.Send("GET", "/v1/applications/"+url.PathEscape(e.Args[0]), nil, nil, "", false, true)
			}},
			{Name: "create", Summary: "create an application", Mutating: true, Usage: "pursuit apps create --company <name> --title <title>", Example: "pursuit apps create --company Example --title Engineer --status to_apply", Flags: []Flag{
				{Name: "company", Type: "string", Help: "company name"},
				{Name: "company-id", Type: "string", Help: "existing company id"},
				{Name: "title", Type: "string", Help: "role title"},
				{Name: "location", Type: "string", Help: "location"},
				{Name: "salary", Type: "string", Help: "salary text"},
				{Name: "fit", Type: "string", Help: "strong, reach, or maybe"},
				{Name: "posting-url", Type: "string", Help: "posting URL"},
				{Name: "source", Type: "string", Help: "source override"},
				{Name: "status", Type: "string", Help: "initial status, default saved"},
				{Name: "applied-date", Type: "string", Help: "YYYY-MM-DD"},
				{Name: "next-step", Type: "string", Help: "next step"},
				{Name: "notes", Type: "string", Help: "notes"},
				{Name: "contact-email", Type: "string", Help: "contact email"},
			}, Run: func(e *exec) error {
				body, err := e.JSONBody(map[string]any{
					"company": e.Flag("company"), "company_id": e.Flag("company-id"), "title": e.Flag("title"),
					"location": e.Flag("location"), "salary": e.Flag("salary"), "fit": e.Flag("fit"),
					"posting_url": e.Flag("posting-url"), "source": e.Flag("source"), "status": e.Flag("status"),
					"applied_date": e.Flag("applied-date"), "next_step": e.Flag("next-step"), "notes": e.Flag("notes"),
					"contact_email": e.Flag("contact-email"),
				})
				if err != nil {
					return err
				}
				return e.Send("POST", "/v1/applications", nil, body, "application/json", true, true)
			}},
			{Name: "update", Summary: "update an application", Mutating: true, Usage: "pursuit apps update <id> [flags]", Example: "pursuit apps update APP --status interviewing --next-step 'Schedule call'", Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{
				{Name: "version", Type: "string", Help: "required if you already know it; otherwise the CLI fetches it"},
				{Name: "status", Type: "string", Help: "new status"},
				{Name: "next-step", Type: "string", Help: "next step"},
				{Name: "notes", Type: "string", Help: "notes"},
				{Name: "applied-date", Type: "string", Help: "YYYY-MM-DD, or null to clear"},
				{Name: "company", Type: "string", Help: "company name; moves the role, creating the company if new"},
				{Name: "title", Type: "string", Help: "role title"},
				{Name: "location", Type: "string", Help: "location"},
				{Name: "salary", Type: "string", Help: "salary text"},
				{Name: "fit", Type: "string", Help: "fit"},
				{Name: "posting-url", Type: "string", Help: "posting URL"},
				{Name: "resume-version", Type: "string", Help: "resume version label"},
				{Name: "contact-email", Type: "string", Help: "contact email"},
				{Name: "tracked-by", Type: "string", Help: "id of the application at the same company whose process covers this one; empty to clear"},
			}, Run: runAppUpdate},
			{Name: "delete", Summary: "delete an application", Mutating: true, Usage: "pursuit apps delete <id>", Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "object version"}}, Run: func(e *exec) error {
				return e.delete("/v1/applications/" + url.PathEscape(e.Args[0]))
			}},
			{Name: "notes", Summary: "print an application's compiled notes as markdown: its notes, then each round in notes", Usage: "pursuit apps notes <id> > notes.md", Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
				resp, err := e.client.Do(nil, "GET", "/v1/applications/"+url.PathEscape(e.Args[0])+"/notes", nil, nil, "", apiclient.Options{})
				if err != nil {
					return &exitError{Code: 1, Message: err.Error(), Hint: "check that pursuit serve is running and PURSUIT_SERVER is correct"}
				}
				if resp.Status >= 400 {
					return &exitError{Code: exitStatus(resp.Status), Raw: resp.Body}
				}
				var doc struct {
					Markdown string `json:"markdown"`
				}
				if err := json.Unmarshal(resp.Body, &doc); err != nil {
					return err
				}
				_, err = fmt.Fprint(e.Stdout, doc.Markdown)
				return err
			}},
			{Name: "transition", Summary: "change status and append an event", Mutating: true, Usage: "pursuit apps transition <id> --status <status>", Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{
				{Name: "status", Type: "string", Help: "target status"},
				{Name: "version", Type: "string", Help: "object version"},
				{Name: "next-step", Type: "string", Help: "next step"},
				{Name: "note", Type: "string", Help: "event note"},
			}, Run: func(e *exec) error {
				ver, err := e.ensureVersion("/v1/applications/" + url.PathEscape(e.Args[0]))
				if err != nil {
					return err
				}
				payload := map[string]any{"version": ver, "status": e.Flag("status"), "note": e.Flag("note")}
				if v := e.Flag("next-step"); v != "" {
					payload["next_step"] = v
				}
				body, err := e.JSONBody(payload)
				if err != nil {
					return err
				}
				return e.Send("POST", "/v1/applications/"+url.PathEscape(e.Args[0])+"/transition", nil, body, "application/json", true, true)
			}},
			{Name: "archive", Summary: "hide an application from the default list without changing its status", Mutating: true, Usage: "pursuit apps archive <id>", Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "object version"}}, Run: func(e *exec) error {
				ver, err := e.ensureVersion("/v1/applications/" + url.PathEscape(e.Args[0]))
				if err != nil {
					return err
				}
				body, err := e.JSONBody(map[string]any{"version": ver})
				if err != nil {
					return err
				}
				return e.Send("POST", "/v1/applications/"+url.PathEscape(e.Args[0])+"/archive", nil, body, "application/json", true, true)
			}},
			{Name: "unarchive", Summary: "restore an archived application to the default list", Mutating: true, Usage: "pursuit apps unarchive <id>", Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "object version"}}, Run: func(e *exec) error {
				ver, err := e.ensureVersion("/v1/applications/" + url.PathEscape(e.Args[0]))
				if err != nil {
					return err
				}
				body, err := e.JSONBody(map[string]any{"version": ver})
				if err != nil {
					return err
				}
				return e.Send("POST", "/v1/applications/"+url.PathEscape(e.Args[0])+"/unarchive", nil, body, "application/json", true, true)
			}},
			{Name: "tag", Summary: "application tags", Children: []*Command{
				{Name: "add", Summary: "attach a tag", Mutating: true, Usage: "pursuit apps tag add <id> --name <tag>", Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "name", Type: "string", Help: "tag name"}, {Name: "tag-id", Type: "string", Help: "existing tag id"}}, Run: func(e *exec) error {
					body, err := e.JSONBody(map[string]string{"name": e.Flag("name"), "tag_id": e.Flag("tag-id")})
					if err != nil {
						return err
					}
					return e.Send("POST", "/v1/applications/"+url.PathEscape(e.Args[0])+"/tags", nil, body, "application/json", true, true)
				}},
				{Name: "remove", Summary: "detach a tag", Mutating: true, Usage: "pursuit apps tag remove <id> <tag-id>", Args: []Arg{{Name: "id", Required: true}, {Name: "tag-id", Required: true}}, Run: func(e *exec) error {
					return e.Send("DELETE", "/v1/applications/"+url.PathEscape(e.Args[0])+"/tags/"+url.PathEscape(e.Args[1]), nil, nil, "", true, true)
				}},
			}},
			{Name: "set-field", Summary: "set a custom field's value", Mutating: true, Usage: "pursuit apps set-field <id> <field-id> --value <value>", Args: []Arg{{Name: "id", Required: true}, {Name: "field-id", Required: true}}, Flags: []Flag{{Name: "value", Type: "string", Help: "the value; empty clears it"}}, Run: func(e *exec) error {
				body, err := e.JSONBody(map[string]string{"value": e.Flag("value")})
				if err != nil {
					return err
				}
				return e.Send("POST", "/v1/applications/"+url.PathEscape(e.Args[0])+"/fields/"+url.PathEscape(e.Args[1]), nil, body, "application/json", true, true)
			}},
		},
	}
}

func runAppList(e *exec) error {
	q := url.Values{}
	for _, st := range e.Flags["status"] {
		q.Add("status", st)
	}
	for _, st := range e.Flags["status-not"] {
		q.Add("status_not", st)
	}
	for _, k := range []string{"company", "fit", "source", "tag", "q", "limit", "cursor", "sort"} {
		if v := e.Flag(k); v != "" {
			q.Set(strings.ReplaceAll(k, "-", "_"), v)
		}
	}
	if e.Flag("stale") == "true" {
		q.Set("stale", "true")
	}
	if e.Flag("archived") == "true" {
		q.Set("archived", "true")
	}
	return e.Send("GET", "/v1/applications", q, nil, "", false, true)
}

func runAppUpdate(e *exec) error {
	id := url.PathEscape(e.Args[0])
	ver, err := e.ensureVersion("/v1/applications/" + id)
	if err != nil {
		return err
	}
	payload := map[string]any{"version": ver}
	put := func(jsonKey, flag string) {
		if _, ok := e.Flags[flag]; ok {
			payload[jsonKey] = e.Flag(flag)
		}
	}
	put("status", "status")
	put("next_step", "next-step")
	put("notes", "notes")
	put("company", "company")
	put("title", "title")
	put("location", "location")
	put("salary", "salary")
	put("fit", "fit")
	put("posting_url", "posting-url")
	put("resume_version", "resume-version")
	put("contact_email", "contact-email")
	put("tracked_by", "tracked-by")
	if _, ok := e.Flags["applied-date"]; ok {
		if strings.EqualFold(e.Flag("applied-date"), "null") {
			payload["applied_date"] = nil
		} else {
			payload["applied_date"] = e.Flag("applied-date")
		}
	}
	body, err := e.JSONBody(payload)
	if err != nil {
		return err
	}
	return e.Send("PATCH", "/v1/applications/"+id, nil, body, "application/json", true, true)
}

func (e *exec) ensureVersion(path string) (int, error) {
	if v := e.Flag("version"); v != "" {
		return atoiFlag(v)
	}
	return e.VersionOf(path)
}

func (e *exec) delete(path string) error {
	ver, err := e.ensureVersion(path)
	if err != nil {
		return err
	}
	q := url.Values{"version": {strconv.Itoa(ver)}}
	return e.Send("DELETE", path, q, nil, "", true, false)
}

func cmdCompanies() *Command {
	return &Command{Name: "companies", Summary: "companies", Children: []*Command{
		{Name: "list", Summary: "list companies", Flags: []Flag{{Name: "q", Type: "string", Help: "search"}, {Name: "limit", Type: "string", Help: "page size"}, {Name: "cursor", Type: "string", Help: "cursor"}, {Name: "sort", Type: "string", Help: "sort"}}, Run: func(e *exec) error {
			return e.Send("GET", "/v1/companies", listQ(e), nil, "", false, true)
		}},
		{Name: "get", Summary: "get a company", Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			return e.Send("GET", "/v1/companies/"+url.PathEscape(e.Args[0]), nil, nil, "", false, true)
		}},
		{Name: "create", Summary: "create a company", Mutating: true, Flags: []Flag{{Name: "name", Type: "string", Help: "name"}, {Name: "website", Type: "string", Help: "website"}, {Name: "icon-url", Type: "string", Help: "favicon override url"}, {Name: "notes", Type: "string", Help: "notes"}}, Run: func(e *exec) error {
			body, err := e.JSONBody(map[string]string{"name": e.Flag("name"), "website": e.Flag("website"), "icon_url": e.Flag("icon-url"), "notes": e.Flag("notes")})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/companies", nil, body, "application/json", true, true)
		}},
		{Name: "update", Summary: "update a company", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "version"}, {Name: "name", Type: "string", Help: "name"}, {Name: "website", Type: "string", Help: "website"}, {Name: "icon-url", Type: "string", Help: "favicon override url"}, {Name: "notes", Type: "string", Help: "notes"}}, Run: func(e *exec) error {
			ver, err := e.ensureVersion("/v1/companies/" + url.PathEscape(e.Args[0]))
			if err != nil {
				return err
			}
			payload := map[string]any{"version": ver}
			if _, ok := e.Flags["name"]; ok {
				payload["name"] = e.Flag("name")
			}
			if _, ok := e.Flags["website"]; ok {
				payload["website"] = e.Flag("website")
			}
			if _, ok := e.Flags["icon-url"]; ok {
				payload["icon_url"] = e.Flag("icon-url")
			}
			if _, ok := e.Flags["notes"]; ok {
				payload["notes"] = e.Flag("notes")
			}
			body, err := e.JSONBody(payload)
			if err != nil {
				return err
			}
			return e.Send("PATCH", "/v1/companies/"+url.PathEscape(e.Args[0]), nil, body, "application/json", true, true)
		}},
		{Name: "delete", Summary: "delete a company", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "version"}}, Run: func(e *exec) error {
			return e.delete("/v1/companies/" + url.PathEscape(e.Args[0]))
		}},
	}}
}

func cmdRoles() *Command {
	return &Command{Name: "roles", Summary: "roles", Children: []*Command{
		{Name: "list", Summary: "list roles", Flags: []Flag{{Name: "q", Type: "string", Help: "search"}, {Name: "company", Type: "string", Help: "company name"}, {Name: "fit", Type: "string", Help: "fit"}, {Name: "source", Type: "string", Help: "source"}, {Name: "limit", Type: "string", Help: "page size"}, {Name: "cursor", Type: "string", Help: "cursor"}, {Name: "sort", Type: "string", Help: "sort"}}, Run: func(e *exec) error {
			return e.Send("GET", "/v1/roles", listQ(e), nil, "", false, true)
		}},
		{Name: "get", Summary: "get a role", Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			return e.Send("GET", "/v1/roles/"+url.PathEscape(e.Args[0]), nil, nil, "", false, true)
		}},
		{Name: "create", Summary: "create a role", Mutating: true, Flags: []Flag{
			{Name: "company", Type: "string", Help: "company name"}, {Name: "company-id", Type: "string", Help: "company id"},
			{Name: "title", Type: "string", Help: "title"}, {Name: "location", Type: "string", Help: "location"},
			{Name: "salary", Type: "string", Help: "salary"}, {Name: "fit", Type: "string", Help: "fit"},
			{Name: "posting-url", Type: "string", Help: "posting URL"}, {Name: "source", Type: "string", Help: "source"},
		}, Run: func(e *exec) error {
			body, err := e.JSONBody(map[string]string{"company": e.Flag("company"), "company_id": e.Flag("company-id"), "title": e.Flag("title"), "location": e.Flag("location"), "salary": e.Flag("salary"), "fit": e.Flag("fit"), "posting_url": e.Flag("posting-url"), "source": e.Flag("source")})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/roles", nil, body, "application/json", true, true)
		}},
		{Name: "update", Summary: "update a role", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "version"}, {Name: "title", Type: "string", Help: "title"}, {Name: "location", Type: "string", Help: "location"}, {Name: "salary", Type: "string", Help: "salary"}, {Name: "fit", Type: "string", Help: "fit"}, {Name: "posting-url", Type: "string", Help: "posting URL"}}, Run: func(e *exec) error {
			ver, err := e.ensureVersion("/v1/roles/" + url.PathEscape(e.Args[0]))
			if err != nil {
				return err
			}
			payload := map[string]any{"version": ver}
			for _, pair := range [][2]string{{"title", "title"}, {"location", "location"}, {"salary", "salary"}, {"fit", "fit"}, {"posting_url", "posting-url"}} {
				if _, ok := e.Flags[pair[1]]; ok {
					payload[pair[0]] = e.Flag(pair[1])
				}
			}
			body, err := e.JSONBody(payload)
			if err != nil {
				return err
			}
			return e.Send("PATCH", "/v1/roles/"+url.PathEscape(e.Args[0]), nil, body, "application/json", true, true)
		}},
		{Name: "delete", Summary: "delete a role", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "version"}}, Run: func(e *exec) error {
			return e.delete("/v1/roles/" + url.PathEscape(e.Args[0]))
		}},
	}}
}

func listQ(e *exec) url.Values {
	q := url.Values{}
	for _, k := range []string{"q", "company", "fit", "source", "limit", "cursor", "sort", "actor", "action", "since", "until"} {
		if v := e.Flag(k); v != "" {
			q.Set(k, v)
		}
	}
	if v := e.Flag("entity-type"); v != "" {
		q.Set("entity_type", v)
	}
	if v := e.Flag("entity-id"); v != "" {
		q.Set("entity_id", v)
	}
	return q
}

func cmdEvents() *Command {
	return &Command{Name: "events", Summary: "timeline events", Usage: "pursuit events [flags]", Example: "pursuit events --follow --timeout 25s", Flags: []Flag{
		{Name: "application", Type: "string", Help: "application id"},
		{Name: "after", Type: "string", Help: "return events after this event id"},
		{Name: "follow", Type: "bool", Help: "long-poll until an event arrives or timeout"},
		{Name: "timeout", Type: "string", Help: "follow wait, default 25s"},
		{Name: "limit", Type: "string", Help: "page size"},
	}, Run: runEvents, Children: []*Command{
		{Name: "list", Summary: "list events", Flags: []Flag{{Name: "application", Type: "string", Help: "application id"}, {Name: "after", Type: "string", Help: "event id cursor"}, {Name: "follow", Type: "bool", Help: "long-poll"}, {Name: "timeout", Type: "string", Help: "wait"}, {Name: "limit", Type: "string", Help: "page size"}}, Run: runEvents},
		{Name: "add", Summary: "append an email, interview, or note", Mutating: true, Usage: "pursuit events add <application-id> --kind note --body <text>", Args: []Arg{{Name: "application-id", Required: true}}, Flags: []Flag{{Name: "kind", Type: "string", Help: "email_received, interview_scheduled, or note"}, {Name: "title", Type: "string", Help: "title"}, {Name: "body", Type: "string", Help: "body"}}, Run: func(e *exec) error {
			body, err := e.JSONBody(map[string]string{"kind": e.Flag("kind"), "title": e.Flag("title"), "body": e.Flag("body")})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/applications/"+url.PathEscape(e.Args[0])+"/events", nil, body, "application/json", true, true)
		}},
	}}
}

func runEvents(e *exec) error {
	q := url.Values{}
	if v := e.Flag("application"); v != "" {
		q.Set("application_id", v)
	}
	if v := e.Flag("after"); v != "" {
		q.Set("after", v)
	}
	if v := e.Flag("limit"); v != "" {
		q.Set("limit", v)
	}
	if e.Flag("follow") == "true" {
		wait := 25000
		if v := e.Flag("timeout"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				n, nerr := strconv.Atoi(v)
				if nerr != nil {
					return &exitError{Code: 2, Message: "invalid --timeout", Hint: "use a duration such as 25s"}
				}
				d = time.Duration(n) * time.Second
			}
			wait = int(d / time.Millisecond)
		}
		q.Set("wait_ms", strconv.Itoa(wait))
	}
	return e.Send("GET", "/v1/events", q, nil, "", false, true)
}

func cmdTags() *Command {
	return &Command{Name: "tags", Summary: "tags", Children: []*Command{
		{Name: "list", Summary: "list tags", Run: func(e *exec) error { return e.Send("GET", "/v1/tags", nil, nil, "", false, true) }},
		{Name: "create", Summary: "create a tag", Mutating: true, Flags: []Flag{{Name: "name", Type: "string", Help: "tag name"}}, Run: func(e *exec) error {
			body, err := e.JSONBody(map[string]string{"name": e.Flag("name")})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/tags", nil, body, "application/json", true, true)
		}},
		{Name: "delete", Summary: "delete a tag", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			return e.Send("DELETE", "/v1/tags/"+url.PathEscape(e.Args[0]), nil, nil, "", true, false)
		}},
	}}
}

func cmdFields() *Command {
	return &Command{Name: "fields", Summary: "custom application fields", Children: []*Command{
		{Name: "list", Summary: "list custom fields", Run: func(e *exec) error { return e.Send("GET", "/v1/fields", nil, nil, "", false, true) }},
		{Name: "create", Summary: "create a custom field", Mutating: true, Usage: "pursuit fields create --name <name> --type text|number|date", Flags: []Flag{{Name: "name", Type: "string", Help: "field name"}, {Name: "type", Type: "string", Help: "text, number, or date"}}, Run: func(e *exec) error {
			body, err := e.JSONBody(map[string]string{"name": e.Flag("name"), "type": e.Flag("type")})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/fields", nil, body, "application/json", true, true)
		}},
		{Name: "delete", Summary: "delete a custom field", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			return e.Send("DELETE", "/v1/fields/"+url.PathEscape(e.Args[0]), nil, nil, "", true, false)
		}},
	}}
}

// roundCommands are the children of rounds and of its alias interviews.
func roundCommands() []*Command {
	fields := []Flag{
		{Name: "stage", Type: "string", Help: "prepared, applied, screening, interviewing, or final (default: the application's current stage; an offer has no rounds)"},
		{Name: "kind", Type: "string", Help: "the round, e.g. Recruiter call or System design"},
		{Name: "interviewer", Type: "string", Help: "who you meet"},
		{Name: "when", Type: "string", Help: "RFC3339 or YYYY-MM-DD"},
		{Name: "prep", Type: "string", Help: "set the notes' ## Prep section"},
		{Name: "prep-file", Type: "string", Help: "read the ## Prep section from a file"},
		{Name: "notes", Type: "string", Help: "replace the whole markdown notes document"},
		{Name: "notes-file", Type: "string", Help: "replace the notes document from a file (- for stdin)"},
		{Name: "append", Type: "string", Help: "add text at the end of the notes, e.g. a transcript"},
		{Name: "question", Type: "string", Help: "set one question under ## Questions (a missing ? is added); use with --answer"},
		{Name: "answer", Type: "string", Help: "the answer for --question"},
		{Name: "in-notes", Type: "string", Help: "true or false: show the round expanded in the application's notes outline"},
	}
	read := func(path, flag string) (string, error) {
		var b []byte
		var err error
		if path == "-" {
			b, err = io.ReadAll(os.Stdin)
		} else {
			b, err = os.ReadFile(path)
		}
		if err != nil {
			return "", &exitError{Code: 2, Message: err.Error(), Hint: "check --" + flag}
		}
		return string(b), nil
	}
	// put copies a flag into the payload only when it was passed, so set
	// leaves everything else alone.
	put := func(e *exec, payload map[string]any) error {
		for key, flag := range map[string]string{"stage": "stage", "kind": "kind", "interviewer": "interviewer", "scheduled_at": "when", "prep": "prep", "notes": "notes", "append": "append"} {
			if _, ok := e.Flags[flag]; ok {
				payload[key] = e.Flag(flag)
			}
		}
		for key, flag := range map[string]string{"prep": "prep-file", "notes": "notes-file"} {
			if path := e.Flag(flag); path != "" {
				text, err := read(path, flag)
				if err != nil {
					return err
				}
				payload[key] = text
			}
		}
		if v, ok := e.Flags["in-notes"]; ok {
			payload["in_notes"] = len(v) == 0 || v[0] != "false"
		}
		if _, ok := e.Flags["question"]; ok {
			payload["set_question"] = map[string]string{"prompt": e.Flag("question"), "answer": e.Flag("answer")}
		}
		return nil
	}
	return []*Command{
		{Name: "list", Summary: "list an application's rounds", Usage: "pursuit rounds list <application-id>", Args: []Arg{{Name: "application-id", Required: true}}, Run: func(e *exec) error {
			return e.Send("GET", "/v1/applications/"+url.PathEscape(e.Args[0])+"/rounds", nil, nil, "", false, true)
		}},
		{Name: "add", Summary: "add a round", Mutating: true, Usage: "pursuit rounds add <application-id> --kind \"System design\" --when 2026-11-05 --prep-file prep.md", Args: []Arg{{Name: "application-id", Required: true}}, Flags: append(append([]Flag{}, fields...), Flag{Name: "done", Type: "bool", Help: "already happened"}), Run: func(e *exec) error {
			payload := map[string]any{}
			if err := put(e, payload); err != nil {
				return err
			}
			if e.Flag("done") == "true" {
				payload["done"] = true
			}
			body, err := e.JSONBody(payload)
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/applications/"+url.PathEscape(e.Args[0])+"/rounds", nil, body, "application/json", true, true)
		}},
		{Name: "get", Summary: "get a round, with its notes and parsed questions", Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			return e.Send("GET", "/v1/rounds/"+url.PathEscape(e.Args[0]), nil, nil, "", false, true)
		}},
		{Name: "notes", Summary: "print a round's notes as raw markdown", Usage: "pursuit rounds notes <id> > notes.md", Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			resp, err := e.client.Do(nil, "GET", "/v1/rounds/"+url.PathEscape(e.Args[0]), nil, nil, "", apiclient.Options{})
			if err != nil {
				return &exitError{Code: 1, Message: err.Error(), Hint: "check that pursuit serve is running and PURSUIT_SERVER is correct"}
			}
			if resp.Status >= 400 {
				return &exitError{Code: exitStatus(resp.Status), Raw: resp.Body}
			}
			var round struct {
				Notes string `json:"notes"`
			}
			if err := json.Unmarshal(resp.Body, &round); err != nil {
				return err
			}
			_, err = fmt.Fprintln(e.Stdout, round.Notes)
			return err
		}},
		{Name: "set", Summary: "change a round", Mutating: true, Usage: "pursuit rounds set <id> [--append ...] [--question ... --answer ...] [--notes-file notes.md] [--done|--undone]", Example: "pursuit rounds set ROUND_ID --question \"Why Snowflake?\" --answer \"Query engine work at scale.\"", Args: []Arg{{Name: "id", Required: true}}, Flags: append(append([]Flag{}, fields...), Flag{Name: "done", Type: "bool", Help: "mark done"}, Flag{Name: "undone", Type: "bool", Help: "mark not done"}, Flag{Name: "version", Type: "string", Help: "object version"}), Run: func(e *exec) error {
			return patchDone(e, "/v1/rounds/"+url.PathEscape(e.Args[0]), put)
		}},
		{Name: "done", Summary: "mark a round done", Mutating: true, Usage: "pursuit rounds done <id> [--notes ...]", Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "notes", Type: "string", Help: "added at the end of the notes"}, {Name: "version", Type: "string", Help: "object version"}}, Run: func(e *exec) error {
			e.Flags["done"] = []string{"true"}
			// Finishing a round adds to its notes; it never replaces them.
			if v, ok := e.Flags["notes"]; ok {
				e.Flags["append"] = v
				delete(e.Flags, "notes")
			}
			return patchDone(e, "/v1/rounds/"+url.PathEscape(e.Args[0]), put)
		}},
		{Name: "delete", Summary: "delete a round", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "object version"}}, Run: func(e *exec) error {
			return e.delete("/v1/rounds/" + url.PathEscape(e.Args[0]))
		}},
	}
}

// cmdNotes manages an application's notes files: markdown of their own,
// beside its notes and its rounds' notes.
func cmdNotes() *Command {
	put := func(e *exec, payload map[string]any) error {
		for key, flag := range map[string]string{"name": "name", "body": "body", "append": "append"} {
			if _, ok := e.Flags[flag]; ok {
				payload[key] = e.Flag(flag)
			}
		}
		if path := e.Flag("file"); path != "" {
			var b []byte
			var err error
			if path == "-" {
				b, err = io.ReadAll(os.Stdin)
			} else {
				b, err = os.ReadFile(path)
			}
			if err != nil {
				return &exitError{Code: 2, Message: err.Error(), Hint: "check --file"}
			}
			payload["body"] = string(b)
		}
		if _, ok := e.Flags["question"]; ok {
			payload["set_question"] = map[string]string{"prompt": e.Flag("question"), "answer": e.Flag("answer")}
		}
		return nil
	}
	fields := []Flag{
		{Name: "name", Type: "string", Help: "the file's name"},
		{Name: "body", Type: "string", Help: "replace the markdown"},
		{Name: "file", Type: "string", Help: "replace the markdown from a file (- for stdin)"},
		{Name: "append", Type: "string", Help: "add text at the end"},
		{Name: "question", Type: "string", Help: "set one question (a missing ? is added); use with --answer"},
		{Name: "answer", Type: "string", Help: "the answer for --question"},
	}
	return &Command{Name: "notes", Summary: "an application's notes files: markdown beside its notes and rounds", Children: []*Command{
		{Name: "list", Summary: "list an application's notes files", Usage: "pursuit notes list <application-id>", Args: []Arg{{Name: "application-id", Required: true}}, Run: func(e *exec) error {
			return e.Send("GET", "/v1/applications/"+url.PathEscape(e.Args[0])+"/notes/files", nil, nil, "", false, true)
		}},
		{Name: "add", Summary: "add a notes file", Mutating: true, Usage: "pursuit notes add <application-id> --name \"Salary research\" [--file notes.md]", Args: []Arg{{Name: "application-id", Required: true}}, Flags: fields, Run: func(e *exec) error {
			payload := map[string]any{}
			if err := put(e, payload); err != nil {
				return err
			}
			body, err := e.JSONBody(payload)
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/applications/"+url.PathEscape(e.Args[0])+"/notes/files", nil, body, "application/json", true, true)
		}},
		{Name: "get", Summary: "get a notes file", Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			return e.Send("GET", "/v1/notes/"+url.PathEscape(e.Args[0]), nil, nil, "", false, true)
		}},
		{Name: "set", Summary: "change a notes file", Mutating: true, Usage: "pursuit notes set <id> [--append ...] [--question ... --answer ...] [--file notes.md] [--name ...]", Args: []Arg{{Name: "id", Required: true}}, Flags: append(append([]Flag{}, fields...), Flag{Name: "version", Type: "string", Help: "object version"}), Run: func(e *exec) error {
			return patchDone(e, "/v1/notes/"+url.PathEscape(e.Args[0]), put)
		}},
		{Name: "delete", Summary: "delete a notes file", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "object version"}}, Run: func(e *exec) error {
			return e.delete("/v1/notes/" + url.PathEscape(e.Args[0]))
		}},
	}}
}

func cmdTasks() *Command {
	fields := []Flag{
		{Name: "title", Type: "string", Help: "what needs doing"},
		{Name: "stage", Type: "string", Help: "anytime (no stage), to_apply, prepared, applied, screening, interviewing, final, or offer (default: the application's current stage)"},
		{Name: "notes", Type: "string", Help: "details"},
		{Name: "due", Type: "string", Help: "RFC3339 or YYYY-MM-DD"},
		{Name: "round", Type: "string", Help: "round id: the task belongs to that round and takes its stage; empty to untie"},
	}
	put := func(e *exec, payload map[string]any) error {
		for key, flag := range map[string]string{"title": "title", "stage": "stage", "notes": "notes", "due_at": "due", "round_id": "round"} {
			if _, ok := e.Flags[flag]; ok {
				payload[key] = e.Flag(flag)
			}
		}
		return nil
	}
	return &Command{Name: "tasks", Summary: "to-dos on an application, things waiting on you", Children: []*Command{
		{Name: "list", Summary: "list an application's tasks", Usage: "pursuit tasks list <application-id>", Args: []Arg{{Name: "application-id", Required: true}}, Run: func(e *exec) error {
			return e.Send("GET", "/v1/applications/"+url.PathEscape(e.Args[0])+"/tasks", nil, nil, "", false, true)
		}},
		{Name: "add", Summary: "add a task", Mutating: true, Usage: "pursuit tasks add <application-id> --title \"Send references\" [--stage ...] [--due 2026-11-05]", Args: []Arg{{Name: "application-id", Required: true}}, Flags: append(append([]Flag{}, fields...), Flag{Name: "done", Type: "bool", Help: "already done"}), Run: func(e *exec) error {
			payload := map[string]any{}
			if err := put(e, payload); err != nil {
				return err
			}
			if e.Flag("done") == "true" {
				payload["done"] = true
			}
			body, err := e.JSONBody(payload)
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/applications/"+url.PathEscape(e.Args[0])+"/tasks", nil, body, "application/json", true, true)
		}},
		{Name: "set", Summary: "change a task", Mutating: true, Usage: "pursuit tasks set <id> [--title ...] [--due ...] [--done|--undone]", Args: []Arg{{Name: "id", Required: true}}, Flags: append(append([]Flag{}, fields...), Flag{Name: "done", Type: "bool", Help: "mark done"}, Flag{Name: "undone", Type: "bool", Help: "mark not done"}, Flag{Name: "version", Type: "string", Help: "object version"}), Run: func(e *exec) error {
			return patchDone(e, "/v1/tasks/"+url.PathEscape(e.Args[0]), put)
		}},
		{Name: "done", Summary: "mark a task done", Mutating: true, Usage: "pursuit tasks done <id>", Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "notes", Type: "string", Help: "details"}, {Name: "version", Type: "string", Help: "object version"}}, Run: func(e *exec) error {
			e.Flags["done"] = []string{"true"}
			return patchDone(e, "/v1/tasks/"+url.PathEscape(e.Args[0]), put)
		}},
		{Name: "delete", Summary: "delete a task", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "object version"}}, Run: func(e *exec) error {
			return e.delete("/v1/tasks/" + url.PathEscape(e.Args[0]))
		}},
	}}
}

// patchDone PATCHes path with the flags put copies, plus done from --done or
// --undone, fetching the version when it wasn't passed.
func patchDone(e *exec, path string, put func(*exec, map[string]any) error) error {
	ver, err := e.ensureVersion(path)
	if err != nil {
		return err
	}
	payload := map[string]any{"version": ver}
	if err := put(e, payload); err != nil {
		return err
	}
	if e.Flag("done") == "true" {
		payload["done"] = true
	}
	if e.Flag("undone") == "true" {
		payload["done"] = false
	}
	body, err := e.JSONBody(payload)
	if err != nil {
		return err
	}
	return e.Send("PATCH", path, nil, body, "application/json", true, true)
}

func cmdTokens() *Command {
	return &Command{Name: "tokens", Summary: "API tokens", Children: []*Command{
		{Name: "list", Summary: "list tokens", Run: func(e *exec) error { return e.Send("GET", "/v1/tokens", nil, nil, "", false, true) }},
		{Name: "create", Summary: "create a token", Mutating: true, Example: "pursuit tokens create --name agent --scope read --scope write", Flags: []Flag{{Name: "name", Type: "string", Help: "token name"}, {Name: "scope", Type: "string", Help: "read, write, or approve; only scopes your own token holds, and approve only from a human token", Repeatable: true}, {Name: "expires-in-days", Type: "string", Help: "stop the token working after this many days (default never)"}}, Run: func(e *exec) error {
			days, _ := strconv.Atoi(e.Flag("expires-in-days"))
			body, err := e.JSONBody(map[string]any{"name": e.Flag("name"), "scopes": e.Flags["scope"], "expires_in_days": days})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/tokens", nil, body, "application/json", true, false)
		}},
		{Name: "revoke", Summary: "revoke a token", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			return e.Send("DELETE", "/v1/tokens/"+url.PathEscape(e.Args[0]), nil, nil, "", true, false)
		}},
	}}
}

func cmdAudit() *Command {
	return &Command{Name: "audit", Summary: "audit log", Children: []*Command{
		{Name: "list", Summary: "list audit entries", Flags: []Flag{{Name: "actor", Type: "string", Help: "actor name"}, {Name: "action", Type: "string", Help: "action"}, {Name: "entity-type", Type: "string", Help: "entity type"}, {Name: "entity-id", Type: "string", Help: "entity id, for example an application id to trace its history"}, {Name: "q", Type: "string", Help: "text in the summary, action, entity, actor, or metadata"}, {Name: "since", Type: "string", Help: "entries at or after this date or RFC 3339 time"}, {Name: "until", Type: "string", Help: "entries at or before this date or RFC 3339 time"}, {Name: "sort", Type: "string", Help: "created_at or -created_at (default newest first)"}, {Name: "limit", Type: "string", Help: "page size"}, {Name: "cursor", Type: "string", Help: "cursor"}}, Run: func(e *exec) error {
			return e.Send("GET", "/v1/audit", listQ(e), nil, "", false, true)
		}},
	}}
}

func cmdImport() *Command {
	file := []Arg{{Name: "file", Required: true, Help: "path or - for stdin"}}
	return &Command{Name: "import", Summary: "import CSV, JSON, markdown, or the example seed", Children: []*Command{
		{Name: "csv", Summary: "import tracker CSV", Mutating: true, Usage: "pursuit import csv <file> [--database-url DSN]", Example: "pursuit import csv applications.csv --database-url \"$DATABASE_URL\"", Args: file, Flags: []Flag{{Name: "database-url", Type: "string", Help: "import into this Postgres or SQLite DSN instead of PURSUIT_SERVER"}}, Run: func(e *exec) error { return e.importFile("csv", "/v1/import/csv", "text/csv") }},
		{Name: "json", Summary: "import a JSON array of postings", Mutating: true, Usage: "pursuit import json <file>", Args: file, Run: func(e *exec) error { return e.importFile("json", "/v1/import/json", "application/json") }},
		{Name: "markdown", Summary: "import markdown tables as to_apply", Mutating: true, Usage: "pursuit import markdown <file>", Args: file, Run: func(e *exec) error { return e.importFile("markdown", "/v1/import/markdown", "text/markdown") }},
		{Name: "seed", Summary: "import the example applications and role batches", Mutating: true, Usage: "pursuit import seed", Run: func(e *exec) error {
			return e.Send("POST", "/v1/import/seed", nil, []byte("{}"), "application/json", true, false)
		}},
	}}
}

func (e *exec) importFile(kind, path, ct string) error {
	var b []byte
	var err error
	if e.Args[0] == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(e.Args[0])
	}
	if err != nil {
		return &exitError{Code: 2, Message: err.Error(), Hint: "pass a readable " + kind + " file"}
	}
	if kind == "csv" {
		if dsn := strings.TrimSpace(e.Flag("database-url")); dsn != "" {
			return e.importCSVDatabase(b, dsn)
		}
	}
	return e.Send("POST", path, nil, b, ct, true, false)
}

func (e *exec) importCSVDatabase(body []byte, dsn string) error {
	ctx := context.Background()
	store, err := sqlstore.Open(ctx, dsn)
	if err != nil {
		return &exitError{Code: 1, Message: redactSecret(err.Error(), dsn), Hint: "pass a postgres:// or sqlite:// URL"}
	}
	defer store.Close()
	svc := service.New(store)
	supplied := strings.TrimSpace(e.cfg.BootstrapToken)
	secret, created, err := svc.Bootstrap(ctx, e.cfg.OwnerName, e.cfg.OwnerEmail, e.cfg.BootstrapName, supplied)
	if err != nil {
		return &exitError{Code: 1, Message: redactSecret(err.Error(), dsn), Hint: "check the database URL"}
	}
	user, err := store.FirstUser(ctx)
	if err != nil {
		return &exitError{Code: 1, Message: err.Error(), Hint: "check the database URL"}
	}
	ctx = domain.WithActor(ctx, domain.Actor{UserID: user.ID, Name: "import", Scopes: []string{"read", "write", "approve"}})
	result, err := svc.ImportCSV(ctx, bytes.NewReader(body))
	if err != nil {
		if de, ok := domain.As(err); ok {
			return &exitError{Code: exitStatus(de.Status), Message: de.Message, Hint: de.Hint}
		}
		return &exitError{Code: 1, Message: err.Error(), Hint: "check the CSV columns"}
	}
	out := struct {
		service.ImportResult
		BootstrapToken string `json:"bootstrap_token,omitempty"`
	}{ImportResult: result}
	if created && supplied == "" {
		out.BootstrapToken = secret
	}
	return e.printObj(out)
}

func redactSecret(msg, secret string) string {
	if secret == "" {
		return msg
	}
	return strings.ReplaceAll(msg, secret, "[redacted]")
}

func cmdExport() *Command {
	flags := []Flag{{Name: "output", Type: "string", Help: "write the file here"}}
	return &Command{Name: "export", Summary: "export applications", Children: []*Command{
		{Name: "simplify", Summary: "Simplify Job Tracker CSV", Usage: "pursuit export simplify [-o file]", Flags: flags, Run: func(e *exec) error { return e.export("/v1/export/simplify", "simplify.csv") }},
		{Name: "json", Summary: "JSON export", Usage: "pursuit export json [-o file]", Flags: flags, Run: func(e *exec) error { return e.export("/v1/export/json", "pursuit.json") }},
	}}
}

func (e *exec) export(path, name string) error {
	resp, err := e.client.Do(nil, "GET", path, nil, nil, "", apiclient.Options{})
	if err != nil {
		return &exitError{Code: 1, Message: err.Error(), Hint: "check that pursuit serve is running and PURSUIT_SERVER is correct"}
	}
	if resp.Status >= 400 {
		return &exitError{Code: exitStatus(resp.Status), Raw: resp.Body}
	}
	out := e.Flag("output")
	if out == "" {
		out = e.Flag("o")
	}
	if out != "" {
		if err := os.WriteFile(out, resp.Body, 0o644); err != nil {
			return &exitError{Code: 1, Message: err.Error(), Hint: "choose a writable --output path"}
		}
		return e.printObj(map[string]any{"path": out, "bytes": len(resp.Body)})
	}
	return e.Print(resp.Body, resp.ContentType)
}

func (e *exec) printObj(v any) error {
	b, err := e.JSONBody(v)
	if err != nil {
		return err
	}
	return e.Print(b, "application/json")
}

func cmdApply() *Command {
	return &Command{
		Name: "apply", Summary: "prepare or enqueue application work",
		Usage: "pursuit apply [<id>] --prepare | --async | --batch key=value",
		Example: "pursuit apply APP --prepare\npursuit apply --batch status=to_apply --async",
		Mutating: true,
		Args: []Arg{{Name: "id", Help: "application id"}},
		Flags: []Flag{
			{Name: "prepare", Type: "bool", Help: "run the one-step prepare now"},
			{Name: "async", Type: "bool", Help: "enqueue the work"},
			{Name: "batch", Type: "string", Help: "filter key=value", Repeatable: true},
			{Name: "worker", Type: "string", Help: "worker id for a synchronous prepare"},
		},
		Run: func(e *exec) error {
			prepare := e.Flag("prepare") == "true"
			async := e.Flag("async") == "true"
			if len(e.Flags["batch"]) > 0 {
				if prepare && async {
					return &exitError{Code: 2, Message: "pass only one of --prepare or --async", Hint: "batch takes one mode"}
				}
				if !prepare && !async {
					async = true
				}
				body := map[string]any{"prepare": prepare, "async": async && !prepare}
				for _, raw := range e.Flags["batch"] {
					k, v, ok := strings.Cut(raw, "=")
					if !ok {
						return &exitError{Code: 2, Message: "batch filter must be key=value", Hint: "for example --batch status=to_apply"}
					}
					switch k {
					case "status":
						body["status"] = v
					case "company":
						body["company"] = v
					case "q":
						body["q"] = v
					case "stale":
						body["stale"] = v == "true" || v == "1"
					default:
						return &exitError{Code: 2, Message: "unknown batch filter " + k, Hint: "filters are status, company, q, stale"}
					}
				}
				b, err := e.JSONBody(body)
				if err != nil {
					return err
				}
				return e.Send("POST", "/v1/apply", nil, b, "application/json", true, false)
			}
			if len(e.Args) != 1 {
				return &exitError{Code: 2, Message: "application id required", Hint: "pursuit apply <id> --prepare or --batch status=to_apply --async"}
			}
			if prepare == async {
				return &exitError{Code: 2, Message: "choose --prepare or --async", Hint: "prepare runs now; async enqueues"}
			}
			id := url.PathEscape(e.Args[0])
			if prepare {
				q := url.Values{}
				if w := e.Flag("worker"); w != "" {
					q.Set("worker", w)
				}
				return e.Send("POST", "/v1/applications/"+id+"/prepare", q, []byte("{}"), "application/json", true, false)
			}
			b, err := e.JSONBody(map[string]string{"kind": "prepare"})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/applications/"+id+"/enqueue", nil, b, "application/json", true, false)
		},
	}
}

func cmdApprove() *Command {
	return &Command{
		Name: "approve", Summary: "record human approval and enqueue submit",
		Usage: "pursuit approve <id> [<id>...]",
		Example: "pursuit approve APP1 APP2",
		Mutating: true,
		Args: []Arg{{Name: "id", Required: true, Many: true}},
		Flags: []Flag{{Name: "note", Type: "string", Help: "note stored with the approval"}},
		Run: func(e *exec) error {
			b, err := e.JSONBody(map[string]any{"ids": e.Args, "note": e.Flag("note")})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/approvals", nil, b, "application/json", true, false)
		},
	}
}

func cmdReject() *Command {
	return &Command{
		Name: "reject", Summary: "decline a prepared application without submitting it",
		Usage: "pursuit reject <id> [<id>...]",
		Example: "pursuit reject APP1",
		Mutating: true,
		Args: []Arg{{Name: "id", Required: true, Many: true}},
		Flags: []Flag{{Name: "note", Type: "string", Help: "note stored with the rejection"}},
		Run: func(e *exec) error {
			b, err := e.JSONBody(map[string]any{"ids": e.Args, "note": e.Flag("note")})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/approvals/reject", nil, b, "application/json", true, false)
		},
	}
}

func cmdQueue() *Command {
	worker := Flag{Name: "worker", Type: "string", Help: "worker id that holds the lease"}
	return &Command{Name: "queue", Summary: "lease application work", Children: []*Command{
		{Name: "claim", Summary: "lease the next job", Mutating: true, Usage: "pursuit queue claim --worker <id>", Example: "pursuit queue claim --worker agent-1", Flags: []Flag{worker}, Run: func(e *exec) error {
			b, err := e.JSONBody(map[string]string{"worker": e.Flag("worker")})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/queue/claim", nil, b, "application/json", true, false)
		}},
		{Name: "heartbeat", Summary: "extend a lease", Mutating: true, Usage: "pursuit queue heartbeat <work-id> --worker <id>", Args: []Arg{{Name: "work-id", Required: true}}, Flags: []Flag{worker}, Run: func(e *exec) error {
			b, err := e.JSONBody(map[string]string{"worker": e.Flag("worker")})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/queue/"+url.PathEscape(e.Args[0])+"/heartbeat", nil, b, "application/json", true, false)
		}},
		{Name: "complete", Summary: "finish leased work", Mutating: true, Usage: "pursuit queue complete <work-id> --worker <id> --status <status>", Args: []Arg{{Name: "work-id", Required: true}}, Flags: []Flag{worker, {Name: "status", Type: "string", Help: "awaiting_review, needs_input, submitted, or failed"}, {Name: "note", Type: "string", Help: "event note"}}, Run: func(e *exec) error {
			b, err := e.JSONBody(map[string]string{"worker": e.Flag("worker"), "status": e.Flag("status"), "note": e.Flag("note")})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/queue/"+url.PathEscape(e.Args[0])+"/complete", nil, b, "application/json", true, false)
		}},
	}}
}

func cmdArtifacts() *Command {
	return &Command{Name: "artifacts", Summary: "files attached to an application", Children: []*Command{
		{Name: "list", Summary: "list artifacts", Args: []Arg{{Name: "application-id", Required: true}}, Run: func(e *exec) error {
			return e.Send("GET", "/v1/applications/"+url.PathEscape(e.Args[0])+"/artifacts", nil, nil, "", false, false)
		}},
		{Name: "add", Summary: "attach a file", Mutating: true, Usage: "pursuit artifacts add <application-id> --kind <kind> --file <path>", Args: []Arg{{Name: "application-id", Required: true}}, Flags: []Flag{{Name: "kind", Type: "string", Help: "resume_pdf, cover_letter, answers_json, form_snapshot, or confirmation"}, {Name: "file", Type: "string", Help: "file path"}}, Run: func(e *exec) error {
			body, ct, err := uploadForm(e.Flag("kind"), "", e.Flag("file"))
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/applications/"+url.PathEscape(e.Args[0])+"/artifacts", nil, body, ct, true, false)
		}},
		{Name: "get", Summary: "fetch one artifact", Args: []Arg{{Name: "artifact-id", Required: true}}, Run: func(e *exec) error {
			return e.Send("GET", "/v1/artifacts/"+url.PathEscape(e.Args[0]), nil, nil, "", false, false)
		}},
	}}
}

func cmdDocuments() *Command {
	return &Command{Name: "documents", Summary: "your files: résumés, cover letters, other", Children: []*Command{
		{Name: "list", Summary: "list documents", Run: func(e *exec) error { return e.Send("GET", "/v1/documents", nil, nil, "", false, false) }},
		{Name: "add", Summary: "upload a document; the first résumé and cover letter become the main ones prepare attaches", Mutating: true, Usage: "pursuit documents add --kind base_resume --file resume.pdf [--label Backend]", Flags: []Flag{{Name: "kind", Type: "string", Help: "base_resume, cover_letter, or other"}, {Name: "file", Type: "string", Help: "file path"}, {Name: "label", Type: "string", Help: "name for this version"}}, Run: func(e *exec) error {
			body, ct, err := uploadForm(e.Flag("kind"), e.Flag("label"), e.Flag("file"))
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/documents", nil, body, ct, true, false)
		}},
		{Name: "get", Summary: "fetch one document; binary bodies are base64", Args: []Arg{{Name: "document-id", Required: true}}, Run: func(e *exec) error {
			return e.Send("GET", "/v1/documents/"+url.PathEscape(e.Args[0]), nil, nil, "", false, false)
		}},
		{Name: "set", Summary: "rename a document, or make it the main résumé or cover letter", Mutating: true, Usage: "pursuit documents set <document-id> [--label ...] [--primary]", Args: []Arg{{Name: "document-id", Required: true}}, Flags: []Flag{{Name: "label", Type: "string", Help: "name for this version"}, {Name: "primary", Type: "bool", Help: "make it the one prepare attaches"}}, Run: func(e *exec) error {
			payload := map[string]any{}
			if v, ok := e.Flags["label"]; ok && len(v) > 0 {
				payload["label"] = v[0]
			}
			if e.Flag("primary") == "true" {
				payload["primary"] = true
			}
			b, err := e.JSONBody(payload)
			if err != nil {
				return err
			}
			return e.Send("PATCH", "/v1/documents/"+url.PathEscape(e.Args[0]), nil, b, "application/json", true, false)
		}},
		{Name: "delete", Summary: "delete a document; deleting the main one makes the newest left the main one", Mutating: true, Args: []Arg{{Name: "document-id", Required: true}}, Run: func(e *exec) error {
			return e.Send("DELETE", "/v1/documents/"+url.PathEscape(e.Args[0]), nil, nil, "", true, false)
		}},
	}}
}

// uploadForm builds a multipart body (kind, label, file). Not JSON: a PDF
// is not valid UTF-8 and would not survive a JSON string.
func uploadForm(kind, label, path string) ([]byte, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", &exitError{Code: 2, Message: err.Error(), Hint: "pass --file"}
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("kind", kind)
	if label != "" {
		_ = mw.WriteField("label", label)
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filepath.Base(path)))
	h.Set("Content-Type", cmp.Or(mime.TypeByExtension(filepath.Ext(path)), "application/octet-stream"))
	part, _ := mw.CreatePart(h)
	_, _ = part.Write(b)
	_ = mw.Close()
	return buf.Bytes(), mw.FormDataContentType(), nil
}

func cmdStanding() *Command {
	return &Command{Name: "standing", Summary: "reusable answers for form questions", Children: []*Command{
		{Name: "list", Summary: "list standing answers", Run: func(e *exec) error { return e.Send("GET", "/v1/standing", nil, nil, "", false, false) }},
		{Name: "set", Summary: "set one standing answer", Mutating: true, Usage: "pursuit standing set --key <key> --value <text>", Flags: []Flag{{Name: "key", Type: "string", Help: "answer key"}, {Name: "value", Type: "string", Help: "answer text"}}, Run: func(e *exec) error {
			b, err := e.JSONBody(map[string]string{"key": e.Flag("key"), "answer": e.Flag("value")})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/standing", nil, b, "application/json", true, false)
		}},
		{Name: "delete", Summary: "delete one standing answer", Mutating: true, Usage: "pursuit standing delete <key>", Args: []Arg{{Name: "key", Required: true}}, Run: func(e *exec) error {
			return e.Send("DELETE", "/v1/standing/"+url.PathEscape(e.Args[0]), nil, nil, "", true, false)
		}},
	}}
}

func cmdCaps() *Command {
	return &Command{Name: "caps", Summary: "per-company application caps", Run: func(e *exec) error {
		return e.Send("GET", "/v1/caps", nil, nil, "", false, false)
	}}
}

func cmdCheckLinks() *Command {
	return &Command{
		Name: "check-links", Summary: "re-check that postings are still live",
		Usage: "pursuit check-links --all | pursuit check-links <application-id>",
		Example: "pursuit check-links --all",
		Mutating: true,
		Args: []Arg{{Name: "application-id"}},
		Flags: []Flag{{Name: "all", Type: "bool", Help: "sweep every to_apply and awaiting_review posting"}},
		Run: func(e *exec) error {
			if e.Flag("all") == "true" {
				b, err := e.JSONBody(map[string]string{"owner": "cli"})
				if err != nil {
					return err
				}
				return e.Send("POST", "/v1/links/check", nil, b, "application/json", true, false)
			}
			if len(e.Args) != 1 {
				return &exitError{Code: 2, Message: "pass --all or an application id", Hint: "pursuit check-links --all"}
			}
			return e.Send("POST", "/v1/applications/"+url.PathEscape(e.Args[0])+"/check-link", nil, []byte("{}"), "application/json", true, false)
		},
	}
}

func cmdLinks() *Command {
	return &Command{
		Name: "links", Summary: "list postings the checker marked closed or changed",
		Usage: "pursuit links [--all | --status <status> | --changed]",
		Example: "pursuit links --fields company,title,status,detail",
		Flags: []Flag{
			{Name: "all", Type: "bool", Help: "every checked posting, not only closed or changed"},
			{Name: "status", Type: "string", Help: "live, closed, or unknown"},
			{Name: "changed", Type: "bool", Help: "only postings whose status changed since the previous check"},
			{Name: "limit", Type: "string", Help: "page size, 1-200, default 50"},
			{Name: "cursor", Type: "string", Help: "next_cursor from the previous page"},
			{Name: "sort", Type: "string", Help: "checked_at, company, or status, with an optional leading -"},
		},
		Run: func(e *exec) error {
			q := url.Values{}
			for _, k := range []string{"status", "limit", "cursor", "sort"} {
				if v := e.Flag(k); v != "" {
					q.Set(k, v)
				}
			}
			if e.Flag("changed") == "true" {
				q.Set("changed", "true")
			}
			if e.Flag("all") != "true" && q.Get("status") == "" && q.Get("changed") == "" {
				q.Set("attention", "true")
			}
			return e.Send("GET", "/v1/links", q, nil, "", false, true)
		},
	}
}

func cmdWebhooks() *Command {
	return &Command{Name: "webhooks", Summary: "event webhooks", Children: []*Command{
		{Name: "list", Summary: "list webhooks", Run: func(e *exec) error { return e.Send("GET", "/v1/webhooks", nil, nil, "", false, false) }},
		{Name: "add", Summary: "register a webhook", Mutating: true, Flags: []Flag{{Name: "url", Type: "string", Help: "https endpoint"}, {Name: "secret", Type: "string", Help: "sent as X-Pursuit-Secret"}}, Run: func(e *exec) error {
			b, err := e.JSONBody(map[string]string{"url": e.Flag("url"), "secret": e.Flag("secret")})
			if err != nil {
				return err
			}
			return e.Send("POST", "/v1/webhooks", nil, b, "application/json", true, false)
		}},
		{Name: "delete", Summary: "delete a webhook", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			return e.Send("DELETE", "/v1/webhooks/"+url.PathEscape(e.Args[0]), nil, nil, "", true, false)
		}},
	}}
}

func cmdBatch() *Command {
	return &Command{
		Name: "batch", Summary: "run up to 100 API requests in one call", Mutating: true,
		Usage: "pursuit batch <file|->",
		Example: `echo '{"requests":[{"method":"PATCH","path":"/v1/applications/APP?version=3","body":{"fit":"strong"}}]}' | pursuit batch -`,
		Args: []Arg{{Name: "file", Required: true, Help: "JSON {requests:[{method,path,body,idempotency_key}],stop_on_error}, path or - for stdin"}},
		Run: func(e *exec) error {
			var b []byte
			var err error
			if e.Args[0] == "-" {
				b, err = io.ReadAll(os.Stdin)
			} else {
				b, err = os.ReadFile(e.Args[0])
			}
			if err != nil {
				return &exitError{Code: 2, Message: err.Error(), Hint: "pass a readable JSON file or -"}
			}
			resp, err := e.client.Do(nil, "POST", "/v1/batch", nil, b, "application/json", apiclient.Options{DryRun: e.G.DryRun, IdempotencyKey: e.G.Idem})
			if err != nil {
				return &exitError{Code: 1, Message: strings.TrimPrefix(err.Error(), "unavailable: "), Hint: "check that pursuit serve is running and PURSUIT_SERVER is correct"}
			}
			if resp.Status >= 400 {
				return &exitError{Code: exitStatus(resp.Status), Raw: resp.Body}
			}
			if err := e.Print(resp.Body, resp.ContentType); err != nil {
				return err
			}
			var out struct {
				Results []struct {
					Status int `json:"status"`
				} `json:"results"`
			}
			_ = json.Unmarshal(resp.Body, &out)
			for _, r := range out.Results {
				if r.Status >= 400 {
					return &exitError{Code: exitStatus(r.Status), Message: "one or more batch items failed", Hint: "check each result status"}
				}
			}
			return nil
		},
	}
}
