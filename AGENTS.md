# Pursuit for agents

Use the `pursuit` CLI. Do not look for an MCP server. Discover one command with `pursuit schema <command>` or `--help --json`. HTTP shapes are `GET /openapi.json` and [llms.txt](llms.txt).

```bash
export PURSUIT_SERVER=http://127.0.0.1:8787
export PURSUIT_TOKEN=pst_...   # write scope to prepare; never put approve on an agent token
```

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
