package cli

import (
	"context"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/apiclient"
	"github.com/pursuit/pursuit/internal/app"
)

func rootCommand() *Command {
	return &Command{
		Name: "pursuit", Summary: "job application tracker",
		Usage: "pursuit <command> [flags]",
		Example: "pursuit apps list --status to_apply --fields id,company,title,status",
		Children: []*Command{
			cmdServe(),
			{Name: "version", Summary: "print the CLI version", Run: func(e *exec) error { localVersion(e.stdout, e.json); return nil }},
			{Name: "health", Summary: "check liveness", Run: func(e *exec) error { return e.send("GET", "/healthz", nil, nil, "", false, false) }},
			{Name: "ready", Summary: "check readiness", Run: func(e *exec) error { return e.send("GET", "/readyz", nil, nil, "", false, false) }},
			cmdApps(),
			{Name: "list", Summary: "list applications", Usage: "pursuit list [flags]", Example: "pursuit list --stale --fields id,company,title,link_status", Flags: appListFlags(), Run: runAppList},
			cmdRoles(),
			cmdCompanies(),
			cmdEvents(),
			cmdTags(),
			cmdTokens(),
			cmdAudit(),
			cmdImport(),
			cmdExport(),
			cmdApply(),
			cmdApprove(),
			cmdQueue(),
			cmdArtifacts(),
			cmdStanding(),
			cmdCaps(),
			cmdCheckLinks(),
			cmdWebhooks(),
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
			if v := e.flag("addr"); v != "" {
				cfg.Addr = v
			}
			if v := e.flag("database-url"); v != "" {
				cfg.DatabaseURL = v
			}
			if v := e.flag("log-level"); v != "" {
				cfg.LogLevel = v
			}
			if v := e.flag("log-format"); v != "" {
				cfg.LogFormat = v
			}
			if v := e.flag("owner-name"); v != "" {
				cfg.OwnerName = v
			}
			if v := e.flag("owner-email"); v != "" {
				cfg.OwnerEmail = v
			}
			if v := e.flag("bootstrap-token"); v != "" {
				cfg.BootstrapToken = v
			}
			if v := e.flag("bootstrap-name"); v != "" {
				cfg.BootstrapName = v
			}
			if v := e.flag("cors-origins"); v != "" {
				cfg.CORSOrigins = strings.Split(v, ",")
			}
			if v := e.flag("shutdown-timeout"); v != "" {
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 {
					return &exitError{code: 2, message: "invalid --shutdown-timeout", hint: "pass seconds"}
				}
				cfg.ShutdownTimeout = time.Duration(n) * time.Second
			}
			if v := e.flag("check-interval"); v != "" {
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 {
					return &exitError{code: 2, message: "invalid --check-interval", hint: "pass seconds"}
				}
				cfg.CheckInterval = time.Duration(n) * time.Second
			}
			if v := e.flag("profile"); v != "" {
				cfg.ProfilePath = v
			}
			if v := e.flag("resume-file"); v != "" {
				cfg.ResumeFile = v
			}
			if err := app.Run(context.Background(), cfg); err != nil {
				return &exitError{code: 1, message: err.Error(), hint: "check the database URL and address"}
			}
			return nil
		},
	}
}

