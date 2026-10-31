# Dashboard v3: fine-tuning pass

Feedback batch after the shadcn migration (main, PR #11) shipped. Stacked
worktrees, in dependency order — later phases assume earlier ones landed.
CLI/API first where a phase touches the backend; the dashboard consumes
whatever the CLI/API already expose.

## Phases

1. **`dashboard-fix-bugs`** (this one) — two real defects, not feature work:
   - Command palette (⌘P) renders a blank dialog: `CommandDialog` in
     `web/src/components/ui/command.tsx` never wraps `children` in the cmdk
     `<Command>` root primitive, so `CommandInput`/`CommandList`/`CommandItem`
     have no context to read from.
   - Link checker (`internal/links/check.go`) only has dedicated handling for
     Greenhouse, Ashby, and Lever. Everything else — including Workday and
     LinkedIn, both of which the extension already captures postings from
     (see `extension/fixtures/workday-*`, `linkedin-job.html`) — falls back to
     a fragile generic HTTP-status + phrase-match heuristic. Adds Workday and
     LinkedIn-specific checks using their public APIs, same pattern as the
     existing three.

2. **`dashboard-header-rework`** — remove the "Pipeline" heading entirely
   (no page heading). Search bar grows to fill the freed width when there's
   room. Account menu (the "⋯") trims to exactly: identity, Settings, Sign
   out — theme toggle moves into Settings (phase 11).

3. **`dashboard-row-actions`** — remove the per-row `⋯` dropdown menu. Row
   actions become inline icons: open posting, a single rotate icon *next to*
   the posting link (replaces both the old Link-column header refresh and
   the dropdown's "Re-check link" — one button, one place), archive (see
   phase 5), approve/reject when `awaiting_review`. "Edit" goes away as a
   menu item because edit now happens in place (phase 4) — clicking the row
   navigates to the detail page, which opens editable.

4. **`dashboard-inline-edit`** — create and edit stop being a modal form.
   "Add" and a row click both land on the detail page (`/app/:id`, new
   `/app/new` for create); the whole page is the edit surface. Notes becomes
   a large inline `Textarea`, not a dialog field. Other attributes (location,
   salary, fit, posting URL, status) stay as a compact form section above it.
   Same quick actions (rotate link, archive, approve/reject) appear here too.

5. **`dashboard-archive`** — archive is its own concept, not a status.
   Backend: `archived_at *time.Time` on applications (migration), `POST
   /v1/applications/{id}/archive` + `/unarchive`, `pursuit apps archive` /
   `unarchive` CLI, list query gets an `archived` filter (default excludes).
   Dashboard: an "Archived" view/toggle on the pipeline, unarchive action
   there, archive icon in row actions and the detail page.

6. **`dashboard-status-slider`** — replace the status toggle-buttons-in-a-
   popover from phase 1 of the original migration with an actual slider
   control (continuous track, statuses as positions along it, drag or click
   to jump) — the ChatGPT effort-slider pattern the earlier attempt didn't
   deliver.

7. **`dashboard-custom-columns`** — backend: a small EAV subsystem —
   `custom_fields(id, user_id, name, type)` where type is
   text/number/date/currency-range, `custom_field_values(application_id,
   field_id, value)`. CLI: `pursuit fields create/list/delete`, `pursuit apps
   set-field`. API: `/v1/fields`, value read/write folded into application
   get/update. Dashboard: column management UI, custom columns sortable like
   built-in ones.

8. **`dashboard-column-filters`** — small sort/filter icons on every column
   header (built-in and custom) for a per-column quick filter, alongside the
   existing larger "Filter & sort" popover which stays as the global
   control. Filter *exclusion* (not just inclusion) — e.g. "status is not
   rejected" — for both. Everything scoped under the active search.

9. **`dashboard-search-syntax`** — qualifier syntax in the (now wider) global
   search: `status:applied`, `applied:>2026-01-01`, `tag:remote`, etc.,
   GitHub-search-style. Free text outside qualifiers still fuzzy-matches.

10. **`dashboard-pagination`** — real pagination for the table (the API
    already supports cursor paging), plus a floating down-arrow when
    scrolled to the top and an up-arrow at the bottom, for anyone who'd
    rather scroll a long unpaginated view.

11. **`dashboard-settings-expand`** — Settings grows submenus: Appearance
    (theme, moved here from the account menu), Table (default status/sort,
    already there — relocated), Audit (a read view over `GET /v1/audit`,
    already a real endpoint, so this is dashboard-only).

12. **`dashboard-favicon-logos`** — company logo sources from the company's
    actual favicon instead of an initial avatar. Needs a small backend proxy
    (`GET /v1/companies/{id}/favicon` fetching via the company's known
    domain, or derived from `posting_url`) so the browser isn't making
    cross-origin requests directly and so results can be cached; falls back
    to the initial avatar when no favicon resolves.

13. **`dashboard-homepage`** — clicking "Pursuit" in the top bar (when
    already signed in) goes to a homepage: a short intro to what the tool
    does, a "start a new application" CTA, a link to download the browser
    extension, and import entry points for existing data (Simplify export,
    spreadsheet/CSV) — the CSV/JSON/markdown import endpoints already exist
    (`POST /v1/import/{csv,json,markdown}`), so this phase is dashboard-only
    too: wire a form to the existing API.

## Out of scope for now

Nothing explicitly deferred — this is the full feedback batch. Order above
is the dependency order; phases 7+ are independent of each other and could
be reordered or dropped if priorities change.
