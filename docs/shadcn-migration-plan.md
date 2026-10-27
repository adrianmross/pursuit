# Dashboard → shadcn migration plan

Branch/worktree: `shadcn-dashboard-migration`. The current dashboard
(`internal/dashboard/static/{index.html,dashboard.css,dashboard.js}`) is a
752-line vanilla-JS SPA served by the Go daemon via `embed.FS`. It stays
running and unmodified until a phase below explicitly replaces it — the Go
API (`internal/httpapi`) does not change in any phase; this is a frontend
rewrite only.

## Dev environment (done)

- `web/` — Vite + React + TypeScript, Tailwind v4, shadcn (`radix` base,
  `nova` preset — Geist font, neutral palette, `.dark` class theming already
  wired).
- `web/portless.json` → `{"name": "pursuit-web"}`. `cd web && portless run npm
  run dev` serves at `https://<branch>.pursuit-web.localhost`.
- `web/vite.config.ts` proxies `/v1` to `PURSUIT_API` (default
  `http://127.0.0.1:8787`) and binds host/port from portless's env vars.
- Run the API alongside it from repo root: `make build && ./bin/pursuit serve
  --addr 127.0.0.1:8787`.

## Why a rewrite, not an in-place edit

shadcn components are React. The existing dashboard has no build step by
design (see the comment at the top of `dashboard.js`). Bringing in shadcn
means accepting a build step; there's no halfway version. `web/` builds to
static assets that get embedded the same way the current static files are
(`//go:embed`), so the daemon's deployment story doesn't change — only
`internal/dashboard/dashboard.go`'s embed paths do, at the end.

## Phases

Each phase is independently shippable and sized for one worktree/session.
Do them in order; later phases assume earlier ones landed on `main`.

### Phase 1 — Shell, auth, routing
- Router (`react-router`, data mode) with routes: `/` (pipeline), `/app/:id`
  (detail), `/queue`. No `/links` route — see Phase 3.
- Port the `api()` fetch wrapper and token-in-localStorage auth as-is (it's
  already minimal and correct — 401 clears token and retries once).
- Sign-in screen: keep the single token field. Drop the scope explainer
  paragraph from the form; scopes are implementation detail, not something a
  human pasting a token needs to read before they can get in. Show the
  identity + scopes only in the account menu (already a `<details>` popover
  in the old UI) for anyone who wants to check what a token can do.
- Top bar shell: brand, nav, account menu, and the global search input (empty
  behavior until Phase 2).

### Phase 2 — Global top-bar search + sort
- Move search out of the pipeline page into the top bar, so it's visible
  from `/queue` and `/app/:id` too. Submitting/typing a query always routes
  to `/` with the query applied — search is global, not page-scoped.
- Fuzzy match: don't call the API per keystroke against a substring filter.
  Fetch the working set client-side (same `/v1/applications` list the
  pipeline table already loads, capped at the existing page size) and rank
  with a small fuzzy matcher — `cmdk`'s built-in command-score, since it's
  already a dependency (Phase 4 needs `cmdk` anyway for fast actions; don't
  add a second fuzzy-match library for this).
- Search matches company, title, status label, *and* tags — not just
  company/title/notes like the current substring search. Tags aren't
  currently surfaced as filter chips in the UI at all (only read in the
  detail page), so this is the one place they become searchable without
  building a whole tag-filter UI.
- Sort stays a dropdown (shadcn `Select`), same options as today
  (`SORTS` in `dashboard.js`) — it's a correct, small control already;
  don't turn it into anything fancier.

### Phase 3 — Fold Links into the main view
- Delete the `/links` route/page. The data it showed (dead/changed postings)
  becomes: a link-status dot already in each pipeline row (port `linkDot`
  as-is) plus a header-level refresh icon button next to "Pipeline" that
  calls `POST /v1/links/check` (today's "Re-check all"), with a spinner state
  while in flight.
- Per-row re-check (today's per-row button on the Links page) becomes an
  icon action in the pipeline row's action menu (see Phase 4) and in the
  detail page, calling `POST /v1/applications/:id/check-link`.
- No backend change — `/v1/links` stays available for anyone scripting
  against the API; just the dedicated page goes away.

### Phase 4 — Queue stays, fast actions, row affordances
- Queue page ports mostly as-is (it's already the "prepared apps" surface
  the user wants kept) — same card layout, approve/reject buttons.
- Add a `cmdk` command palette (⌘K) for fast actions: jump to an application
  by fuzzy company/title match, approve/reject the currently-open
  application, re-check its link, sign out. This is the "fast actions"
  ask — a command palette is the standard shadcn-ecosystem answer and reuses
  the fuzzy matcher from Phase 2.
- Pipeline rows get a per-row overflow menu (shadcn `DropdownMenu`): open
  posting, re-check link, approve/reject when `awaiting_review`. Replaces
  needing to open the detail page for quick decisions.

### Phase 5 — Company icons
- No logo API call, no new dependency: render a deterministic colored
  initial-avatar (first letter of `company`, color hashed from the company
  string) next to the company name in rows and the detail header. This is
  the one-line version — upgrade to real favicons later only if someone
  asks, since that means an external request per row and a cache story.

### Phase 6 — Light/dark + settings
- shadcn's `nova` preset already ships light and dark tokens behind a `.dark`
  class; this phase is wiring, not theming.
- Theme toggle (light/dark/system) in the account menu, persisted to
  `localStorage`, applied by toggling `.dark` on `<html>` — no new state
  library needed for one boolean-ish preference.
- A small settings panel (sheet or dialog, not a route) for the one setting
  that exists today only as a side effect: default sort, default status
  filter. Don't build a settings *page* for two toggles.

### Phase 7 — Cutover
- `vite build` output replaces `internal/dashboard/static/*`; update
  `dashboard.go`'s `//go:embed` + `Open()` switch to match the built
  filenames (likely hashed — switch to embedding the whole `dist/` dir and
  serving by path instead of the current three-case switch).
- Delete the old `dashboard.js`/`dashboard.css`/`index.html` once the React
  build is embedded and `go test ./...` + a manual click-through both pass.
- Add `web/` build to `Makefile`'s `build` target so `make build` produces a
  complete binary without a separate manual step.

## Explicitly out of scope

- Any change to `internal/httpapi` or the API contract.
- A tag-management UI (tags become searchable in Phase 2, not editable).
- Real company logos (see Phase 5).
- Multi-user accounts, SSO — sign-in stays a pasted bearer token.