func appListFlags() []Flag {
	return []Flag{
		{Name: "status", Type: "string", Help: "filter by status", Repeatable: true},
		{Name: "company", Type: "string", Help: "filter by company name"},
		{Name: "fit", Type: "string", Help: "strong, reach, or maybe"},
		{Name: "source", Type: "string", Help: "greenhouse, ashby, lever, workday, company_site, other"},
		{Name: "tag", Type: "string", Help: "filter by tag"},
		{Name: "q", Type: "string", Help: "search company, title, notes, location, next step"},
		{Name: "stale", Type: "bool", Help: "only postings the link checker marked closed"},
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
				return e.send("GET", "/v1/applications/"+url.PathEscape(e.args[0]), nil, nil, "", false, true)
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
				body, err := e.jsonBody(map[string]any{
					"company": e.flag("company"), "company_id": e.flag("company-id"), "title": e.flag("title"),
					"location": e.flag("location"), "salary": e.flag("salary"), "fit": e.flag("fit"),
					"posting_url": e.flag("posting-url"), "source": e.flag("source"), "status": e.flag("status"),
					"applied_date": e.flag("applied-date"), "next_step": e.flag("next-step"), "notes": e.flag("notes"),
					"contact_email": e.flag("contact-email"),
				})
				if err != nil {
					return err
				}
				return e.send("POST", "/v1/applications", nil, body, "application/json", true, true)
			}},
			{Name: "update", Summary: "update an application", Mutating: true, Usage: "pursuit apps update <id> [flags]", Example: "pursuit apps update APP --status interviewing --next-step 'Schedule call'", Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{
				{Name: "version", Type: "string", Help: "required if you already know it; otherwise the CLI fetches it"},
				{Name: "status", Type: "string", Help: "new status"},
				{Name: "next-step", Type: "string", Help: "next step"},
				{Name: "notes", Type: "string", Help: "notes"},
				{Name: "applied-date", Type: "string", Help: "YYYY-MM-DD, or null to clear"},
				{Name: "title", Type: "string", Help: "role title"},
				{Name: "location", Type: "string", Help: "location"},
				{Name: "salary", Type: "string", Help: "salary text"},
				{Name: "fit", Type: "string", Help: "fit"},
				{Name: "posting-url", Type: "string", Help: "posting URL"},
				{Name: "resume-version", Type: "string", Help: "resume version label"},
				{Name: "contact-email", Type: "string", Help: "contact email"},
			}, Run: runAppUpdate},
			{Name: "delete", Summary: "delete an application", Mutating: true, Usage: "pursuit apps delete <id>", Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "object version"}}, Run: func(e *exec) error {
				return e.delete("/v1/applications/" + url.PathEscape(e.args[0]))
			}},
			{Name: "transition", Summary: "change status and append an event", Mutating: true, Usage: "pursuit apps transition <id> --status <status>", Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{
				{Name: "status", Type: "string", Help: "target status"},
				{Name: "version", Type: "string", Help: "object version"},
				{Name: "next-step", Type: "string", Help: "next step"},
				{Name: "note", Type: "string", Help: "event note"},
			}, Run: func(e *exec) error {
				ver, err := e.ensureVersion("/v1/applications/" + url.PathEscape(e.args[0]))
				if err != nil {
					return err
				}
				payload := map[string]any{"version": ver, "status": e.flag("status"), "note": e.flag("note")}
				if v := e.flag("next-step"); v != "" {
					payload["next_step"] = v
				}
				body, err := e.jsonBody(payload)
				if err != nil {
					return err
				}
				return e.send("POST", "/v1/applications/"+url.PathEscape(e.args[0])+"/transition", nil, body, "application/json", true, true)
			}},
			{Name: "tag", Summary: "application tags", Children: []*Command{
				{Name: "add", Summary: "attach a tag", Mutating: true, Usage: "pursuit apps tag add <id> --name <tag>", Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "name", Type: "string", Help: "tag name"}, {Name: "tag-id", Type: "string", Help: "existing tag id"}}, Run: func(e *exec) error {
					body, err := e.jsonBody(map[string]string{"name": e.flag("name"), "tag_id": e.flag("tag-id")})
					if err != nil {
						return err
					}
					return e.send("POST", "/v1/applications/"+url.PathEscape(e.args[0])+"/tags", nil, body, "application/json", true, true)
				}},
				{Name: "remove", Summary: "detach a tag", Mutating: true, Usage: "pursuit apps tag remove <id> <tag-id>", Args: []Arg{{Name: "id", Required: true}, {Name: "tag-id", Required: true}}, Run: func(e *exec) error {
					return e.send("DELETE", "/v1/applications/"+url.PathEscape(e.args[0])+"/tags/"+url.PathEscape(e.args[1]), nil, nil, "", true, true)
				}},
			}},
		},
	}
}

