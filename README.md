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

Or sign the CLI in through the dashboard instead of exporting a token. `pursuit auth login` opens the dashboard's `/authorize` page; sign in, check the scopes, and approve. On a machine with no browser (SSH, no display) or with `--device`, it prints a short code; type it at `/device` yourself (links never pre-fill it). The token is stored per server in the OS keyring (macOS Keychain or Linux Secret Service) or, without one, in `~/.config/pursuit/credentials.json` (mode 0600). It gets `read` and `write`, or less with `--scope read`. It never gets `approve`: approving stays in the dashboard, or in an approve token you mint yourself and pass as `PURSUIT_TOKEN`. `PURSUIT_TOKEN` and `--token` still win over a stored login. Any process running as you that calls the CLI without them, an agent included, uses the stored login; that is at most `read,write`, the same as an agent token. A Keychain item written by `/usr/bin/security` can be read back by any process running as your user, so on macOS it protects no better than the 0600 file against a local agent.

```bash
pursuit auth login --server http://127.0.0.1:8787
pursuit auth status
pursuit auth logout
```

Applicant answers and the résumé file stay outside the repo:

```bash
export PURSUIT_PROFILE=$HOME/.config/pursuit/profile.json
export PURSUIT_RESUME_FILE=$HOME/.config/pursuit/resume.pdf
export PURSUIT_CONTACT_EMAIL=you@example.com
```

`profile.json` is `{"answers": {"email": "...", "work_authorization": "..."}}`. Keys match form questions after lowercasing and turning other characters into underscores (`why_us`, `cover_letter`).

On a server with no local disk (Vercel), upload the résumé instead. Prepare attaches the newest `base_resume` document and only falls back to `PURSUIT_RESUME_FILE`. The dashboard does the same on its Profile page.

```bash
pursuit documents add --kind base_resume --file resume.pdf
```

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

Open that URL and paste a token once. The server checks it and keeps it in an httpOnly `pursuit_session` cookie, so page scripts never see it. Writes made with that cookie must come from the dashboard's own origin. A dashboard served from another origin than its API (`NEXT_PUBLIC_PURSUIT_API_URL`) keeps the token in `localStorage` instead and sends it as `Authorization: Bearer`. A read token can browse. Approve and Reject need the `approve` scope. Re-checking a posting needs `write`. The bootstrap token has all three. On localhost the page and the API share an origin, so you do not set `PURSUIT_CORS_ORIGINS` for the dashboard.

The pipeline lists applications with status filters, search, and sorting. An application page shows the posting, notes, history, and prepared artifacts. The approval queue lists `awaiting_review` work. Approve calls `POST /v1/approvals`, which marks the application `approved` and does not submit the form. Reject calls `POST /v1/approvals/reject`, which marks it `withdrawn` and does not submit either. Link health lists postings the checker marked closed and postings whose status changed since the previous check. Re-check calls the existing link endpoints.

The UI is plain HTML, CSS, and JavaScript, with no build step, so those files embed in the one binary and call the JSON API.

## Deploy to Vercel

The same dashboard and API can run as one Go function on Vercel, with Neon Postgres from the Vercel Marketplace. This does not change `pursuit serve`. With the variables below unset, the binary still uses its own database URL, still requires a Pursuit token, and still runs the link checker in the daemon.

With `PURSUIT_TENANT_MODE=multi`, any verified email that signs in gets its own account. Set `PURSUIT_SIGNUPS=closed` to pause that: existing users still sign in, a new email gets `403 sign-ups are paused`, and the dashboard hides "Create an account". Neon Auth's own switch (`neon neon-auth config email-password update --disable-sign-up true`) stops new email/password accounts at the source; the pursuit toggle also covers social and OIDC sign-ins.

Turn on Deployment Protection and choose Vercel Authentication so preview deployments are private to you. `PURSUIT_TRUST_PROXY_AUTH=vercel` is safe only in that case. It is off unless that exact value is set, and it is honored only on preview deployments (`VERCEL_ENV=preview`). Production ignores it and needs a real sign-in: a token, Google or another OIDC provider, or Better Auth.

