# Pursuit for agents

Use the `pursuit` CLI. Do not look for an MCP server. Discover one command with `pursuit schema <command>` or `--help --json`. HTTP shapes are `GET /openapi.json` and [llms.txt](llms.txt).

```bash
export PURSUIT_SERVER=http://127.0.0.1:8787
export PURSUIT_TOKEN=pst_...   # write scope to prepare; never put approve on an agent token
```

`pursuit auth login` is for a human at a terminal: it stores a per-server token after someone approves in the dashboard. `PURSUIT_TOKEN` and `--token` always win over it. Do not approve your own login at `/authorize` or `/device`, and do not request `--scope approve`.

Stdout is compact JSON when piped. Errors are JSON on stderr. Exit codes: 0 ok, 2 usage, 3 not found, 4 conflict, 5 auth, 6 validation, 1 other. Pass `--fields` to keep payloads small, `--dry-run` before a mutation, and `--idempotency-key` on writes. Pass `--all-pages` to a list instead of looping cursors, and `pursuit batch <file|->` (`POST /v1/batch`, up to 100 requests) instead of one call per item. Batched approve still needs the `approve` scope.

Prepare and enqueue are allowed. `pursuit approve` and `pursuit reject` are human-only (`approve` scope). Do not transition to `submitting` or `submitted` yourself. After approval, `pursuit queue claim --worker <id>` returns submit work. Heartbeat before the 2 minute lease expires. Complete with `awaiting_review`, `needs_input`, `submitted`, or `failed`. The web dashboard is the human UI for the same API. Do not drive it. A Vercel deployment that sets `PURSUIT_TRUST_PROXY_AUTH=vercel` is for that human behind deployment protection. Agents still send `PURSUIT_TOKEN`.

```bash
pursuit apply APP_ID --async
pursuit queue claim --worker agent-1
pursuit queue complete WORK_ID --worker agent-1 --status awaiting_review
pursuit events --follow --timeout 25s
pursuit list --stale --fields id,company,title,link_status
```

Standing answers (`pursuit standing set`) fill form questions. Unknown required questions become `needs_input`. Company caps return a conflict when exceeded.

`GET /v1/profile` is those standing answers as one map. `POST /v1/applications/capture` saves a posting the user is viewing as `to_apply` and dedupes on the URL. The browser extension calls those. It is not a dashboard.

Debugging with the audit log. Every successful change records who did what and when, with details in `metadata`.

```bash
pursuit audit list --entity-id APP_ID --sort created_at   # one application's history, oldest first
pursuit audit list --q needs_input --since 2026-10-30      # text in summary, action, entity, actor, or metadata
pursuit audit list --actor agent-1 --until 2026-10-30T12:00:00Z
```

A rejected request leaves no entry, so a missing step is the clue: compare the trace with what should have happened, then check the server log by `request_id`. The dashboard's Settings, Audit log page has "Copy for an agent", which copies the same entries as JSON with the command that reproduces them.