func runAppList(e *exec) error {
	q := url.Values{}
	for _, st := range e.flags["status"] {
		q.Add("status", st)
	}
	for _, k := range []string{"company", "fit", "source", "tag", "q", "limit", "cursor", "sort"} {
		if v := e.flag(k); v != "" {
			q.Set(strings.ReplaceAll(k, "-", "_"), v)
		}
	}
	if e.flag("stale") == "true" {
		q.Set("stale", "true")
	}
	return e.send("GET", "/v1/applications", q, nil, "", false, true)
}

func runAppUpdate(e *exec) error {
	id := url.PathEscape(e.args[0])
	ver, err := e.ensureVersion("/v1/applications/" + id)
	if err != nil {
		return err
	}
	payload := map[string]any{"version": ver}
	put := func(jsonKey, flag string) {
		if _, ok := e.flags[flag]; ok {
			payload[jsonKey] = e.flag(flag)
		}
	}
	put("status", "status")
	put("next_step", "next-step")
	put("notes", "notes")
	put("title", "title")
	put("location", "location")
	put("salary", "salary")
	put("fit", "fit")
	put("posting_url", "posting-url")
	put("resume_version", "resume-version")
	put("contact_email", "contact-email")
	if _, ok := e.flags["applied-date"]; ok {
		if strings.EqualFold(e.flag("applied-date"), "null") {
			payload["applied_date"] = nil
		} else {
			payload["applied_date"] = e.flag("applied-date")
		}
	}
	body, err := e.jsonBody(payload)
	if err != nil {
		return err
	}
	return e.send("PATCH", "/v1/applications/"+id, nil, body, "application/json", true, true)
}

func (e *exec) ensureVersion(path string) (int, error) {
	if v := e.flag("version"); v != "" {
		return atoiFlag(v)
	}
	return e.versionOf(path)
}

func (e *exec) delete(path string) error {
	ver, err := e.ensureVersion(path)
	if err != nil {
		return err
	}
	q := url.Values{"version": {strconv.Itoa(ver)}}
	return e.send("DELETE", path, q, nil, "", true, false)
}

func cmdCompanies() *Command {
	return &Command{Name: "companies", Summary: "companies", Children: []*Command{
		{Name: "list", Summary: "list companies", Flags: []Flag{{Name: "q", Type: "string", Help: "search"}, {Name: "limit", Type: "string", Help: "page size"}, {Name: "cursor", Type: "string", Help: "cursor"}, {Name: "sort", Type: "string", Help: "sort"}}, Run: func(e *exec) error {
			return e.send("GET", "/v1/companies", listQ(e), nil, "", false, true)
		}},
		{Name: "get", Summary: "get a company", Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			return e.send("GET", "/v1/companies/"+url.PathEscape(e.args[0]), nil, nil, "", false, true)
		}},
		{Name: "create", Summary: "create a company", Mutating: true, Flags: []Flag{{Name: "name", Type: "string", Help: "name"}, {Name: "website", Type: "string", Help: "website"}, {Name: "notes", Type: "string", Help: "notes"}}, Run: func(e *exec) error {
			body, err := e.jsonBody(map[string]string{"name": e.flag("name"), "website": e.flag("website"), "notes": e.flag("notes")})
			if err != nil {
				return err
			}
			return e.send("POST", "/v1/companies", nil, body, "application/json", true, true)
		}},
		{Name: "update", Summary: "update a company", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "version"}, {Name: "name", Type: "string", Help: "name"}, {Name: "website", Type: "string", Help: "website"}, {Name: "notes", Type: "string", Help: "notes"}}, Run: func(e *exec) error {
			ver, err := e.ensureVersion("/v1/companies/" + url.PathEscape(e.args[0]))
			if err != nil {
				return err
			}
			payload := map[string]any{"version": ver}
			if _, ok := e.flags["name"]; ok {
				payload["name"] = e.flag("name")
			}
			if _, ok := e.flags["website"]; ok {
				payload["website"] = e.flag("website")
			}
			if _, ok := e.flags["notes"]; ok {
				payload["notes"] = e.flag("notes")
			}
			body, err := e.jsonBody(payload)
			if err != nil {
				return err
			}
			return e.send("PATCH", "/v1/companies/"+url.PathEscape(e.args[0]), nil, body, "application/json", true, true)
		}},
		{Name: "delete", Summary: "delete a company", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "version"}}, Run: func(e *exec) error {
			return e.delete("/v1/companies/" + url.PathEscape(e.args[0]))
		}},
	}}
}

