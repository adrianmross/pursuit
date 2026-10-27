# Pursuit

Self-hosted job-application tracker for one person, driven from the command line. Agents prepare applications. A human token with the `approve` scope is what allows a submit.

SQLite is the default database for a local daemon. Postgres is the option when you run more than one replica.

Licensed under MIT. Copyright Adrian Ross. See [LICENSE](LICENSE). Commit messages use [Conventional Commits](https://www.conventionalcommits.org/). See [CONTRIBUTING.md](CONTRIBUTING.md).

## Quick start

Requires Go 1.22.

```bash
go build -o bin/pursuit ./cmd/pursuit
./bin/pursuit serve
```

With no database URL the daemon creates `~/.local/share/pursuit/pursuit.db` and listens on `:8787`. The first start logs a bootstrap token once. Save it. It is not written to disk.

```bash
export PURSUIT_TOKEN=pst_...
export PURSUIT_SERVER=http://127.0.0.1:8787
./bin/pursuit import seed
./bin/pursuit list --status to_apply --fields id,company,title,status
```

Applicant answers and the résumé file stay outside the repo:

```bash
export PURSUIT_PROFILE=$HOME/.config/pursuit/profile.json
export PURSUIT_RESUME_FILE=$HOME/.config/pursuit/resume.pdf
export PURSUIT_CONTACT_EMAIL=you@example.com
```

`profile.json` is `{"answers": {"email": "...", "work_authorization": "..."}}`. Keys match form questions after lowercasing and turning other characters into underscores (`why_us`, `cover_letter`).

## Browser extension

`extension/` is a Manifest V3 extension. It saves the posting you have open and fills Greenhouse, Ashby, and Lever application forms from this server. It does not crawl, and it never clicks Submit.

Build it once (Node 22), then load the unpacked `extension/dist` directory:

```bash
cd extension
npm install
npm test
npm run build
```

Chrome: open `chrome://extensions`, turn on Developer mode, choose Load unpacked, and select `extension/dist`. Firefox: open `about:debugging#/runtime/this-firefox`, choose Load Temporary Add-on, and select `extension/dist/manifest.json`.

Pair it from the popup with the server URL (`http://127.0.0.1:8787`) and a token that has `write` and not `approve`. Mint that token with `pursuit tokens create`. The popup asks for permission to call that origin.

The daemon only answers the extension when the origin is allowed:

```bash
export PURSUIT_CORS_ORIGINS='chrome-extension://*,moz-extension://*'
./bin/pursuit serve
```

Use the extension's real id (`chrome-extension://<id>`) instead of the wildcard if you want a tighter list. Save job creates a `to_apply` application and dedupes on the posting URL. Autofill uses `GET /v1/profile` plus that application's prepared answers. A résumé file is not uploaded; the popup downloads the `resume_pdf` artifact when one exists.

What the extension copies from other trackers, and what it skips, is in [docs/inspiration.md](docs/inspiration.md).

## Postgres

```bash
export PURSUIT_DATABASE_URL='postgres://pursuit:pursuit@127.0.0.1:5432/pursuit?sslmode=disable'
./bin/pursuit serve
```

Use `sslmode=require` for Neon. Two replicas can share one Postgres database. Queue claims use `SELECT ... FOR UPDATE SKIP LOCKED`. The link sweep takes a Postgres advisory lock so only one replica checks postings.

`docker compose up --build` starts Postgres and two replicas on ports 8787 and 8788.

## Daemon

`deploy/pursuit.service` is a systemd unit. `deploy/com.pursuit.plist` is a launchd agent. Both run `pursuit serve` with SQLite. `deploy/config.example.json` has no secrets.

Health: `GET /healthz`. Readiness: `GET /readyz` (503 until bootstrap finishes, and during shutdown). Logs are JSON on stderr.

## CLI

Stdout is compact JSON when it is not a terminal, or with `--json`. `--text` forces a table. Errors are always JSON on stderr.

Exit codes: 0 ok, 2 usage, 3 not found, 4 conflict, 5 auth, 6 validation, 1 other.

`pursuit schema` lists commands. `pursuit schema apps list` returns that command only. `GET /openapi.json` is the HTTP document.

Scopes are `read`, `write`, and `approve`. `write` includes `read`. `approve` includes `read` and does not include `write`. New tokens are read-only unless you pass scopes. The bootstrap token has all three.

## Apply pipeline

Statuses move `to_apply` → `preparing` → `awaiting_review` → `approved` → `submitting` → `submitted`. A prepare can stop in `needs_input` or `failed`.

```bash
pursuit standing set --key work_authorization --answer "Yes"
pursuit apply APP_ID --prepare
pursuit apply APP_ID --async
pursuit apply --batch status=to_apply --async
pursuit queue claim --worker agent-1
pursuit queue heartbeat WORK_ID --worker agent-1
pursuit queue complete WORK_ID --worker agent-1 --status awaiting_review
pursuit approve APP_ID
pursuit events --follow
pursuit check-links --all
pursuit list --stale
```

Nothing enters `submitting` or `submitted` without an approval row. `pursuit approve` requires a token with `approve`. A write token can prepare and can finish a submit only after that approval exists.

Company caps ship as Coinbase 3 active per 180 days, OpenAI 5 submitted per 180 days, and Block 9 active per 60 days. Exceeding a cap is a conflict.

The link checker prefers Greenhouse, Ashby, and Lever APIs, then HTTP status and closed-posting phrases. It records `link_status` and does not change the pipeline status. Closed postings show up in `pursuit list --stale`.

## Import and export

`pursuit import csv`, `import json`, and `import markdown` accept a file. Markdown tables and "also live" bullets become `to_apply` and skip a posting URL that already exists. `pursuit import seed` loads the example files in `internal/seed/data` (Example Corp only; the same files are in `testdata/seed`). CSV status `Applied` stays `applied`. `Prepared` becomes `awaiting_review`.

`pursuit export simplify` writes the Simplify Job Tracker columns: Company name, Position Title, Location, Job type, Status, Notes, Posting link.

## License

MIT. See [LICENSE](LICENSE).

## Not in this build

A web dashboard, Workday autofill, clicking Submit, JSON-RPC, and gRPC. The HTTP handlers are a thin adapter over `service.Service`. `api/proto/pursuit/v1/pursuit.proto` is the future contract and is not compiled. There is no MCP server.