Production finds its own public URL. With `PURSUIT_SERVER` unset, it uses `VERCEL_PROJECT_PRODUCTION_URL`, which Vercel sets to the shortest custom domain once you add one. Moving to a new domain needs no Pursuit env change. Outside Pursuit, you still update the OIDC provider's redirect URI (`https://<domain>/v1/auth/<provider>/callback`) and any Better Auth trusted origins, run `pursuit auth login` again (stored logins are per server URL), and point the extension at the new URL.

With Better Auth (Neon Auth, `NEXT_PUBLIC_NEON_AUTH_URL` at build time), the dashboard's sign-in card has email and password, a verification code on first sign-in, and "Forgot password?": it emails a 6-digit code, you set a new password, and you are signed in. Codes work with Neon's shared sender; nothing extra to configure. Passkeys are not available: managed Neon Auth does not expose the passkey plugin.

### Environment

| Name | Required | Value |
| --- | --- | --- |
| `DATABASE_URL` | yes, unless `POSTGRES_URL` is set | Neon connection string. Read first. `postgres://` or `postgresql://`. Use `sslmode=require` for Neon. A SQLite URL is rejected. |
| `POSTGRES_URL` | fallback | Used only when `DATABASE_URL` is empty. The Marketplace usually sets both. |
| `PURSUIT_TRUST_PROXY_AUTH` | previews only | `vercel`, set for the Preview environment. A request with no Pursuit token or session is the single owner and has `read`, `write`, and `approve`; its writes must come from the dashboard's own origin. A request that sends `Authorization: Bearer` is still that token, so the CLI and agents keep using scoped tokens. Any other value, and any non-preview deployment, is ignored. |
| `PURSUIT_SERVER` | no | The public URL, when it isn't the one Vercel reports (see above). Used for OIDC redirect URIs. |
| `PURSUIT_NO_DOCS` | no | `true` stops serving the embedded docs at `/docs` (the same variable works for `pursuit serve`). |
| `PURSUIT_CF_BEACON_TOKEN` | no | Cloudflare Web Analytics site token. Read at build time to embed the beacon in the dashboard, and at runtime to allow it in the CSP. Unset means no analytics script. |
| `CRON_SECRET` | yes, for the daily link check | A long random secret. Vercel Cron sends `GET /v1/cron/links` with `Authorization: Bearer <CRON_SECRET>`. There is no background checker on Vercel. A missing or wrong secret is 401, including when proxy auth is on. |

Optional: `PURSUIT_BOOTSTRAP_TOKEN` chooses the owner token created on the first cold start. `PURSUIT_OWNER_NAME`, `PURSUIT_OWNER_EMAIL`, and `PURSUIT_BOOTSTRAP_NAME` override the owner defaults on that first start. Behind `PURSUIT_TRUST_PROXY_AUTH=vercel`, the owner can change their email later under Settings, Account. If you leave the bootstrap token unset, the generated secret is written once to the function log.

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

Agents: [AUTH.md](AUTH.md) (served at `GET /auth.md`) covers tokens, scopes, approval, and auth errors. The dashboard's Connect an agent page (`/agents`) gives a setup prompt with no token in it, the Agent Skill ([skills/pursuit/SKILL.md](skills/pursuit/SKILL.md), served at `GET /skill.md`, installed with `npx skills add adrianmross/pursuit --skill pursuit`), and a `read,write` token for headless agents.

Scopes are `read`, `write`, and `approve`. `write` includes `read`. `approve` includes `read` and does not include `write`. New tokens are read-only unless you pass scopes. The bootstrap token has all three.

`pursuit auth login` (browser, or `--device`), `pursuit auth status`, and `pursuit auth logout` manage a stored per-server token. The OAuth2 endpoints behind it are `POST /v1/auth/device_authorization`, `POST /v1/auth/token`, and `POST /v1/auth/revoke`; the dashboard approves through `/v1/oauth/*` with a session that has `approve` (same-origin JSON only, never the `PURSUIT_TRUST_PROXY_AUTH` owner), and each decision is audited as `oauth.approve` or `oauth.deny`. OAuth grants `read` and `write` only; `approve` is `invalid_scope`.

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