func cmdRoles() *Command {
	return &Command{Name: "roles", Summary: "roles", Children: []*Command{
		{Name: "list", Summary: "list roles", Flags: []Flag{{Name: "q", Type: "string", Help: "search"}, {Name: "company", Type: "string", Help: "company name"}, {Name: "fit", Type: "string", Help: "fit"}, {Name: "source", Type: "string", Help: "source"}, {Name: "limit", Type: "string", Help: "page size"}, {Name: "cursor", Type: "string", Help: "cursor"}, {Name: "sort", Type: "string", Help: "sort"}}, Run: func(e *exec) error {
			return e.send("GET", "/v1/roles", listQ(e), nil, "", false, true)
		}},
		{Name: "get", Summary: "get a role", Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			return e.send("GET", "/v1/roles/"+url.PathEscape(e.args[0]), nil, nil, "", false, true)
		}},
		{Name: "create", Summary: "create a role", Mutating: true, Flags: []Flag{
			{Name: "company", Type: "string", Help: "company name"}, {Name: "company-id", Type: "string", Help: "company id"},
			{Name: "title", Type: "string", Help: "title"}, {Name: "location", Type: "string", Help: "location"},
			{Name: "salary", Type: "string", Help: "salary"}, {Name: "fit", Type: "string", Help: "fit"},
			{Name: "posting-url", Type: "string", Help: "posting URL"}, {Name: "source", Type: "string", Help: "source"},
		}, Run: func(e *exec) error {
			body, err := e.jsonBody(map[string]string{"company": e.flag("company"), "company_id": e.flag("company-id"), "title": e.flag("title"), "location": e.flag("location"), "salary": e.flag("salary"), "fit": e.flag("fit"), "posting_url": e.flag("posting-url"), "source": e.flag("source")})
			if err != nil {
				return err
			}
			return e.send("POST", "/v1/roles", nil, body, "application/json", true, true)
		}},
		{Name: "update", Summary: "update a role", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "version"}, {Name: "title", Type: "string", Help: "title"}, {Name: "location", Type: "string", Help: "location"}, {Name: "salary", Type: "string", Help: "salary"}, {Name: "fit", Type: "string", Help: "fit"}, {Name: "posting-url", Type: "string", Help: "posting URL"}}, Run: func(e *exec) error {
			ver, err := e.ensureVersion("/v1/roles/" + url.PathEscape(e.args[0]))
			if err != nil {
				return err
			}
			payload := map[string]any{"version": ver}
			for _, pair := range [][2]string{{"title", "title"}, {"location", "location"}, {"salary", "salary"}, {"fit", "fit"}, {"posting_url", "posting-url"}} {
				if _, ok := e.flags[pair[1]]; ok {
					payload[pair[0]] = e.flag(pair[1])
				}
			}
			body, err := e.jsonBody(payload)
			if err != nil {
				return err
			}
			return e.send("PATCH", "/v1/roles/"+url.PathEscape(e.args[0]), nil, body, "application/json", true, true)
		}},
		{Name: "delete", Summary: "delete a role", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Flags: []Flag{{Name: "version", Type: "string", Help: "version"}}, Run: func(e *exec) error {
			return e.delete("/v1/roles/" + url.PathEscape(e.args[0]))
		}},
	}}
}

