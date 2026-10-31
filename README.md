# Pursuit

Self-hosted job-application tracker for one person, driven from the command line. Agents prepare applications. A human token with the `approve` scope is what allows a submit.

SQLite is the default database for a local daemon. Postgres is the option when you run more than one replica.

Licensed under MIT. Copyright Adrian Ross. See [LICENSE](LICENSE). Commit messages use [Conventional Commits](https://www.conventionalcommits.org/). See [CONTRIBUTING.md](CONTRIBUTING.md).

## Quick start

```bash
brew install adrianmross/tap/pursuit
pursuit serve
```

Linux and Windows binaries, and the browser extension zip, are on [GitHub Releases](https://github.com/adrianmross/pursuit/releases). To build from source you need Go 1.26 and Node 22: `make build` builds the embedded dashboard and docs, then `bin/pursuit`.

With no database URL the daemon creates `~/.local/share/pursuit/pursuit.db` and listens on `:8787`. The first start logs a bootstrap token once. Save it. It is not written to disk.

In `devenv shell`, `pursuit-dev` runs it through [portless](https://github.com/portless-dev/portless) instead, giving it a stable `https://pursuit.localhost` URL (`$PORT` overrides the listen address; `PURSUIT_ADDR` still wins if both are set).

```bash
export PURSUIT_TOKEN=pst_...
export PURSUIT_SERVER=http://127.0.0.1:8787
pursuit import seed
pursuit list --status to_apply --fields id,company,title,status
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
pursuit serve
```

Use the extension's real id (`chrome-extension://<id>`) instead of the wildcard if you want a tighter list. Save job creates a `to_apply` application and dedupes on the posting URL. Autofill uses `GET /v1/profile` plus that application's prepared answers. A résumé file is not uploaded; the popup downloads the `resume_pdf` artifact when one exists.

What the extension copies from other trackers, and what it skips, is in [docs/inspiration.md](docs/inspiration.md).

## Postgres

```bash
export PURSUIT_DATABASE_URL='postgres://pursuit:pursuit@127.0.0.1:5432/pursuit?sslmode=disable'
pursuit serve
```

Use `sslmode=require` for Neon. Two replicas can share one Postgres database. Queue claims use `SELECT ... FOR UPDATE SKIP LOCKED`. The link sweep takes a Postgres advisory lock so only one replica checks postings.

`devenv up --profile postgres` starts Postgres and two replicas on ports 8787 and 8788. `docker compose up --build` runs the same stack in Docker; `docker-compose.yml` is generated from `devenv.nix` (`devenv tasks run dev:compose`).

## Dev environment (devenv)

[devenv](https://devenv.sh) sets up Go 1.26, Node 22, GoReleaser, actionlint and `changelogger`. `direnv allow` loads it on `cd`; otherwise use `devenv shell`. `pursuit serve` still defaults to SQLite.

```bash
devenv shell
pursuit-run                      # builds the web assets and the binary, then serves
devenv tasks run dev:validate    # tests, vet, extension tests, release config checks
```

Add `--profile postgres` to run a local Postgres and two replicas (8787, 8788) against it. Add `postgres` at the end to start only the database:

```bash
devenv up --profile postgres            # Postgres + pursuit-a + pursuit-b
devenv up --profile postgres postgres   # Postgres only, exports PURSUIT_DATABASE_URL
```

`devenv container build serve` builds an OCI image from a Linux binary you build first with `make linux`. On macOS that needs a Linux builder. CI builds it on every push. The `Dockerfile` builds the same image with plain Docker.

Add `--profile neon` to point at an existing Neon database. Put the connection string in a git-ignored `.env.neon` (`PURSUIT_DATABASE_URL=postgres://...neon.tech/...?sslmode=require`), which devenv loads automatically:

```bash
devenv shell --profile neon
```

## Dashboard

Fastest path: `make run` (builds the embedded web assets, then the binary, then serves), then open `http://127.0.0.1:8787/` and paste the bootstrap token the first start printed. No database setup needed; it uses SQLite by default.

`pursuit serve` embeds a small web dashboard and serves it at the same address as the API, `http://127.0.0.1:8787/`. The pages are static files in the binary. They call the REST API and do not have a private backend.

Open that URL and paste a token once. The browser keeps it in `localStorage` and sends it as `Authorization: Bearer`. A read token can browse. Approve and Reject need the `approve` scope. Re-checking a posting needs `write`. The bootstrap token has all three. On localhost the page and the API share an origin, so you do not set `PURSUIT_CORS_ORIGINS` for the dashboard.

The pipeline lists applications with status filters, search, and sorting. An application page shows the posting, notes, history, and prepared artifacts. The approval queue lists `awaiting_review` work. Approve calls `POST /v1/approvals`, which marks the application `approved` and does not submit the form. Reject calls `POST /v1/approvals/reject`, which marks it `withdrawn` and does not submit either. Link health lists postings the checker marked closed and postings whose status changed since the previous check. Re-check calls the existing link endpoints.

The UI is plain HTML, CSS, and JavaScript, with no build step, so those files embed in the one binary and call the JSON API.

## Deploy to Vercel

The same dashboard and API can run as one Go function on Vercel, with Neon Postgres from the Vercel Marketplace. This does not change `pursuit serve`. With the variables below unset, the binary still uses its own database URL, still requires a Pursuit token, and still runs the link checker in the daemon.

Turn on Deployment Protection and choose Vercel Authentication so the deployment is private to you. `PURSUIT_TRUST_PROXY_AUTH=vercel` is safe only in that case. It is off unless that exact value is set. Anyone who can open a deployment without protection would be the owner.

### Environment

| Name | Required | Value |
| --- | --- | --- |
| `DATABASE_URL` | yes, unless `POSTGRES_URL` is set | Neon connection string. Read first. `postgres://` or `postgresql://`. Use `sslmode=require` for Neon. A SQLite URL is rejected. |
| `POSTGRES_URL` | fallback | Used only when `DATABASE_URL` is empty. The Marketplace usually sets both. |
| `PURSUIT_TRUST_PROXY_AUTH` | yes, for the private dashboard | `vercel`. A request with no Pursuit token is the single owner and has `read`, `write`, and `approve`. A request that sends `Authorization: Bearer` is still that token, so the CLI and agents keep using scoped tokens. Any other value is ignored. |
| `PURSUIT_NO_DOCS` | no | `true` stops serving the embedded docs at `/docs` (the same variable works for `pursuit serve`). |
| `CRON_SECRET` | yes, for the daily link check | A long random secret. Vercel Cron sends `GET /v1/cron/links` with `Authorization: Bearer <CRON_SECRET>`. There is no background checker on Vercel. A missing or wrong secret is 401, including when proxy auth is on. |

Optional: `PURSUIT_BOOTSTRAP_TOKEN` chooses the owner token created on the first cold start. `PURSUIT_OWNER_NAME`, `PURSUIT_OWNER_EMAIL`, and `PURSUIT_BOOTSTRAP_NAME` override the owner defaults. If you leave the bootstrap token unset, the generated secret is written once to the function log.

The function opens Postgres on cold start and runs the same migrations the daemon runs. They are safe to run again.

`vercel.json` sends every path to `api/index.go` and schedules the link check daily at 08:00 UTC. That file only calls `pkg/vercelapp`, which opens Postgres, runs migrations, and serves the embedded dashboard. Vercel compiles `api/index.go` outside this module, so it cannot import `internal/`. The dashboard files are already embedded, so the Vercel build does not need Node. `go.mod` selects Go 1.26.

### Seed

From a machine that can reach Neon, load the CSV straight into the database. This does not go through the Vercel URL, so deployment protection does not block it:

```bash
pursuit import csv applications.csv --database-url "$DATABASE_URL"
```

Columns: `id`, `company`, `role`, `location`, `salary`, `fit`, `status`, `applied_date`, `next_step`, `posting_url`, `notes`, `last_updated`.

`Applied` is stored as `applied`. `Prepared` is stored as `awaiting_review`, which is the prepared state (the dashboard label is Prepared). Other statuses match case-insensitively (`Interviewing` becomes `interviewing`). `next_step` and `notes` are stored. The same `posting_url` updates that application instead of inserting another one. A matching `id` still updates that row first.

Without `--database-url`, `pursuit import csv` still posts the file to `PURSUIT_SERVER` and needs a write token. If the database has no token yet, the JSON from `--database-url` includes `bootstrap_token` once. Set `PURSUIT_BOOTSTRAP_TOKEN` first when you want to choose that secret.

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
pursuit reject APP_ID
pursuit events --follow
pursuit check-links --all
pursuit list --stale
```

Nothing enters `submitting` or `submitted` without an approval row. `pursuit approve` requires a token with `approve`. It marks the application `approved` and enqueues submit work; it does not submit a form. `pursuit reject` uses the same scope, marks an `awaiting_review` application `withdrawn`, and does not enqueue submit. A write token can prepare and can finish a submit only after an approval exists.

Company caps ship as Coinbase 3 active per 180 days, OpenAI 5 submitted per 180 days, and Block 9 active per 60 days. Exceeding a cap is a conflict.

The link checker prefers Greenhouse, Ashby, and Lever APIs, then HTTP status and closed-posting phrases. It records `link_status` and does not change the pipeline status. Closed postings show up in `pursuit list --stale`. `pursuit links` lists closed postings and postings whose check status changed since the previous check. `pursuit links --all` lists every checked posting.

## Import and export

`pursuit import csv`, `import json`, and `import markdown` accept a file. Markdown tables and "also live" bullets become `to_apply` and skip a posting URL that already exists. `pursuit import seed` loads the example files in `internal/seed/data` (Example Corp only; the same files are in `testdata/seed`). CSV status `Applied` stays `applied`. `Prepared` becomes `awaiting_review`. Other statuses match case-insensitively. `next_step` and `notes` are kept. Importing the same `posting_url` again updates that application. `pursuit import csv applications.csv --database-url "$DATABASE_URL"` writes the file into that database directly. Omit `--database-url` to post the file to the running server, which is what a local daemon expects.

`pursuit export simplify` writes the Simplify Job Tracker columns: Company name, Position Title, Location, Job type, Status, Notes, Posting link.

## License

MIT. See [LICENSE](LICENSE).

## Not in this build

Workday autofill, clicking Submit, JSON-RPC, and gRPC. The HTTP handlers are a thin adapter over `service.Service`. `api/proto/pursuit/v1/pursuit.proto` is the future contract and is not compiled. There is no MCP server.