func listQ(e *exec) url.Values {
	q := url.Values{}
	for _, k := range []string{"q", "company", "fit", "source", "limit", "cursor", "sort", "actor", "action"} {
		if v := e.flag(k); v != "" {
			q.Set(k, v)
		}
	}
	if v := e.flag("entity-type"); v != "" {
		q.Set("entity_type", v)
	}
	if v := e.flag("entity-id"); v != "" {
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
			body, err := e.jsonBody(map[string]string{"kind": e.flag("kind"), "title": e.flag("title"), "body": e.flag("body")})
			if err != nil {
				return err
			}
			return e.send("POST", "/v1/applications/"+url.PathEscape(e.args[0])+"/events", nil, body, "application/json", true, true)
		}},
	}}
}

func runEvents(e *exec) error {
	q := url.Values{}
	if v := e.flag("application"); v != "" {
		q.Set("application_id", v)
	}
	if v := e.flag("after"); v != "" {
		q.Set("after", v)
	}
	if v := e.flag("limit"); v != "" {
		q.Set("limit", v)
	}
	if e.flag("follow") == "true" {
		wait := 25000
		if v := e.flag("timeout"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				n, nerr := strconv.Atoi(v)
				if nerr != nil {
					return &exitError{code: 2, message: "invalid --timeout", hint: "use a duration such as 25s"}
				}
				d = time.Duration(n) * time.Second
			}
			wait = int(d / time.Millisecond)
		}
		q.Set("wait_ms", strconv.Itoa(wait))
	}
	return e.send("GET", "/v1/events", q, nil, "", false, true)
}

func cmdTags() *Command {
	return &Command{Name: "tags", Summary: "tags", Children: []*Command{
		{Name: "list", Summary: "list tags", Run: func(e *exec) error { return e.send("GET", "/v1/tags", nil, nil, "", false, true) }},
		{Name: "create", Summary: "create a tag", Mutating: true, Flags: []Flag{{Name: "name", Type: "string", Help: "tag name"}}, Run: func(e *exec) error {
			body, err := e.jsonBody(map[string]string{"name": e.flag("name")})
			if err != nil {
				return err
			}
			return e.send("POST", "/v1/tags", nil, body, "application/json", true, true)
		}},
		{Name: "delete", Summary: "delete a tag", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			return e.send("DELETE", "/v1/tags/"+url.PathEscape(e.args[0]), nil, nil, "", true, false)
		}},
	}}
}

func cmdTokens() *Command {
	return &Command{Name: "tokens", Summary: "API tokens", Children: []*Command{
		{Name: "list", Summary: "list tokens", Run: func(e *exec) error { return e.send("GET", "/v1/tokens", nil, nil, "", false, true) }},
		{Name: "create", Summary: "create a token", Mutating: true, Example: "pursuit tokens create --name agent --scope read --scope write", Flags: []Flag{{Name: "name", Type: "string", Help: "token name"}, {Name: "scope", Type: "string", Help: "read, write, or approve", Repeatable: true}}, Run: func(e *exec) error {
			body, err := e.jsonBody(map[string]any{"name": e.flag("name"), "scopes": e.flags["scope"]})
			if err != nil {
				return err
			}
			return e.send("POST", "/v1/tokens", nil, body, "application/json", true, false)
		}},
		{Name: "revoke", Summary: "revoke a token", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			return e.send("DELETE", "/v1/tokens/"+url.PathEscape(e.args[0]), nil, nil, "", true, false)
		}},
	}}
}

func cmdAudit() *Command {
	return &Command{Name: "audit", Summary: "audit log", Children: []*Command{
		{Name: "list", Summary: "list audit entries", Flags: []Flag{{Name: "actor", Type: "string", Help: "actor name"}, {Name: "action", Type: "string", Help: "action"}, {Name: "entity-type", Type: "string", Help: "entity type"}, {Name: "entity-id", Type: "string", Help: "entity id"}, {Name: "limit", Type: "string", Help: "page size"}, {Name: "cursor", Type: "string", Help: "cursor"}}, Run: func(e *exec) error {
			return e.send("GET", "/v1/audit", listQ(e), nil, "", false, true)
		}},
	}}
}

func cmdImport() *Command {
	file := []Arg{{Name: "file", Required: true, Help: "path or - for stdin"}}
	return &Command{Name: "import", Summary: "import CSV, JSON, markdown, or the example seed", Children: []*Command{
		{Name: "csv", Summary: "import tracker CSV", Mutating: true, Usage: "pursuit import csv <file>", Args: file, Run: func(e *exec) error { return e.importFile("csv", "/v1/import/csv", "text/csv") }},
		{Name: "json", Summary: "import a JSON array of postings", Mutating: true, Usage: "pursuit import json <file>", Args: file, Run: func(e *exec) error { return e.importFile("json", "/v1/import/json", "application/json") }},
		{Name: "markdown", Summary: "import markdown tables as to_apply", Mutating: true, Usage: "pursuit import markdown <file>", Args: file, Run: func(e *exec) error { return e.importFile("markdown", "/v1/import/markdown", "text/markdown") }},
		{Name: "seed", Summary: "import the example applications and role batches", Mutating: true, Usage: "pursuit import seed", Run: func(e *exec) error {
			return e.send("POST", "/v1/import/seed", nil, []byte("{}"), "application/json", true, false)
		}},
	}}
}

func (e *exec) importFile(kind, path, ct string) error {
	var b []byte
	var err error
	if e.args[0] == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(e.args[0])
	}
	if err != nil {
		return &exitError{code: 2, message: err.Error(), hint: "pass a readable " + kind + " file"}
	}
	return e.send("POST", path, nil, b, ct, true, false)
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
		return &exitError{code: 1, message: err.Error(), hint: "check that pursuit serve is running and PURSUIT_SERVER is correct"}
	}
	if resp.Status >= 400 {
		return &exitError{code: exitStatus(resp.Status), raw: resp.Body}
	}
	out := e.flag("output")
	if out == "" {
		out = e.flag("o")
	}
	if out != "" {
		if err := os.WriteFile(out, resp.Body, 0o644); err != nil {
			return &exitError{code: 1, message: err.Error(), hint: "choose a writable --output path"}
		}
		return e.printObj(map[string]any{"path": out, "bytes": len(resp.Body)})
	}
	return e.print(resp.Body, resp.ContentType)
}

func (e *exec) printObj(v any) error {
	b, err := e.jsonBody(v)
	if err != nil {
		return err
	}
	return e.print(b, "application/json")
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
			prepare := e.flag("prepare") == "true"
			async := e.flag("async") == "true"
			if len(e.flags["batch"]) > 0 {
				if prepare && async {
					return &exitError{code: 2, message: "pass only one of --prepare or --async", hint: "batch takes one mode"}
				}
				if !prepare && !async {
					async = true
				}
				body := map[string]any{"prepare": prepare, "async": async && !prepare}
				for _, raw := range e.flags["batch"] {
					k, v, ok := strings.Cut(raw, "=")
					if !ok {
						return &exitError{code: 2, message: "batch filter must be key=value", hint: "for example --batch status=to_apply"}
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
						return &exitError{code: 2, message: "unknown batch filter " + k, hint: "filters are status, company, q, stale"}
					}
				}
				b, err := e.jsonBody(body)
				if err != nil {
					return err
				}
				return e.send("POST", "/v1/apply", nil, b, "application/json", true, false)
			}
			if len(e.args) != 1 {
				return &exitError{code: 2, message: "application id required", hint: "pursuit apply <id> --prepare or --batch status=to_apply --async"}
			}
			if prepare == async {
				return &exitError{code: 2, message: "choose --prepare or --async", hint: "prepare runs now; async enqueues"}
			}
			id := url.PathEscape(e.args[0])
			if prepare {
				q := url.Values{}
				if w := e.flag("worker"); w != "" {
					q.Set("worker", w)
				}
				return e.send("POST", "/v1/applications/"+id+"/prepare", q, []byte("{}"), "application/json", true, false)
			}
			b, err := e.jsonBody(map[string]string{"kind": "prepare"})
			if err != nil {
				return err
			}
			return e.send("POST", "/v1/applications/"+id+"/enqueue", nil, b, "application/json", true, false)
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
			b, err := e.jsonBody(map[string]any{"ids": e.args, "note": e.flag("note")})
			if err != nil {
				return err
			}
			return e.send("POST", "/v1/approvals", nil, b, "application/json", true, false)
		},
	}
}

func cmdQueue() *Command {
	worker := Flag{Name: "worker", Type: "string", Help: "worker id that holds the lease"}
	return &Command{Name: "queue", Summary: "lease application work", Children: []*Command{
		{Name: "claim", Summary: "lease the next job", Mutating: true, Usage: "pursuit queue claim --worker <id>", Example: "pursuit queue claim --worker agent-1", Flags: []Flag{worker}, Run: func(e *exec) error {
			b, err := e.jsonBody(map[string]string{"worker": e.flag("worker")})
			if err != nil {
				return err
			}
			return e.send("POST", "/v1/queue/claim", nil, b, "application/json", true, false)
		}},
		{Name: "heartbeat", Summary: "extend a lease", Mutating: true, Usage: "pursuit queue heartbeat <work-id> --worker <id>", Args: []Arg{{Name: "work-id", Required: true}}, Flags: []Flag{worker}, Run: func(e *exec) error {
			b, err := e.jsonBody(map[string]string{"worker": e.flag("worker")})
			if err != nil {
				return err
			}
			return e.send("POST", "/v1/queue/"+url.PathEscape(e.args[0])+"/heartbeat", nil, b, "application/json", true, false)
		}},
		{Name: "complete", Summary: "finish leased work", Mutating: true, Usage: "pursuit queue complete <work-id> --worker <id> --status <status>", Args: []Arg{{Name: "work-id", Required: true}}, Flags: []Flag{worker, {Name: "status", Type: "string", Help: "awaiting_review, needs_input, submitted, or failed"}, {Name: "note", Type: "string", Help: "event note"}}, Run: func(e *exec) error {
			b, err := e.jsonBody(map[string]string{"worker": e.flag("worker"), "status": e.flag("status"), "note": e.flag("note")})
			if err != nil {
				return err
			}
			return e.send("POST", "/v1/queue/"+url.PathEscape(e.args[0])+"/complete", nil, b, "application/json", true, false)
		}},
	}}
}

func cmdArtifacts() *Command {
	return &Command{Name: "artifacts", Summary: "files attached to an application", Children: []*Command{
		{Name: "list", Summary: "list artifacts", Args: []Arg{{Name: "application-id", Required: true}}, Run: func(e *exec) error {
			return e.send("GET", "/v1/applications/"+url.PathEscape(e.args[0])+"/artifacts", nil, nil, "", false, false)
		}},
		{Name: "add", Summary: "attach a file", Mutating: true, Usage: "pursuit artifacts add <application-id> --kind <kind> --file <path>", Args: []Arg{{Name: "application-id", Required: true}}, Flags: []Flag{{Name: "kind", Type: "string", Help: "resume_pdf, cover_letter, answers_json, form_snapshot, or confirmation"}, {Name: "file", Type: "string", Help: "file path"}}, Run: func(e *exec) error {
			b, err := os.ReadFile(e.flag("file"))
			if err != nil {
				return &exitError{code: 2, message: err.Error(), hint: "pass --file"}
			}
			body, err := e.jsonBody(map[string]string{"kind": e.flag("kind"), "filename": e.flag("file"), "content_type": contentType(e.flag("kind")), "body": string(b)})
			if err != nil {
				return err
			}
			return e.send("POST", "/v1/applications/"+url.PathEscape(e.args[0])+"/artifacts", nil, body, "application/json", true, false)
		}},
		{Name: "get", Summary: "fetch one artifact", Args: []Arg{{Name: "artifact-id", Required: true}}, Run: func(e *exec) error {
			return e.send("GET", "/v1/artifacts/"+url.PathEscape(e.args[0]), nil, nil, "", false, false)
		}},
	}}
}

func contentType(kind string) string {
	switch kind {
	case "answers_json", "form_snapshot":
		return "application/json"
	case "cover_letter":
		return "text/plain"
	case "resume_pdf", "confirmation":
		return "application/octet-stream"
	default:
		return "text/plain"
	}
}

func cmdStanding() *Command {
	return &Command{Name: "standing", Summary: "reusable answers for form questions", Children: []*Command{
		{Name: "list", Summary: "list standing answers", Run: func(e *exec) error { return e.send("GET", "/v1/standing", nil, nil, "", false, false) }},
		{Name: "set", Summary: "set one standing answer", Mutating: true, Usage: "pursuit standing set --key <key> --value <text>", Flags: []Flag{{Name: "key", Type: "string", Help: "answer key"}, {Name: "value", Type: "string", Help: "answer text"}}, Run: func(e *exec) error {
			b, err := e.jsonBody(map[string]string{"key": e.flag("key"), "answer": e.flag("value")})
			if err != nil {
				return err
			}
			return e.send("POST", "/v1/standing", nil, b, "application/json", true, false)
		}},
	}}
}

func cmdCaps() *Command {
	return &Command{Name: "caps", Summary: "per-company application caps", Run: func(e *exec) error {
		return e.send("GET", "/v1/caps", nil, nil, "", false, false)
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
			if e.flag("all") == "true" {
				b, err := e.jsonBody(map[string]string{"owner": "cli"})
				if err != nil {
					return err
				}
				return e.send("POST", "/v1/links/check", nil, b, "application/json", true, false)
			}
			if len(e.args) != 1 {
				return &exitError{code: 2, message: "pass --all or an application id", hint: "pursuit check-links --all"}
			}
			return e.send("POST", "/v1/applications/"+url.PathEscape(e.args[0])+"/check-link", nil, []byte("{}"), "application/json", true, false)
		},
	}
}

func cmdWebhooks() *Command {
	return &Command{Name: "webhooks", Summary: "event webhooks", Children: []*Command{
		{Name: "list", Summary: "list webhooks", Run: func(e *exec) error { return e.send("GET", "/v1/webhooks", nil, nil, "", false, false) }},
		{Name: "add", Summary: "register a webhook", Mutating: true, Flags: []Flag{{Name: "url", Type: "string", Help: "https endpoint"}, {Name: "secret", Type: "string", Help: "sent as X-Pursuit-Secret"}}, Run: func(e *exec) error {
			b, err := e.jsonBody(map[string]string{"url": e.flag("url"), "secret": e.flag("secret")})
			if err != nil {
				return err
			}
			return e.send("POST", "/v1/webhooks", nil, b, "application/json", true, false)
		}},
		{Name: "delete", Summary: "delete a webhook", Mutating: true, Args: []Arg{{Name: "id", Required: true}}, Run: func(e *exec) error {
			return e.send("DELETE", "/v1/webhooks/"+url.PathEscape(e.args[0]), nil, nil, "", true, false)
		}},
	}}
}
