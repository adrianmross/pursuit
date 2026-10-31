"use client"

import { Fragment, useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import Link from "next/link"
import { usePathname, useRouter, useSearchParams } from "next/navigation"
import commandScore from "command-score"
import { parseSearch, matchesQualifiers } from "@/lib/search"
import {
  Archive,
  ArchiveRestore,
  ArrowDown,
  ArrowUp,
  Check,
  ChevronDown,
  ChevronRight,
  ExternalLink,
  ListChecks,
  MoreHorizontal,
  Plus,
  RefreshCw,
  RotateCcw,
  SlidersHorizontal,
  Trash2,
  X,
} from "lucide-react"
import { api, ApiError } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { useShortcuts } from "@/lib/keybindings"
import { Kbd } from "@/components/ui/kbd"
import { getDefaultSort, getDefaultStatus, getPref } from "@/lib/settings"
import { buildApplicationsQuery } from "@/lib/pipeline-query"
import { applicationHref } from "@/lib/links"
import {
  CATEGORY_LABELS,
  CATEGORY_ORDER,
  label,
  OUTCOMES,
  SORTS,
  STAGE_STOPS,
  sourceLabel,
  statusCategory,
  markOf,
  shown,
  type Application,
  type CustomField,
  type Page,
  type StatusCategory,
} from "@/lib/types"
import { CompanyAvatar } from "@/components/CompanyAvatar"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { ProgressMark } from "@/components/ProgressMark"
import { StatusBadge } from "@/components/StatusBadge"
import { OutcomeChip, SectionLabel, StageHeading, StageSlider, stageTarget, StatusChooser, SubSteps } from "@/components/StatusChooser"
import { STATUS_OPTIONS } from "@/components/AttributesForm"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

// The filter's stops: "All" first, then every piece of the mark.
const FILTER_STOPS: [string, string][] = [["", "All"], ...STAGE_STOPS]
// Category names short enough for one segmented row.
const CATEGORY_SHORT: Record<StatusCategory, string> = { action: "You", waiting: "Them", closed: "Closed", rejected: "Rejected" }

// First pages by query, so flipping between filters shows rows at once while
// they refetch. ponytail: unbounded and per-tab; it holds one page per filter
// combination visited, so cap it if that ever grows large.
const pageCache = new Map<string, { rows: Application[]; total: number; nextCursor: string }>()

export type PipelineInitial = {
  rows: Application[]
  total: number
  nextCursor: string
  fields: CustomField[]
}

function safeURL(raw?: string) {
  if (!raw) return ""
  try {
    const url = new URL(raw)
    return url.protocol === "http:" || url.protocol === "https:" ? url.href : ""
  } catch {
    return ""
  }
}

function shortWhen(value?: string) {
  if (!value) return "—"
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (!match) return value
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]))
  if (Number.isNaN(date.getTime())) return value
  const now = new Date()
  const diff = Math.round(
    (new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime() -
      new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()) /
      86400000,
  )
  if (diff === 0) return "today"
  if (diff > 0 && diff < 7) return diff + "d ago"
  const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"]
  const text = months[date.getMonth()] + " " + date.getDate()
  return date.getFullYear() === now.getFullYear() ? text : text + ", " + date.getFullYear()
}

function LinkBadge({ status }: { status?: string }) {
  const dead = status === "closed"
  const live = status === "live"
  const text = dead ? "Dead link" : live ? "Posting is live" : status === "unknown" ? "Link status unknown" : "Link not checked"
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          role="img"
          aria-label={text}
          className={`inline-block size-2.5 rounded-full ${dead ? "bg-destructive" : "bg-muted-foreground/40"}`}
        />
      </TooltipTrigger>
      <TooltipContent>{text}</TooltipContent>
    </Tooltip>
  )
}

// The pipeline's filter: the same minimal stage slider as the status chooser,
// expanding to outcomes (each a die, tap to hide), who moves next, the steps
// between stages, and sort.
function FilterSortBar({
  status,
  category,
  sort,
  exclude,
  onPatch,
}: {
  status: string
  category: string
  sort: string
  exclude: string[]
  onPatch: (next: Record<string, string>) => void
}) {
  const [expanded, setExpanded] = useState(false)
  const [drag, setDrag] = useState<number | null>(null)
  const index = FILTER_STOPS.findIndex(([value]) => value === status)
  const at = drag ?? Math.max(0, index)
  const current = drag != null ? FILTER_STOPS[drag][0] : status
  const hidden = OUTCOMES.filter((s) => exclude.includes(s)).length
  const active = status !== "" || category !== "" || exclude.length > 0
  const title =
    drag == null && category ? CATEGORY_LABELS[category as StatusCategory] : current ? label(current) : "All stages"

  function toggleOutcome(value: string) {
    const next = exclude.includes(value) ? exclude.filter((v) => v !== value) : [...exclude, value]
    onPatch({ exclude: next.join(",") })
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="relative gap-1.5">
          <SlidersHorizontal className="size-3.5" /> Filter &amp; sort
          {active && <span className="absolute -right-1 -top-1 size-2 rounded-full bg-primary" />}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[22rem] rounded-2xl p-3">
        <div className="flex flex-col gap-3">
          <div className="flex items-start justify-between">
            <Button
              variant="ghost"
              size="icon"
              className={`size-7 ${expanded ? "bg-muted text-foreground" : "text-muted-foreground"}`}
              aria-label={expanded ? "Fewer filters" : "More filters"}
              aria-expanded={expanded}
              onClick={() => setExpanded(!expanded)}
            >
              <SlidersHorizontal className="size-4" />
            </Button>
            <StageHeading
              stage={at === 0 ? 6 : at - 1}
              title={title}
              gold={current === "offer"}
              sub={
                <button
                  type="button"
                  aria-expanded={expanded}
                  onClick={() => setExpanded(!expanded)}
                  className="flex items-center gap-0.5 rounded text-[13px] leading-[18px] text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
                >
                  {hidden ? `${hidden} outcome${hidden === 1 ? "" : "s"} hidden` : "Every outcome shown"}
                  <ChevronRight className={`size-3 transition-transform ${expanded ? "rotate-90" : ""}`} />
                </button>
              }
            />
            <Button
              variant="ghost"
              size="icon"
              className="size-7 text-muted-foreground"
              aria-label="Reset filters"
              disabled={!active}
              onClick={() => onPatch({ status: "", category: "", exclude: "" })}
            >
              <RotateCcw className="size-4" />
            </Button>
          </div>
          <StageSlider
            stops={FILTER_STOPS}
            value={Math.max(0, index)}
            label="Stage filter"
            onDrag={setDrag}
            onCommit={(i) => onPatch({ status: FILTER_STOPS[i][0], category: "" })}
          />
          {expanded && (
            <div className="flex flex-col gap-4 border-t pt-3">
              <div className="flex flex-col gap-1.5">
                <SectionLabel>Outcomes</SectionLabel>
                <div className="flex flex-wrap gap-1.5">
                  {OUTCOMES.map((s) => (
                    <OutcomeChip
                      key={s}
                      status={s}
                      pressed={!exclude.includes(s)}
                      dim={exclude.includes(s)}
                      onClick={() => toggleOutcome(s)}
                    />
                  ))}
                </div>
              </div>
              <div className="flex flex-col gap-1.5">
                <SectionLabel>Who moves next</SectionLabel>
                <div role="radiogroup" aria-label="Category" className="grid grid-cols-5 gap-0.5 rounded-[10px] bg-muted p-0.5">
                  {[["", "Any"] as const, ...CATEGORY_ORDER.map((c) => [c, CATEGORY_SHORT[c]] as const)].map(([value, text]) => (
                    <button
                      key={value || "any"}
                      type="button"
                      role="radio"
                      aria-checked={category === value}
                      title={value ? CATEGORY_LABELS[value] : "Any category"}
                      onClick={() => onPatch({ category: value, status: "" })}
                      className={`h-7 truncate rounded-lg px-1 text-xs outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                        category === value ? "bg-card font-semibold text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground"
                      }`}
                    >
                      {text}
                    </button>
                  ))}
                </div>
              </div>
              <div className="flex flex-col gap-1.5">
                <SectionLabel>Steps between</SectionLabel>
                <SubSteps current={status} onPick={(s) => onPatch({ status: status === s ? "" : s, category: "" })} />
              </div>
              <div className="flex items-center justify-between gap-3">
                <SectionLabel>Sort</SectionLabel>
                <Select value={sort} onValueChange={(value) => onPatch({ sort: value })}>
                  <SelectTrigger size="sm" className="w-48" aria-label="Sort">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {SORTS.map(([value, text]) => (
                      <SelectItem key={value} value={value}>
                        {text}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}

// Opt-in (Settings › Table): the stages as a bar above the table, each segment
// sized by how many applications sit on it. Click one to filter to it.
function StageFunnel({ status, refresh, onPick }: { status: string; refresh: unknown; onPick: (status: string) => void }) {
  const [counts, setCounts] = useState<number[] | null>(null)
  useEffect(() => {
    let live = true
    const requests = STAGE_STOPS.map(([s]) => ({ method: "GET", path: `/v1/applications?status=${s}&limit=1` }))
    api<{ results: { status: number; body: unknown }[] }>("/v1/batch", { method: "POST", body: { requests } })
      .then((r) => {
        if (!live) return
        setCounts(
          r.results.map((x) => {
            const body = (typeof x.body === "string" ? JSON.parse(x.body) : x.body) as Page<Application> | null
            return body?.page?.total ?? 0
          }),
        )
      })
      .catch(() => live && setCounts(null))
    return () => {
      live = false
    }
  }, [refresh])
  if (!counts) return null
  return (
    <div role="radiogroup" aria-label="Stage" className="flex h-12 gap-[3px]">
      {STAGE_STOPS.map(([s, text], i) => {
        const on = status === s
        const gold = s === "offer"
        return (
          <button
            key={s}
            type="button"
            role="radio"
            aria-checked={on}
            onClick={() => onPick(on ? "" : s)}
            style={{ flex: `${Math.max(counts[i], 1)} 1 0px` }}
            className={`flex min-w-16 flex-col items-start justify-center rounded-[3px] px-3 text-left outline-none transition-colors first:rounded-l-[10px] last:rounded-r-[10px] focus-visible:ring-2 focus-visible:ring-ring ${
              on
                ? gold
                  ? "bg-brand-gold text-[#141615]"
                  : "bg-primary text-primary-foreground"
                : gold
                  ? "bg-brand-gold/25 text-gold-ink hover:bg-brand-gold/40"
                  : "bg-muted hover:bg-border"
            }`}
          >
            <span className="font-mono text-sm font-medium tabular-nums">{counts[i]}</span>
            <span className="text-xs opacity-85">{text}</span>
          </button>
        )
      })}
    </div>
  )
}

function ScrollJumpButton() {
  const [atTop, setAtTop] = useState(true)
  const [atBottom, setAtBottom] = useState(false)

  useEffect(() => {
    function onScroll() {
      const scrollY = window.scrollY
      const max = document.documentElement.scrollHeight - window.innerHeight
      setAtTop(scrollY < 200)
      setAtBottom(max - scrollY < 200)
    }
    onScroll()
    window.addEventListener("scroll", onScroll, { passive: true })
    return () => window.removeEventListener("scroll", onScroll)
  }, [])

  if (atTop === atBottom) return null
  return (
    <Button
      variant="outline"
      size="icon"
      className="fixed bottom-6 right-6 size-10 rounded-full shadow-md"
      aria-label={atTop ? "Scroll to bottom" : "Scroll to top"}
      onClick={() => window.scrollTo({ top: atTop ? document.documentElement.scrollHeight : 0, behavior: "smooth" })}
    >
      {atTop ? <ArrowDown className="size-4" /> : <ArrowUp className="size-4" />}
    </Button>
  )
}

function SortHeader({
  text,
  field,
  sort,
  onSort,
  defaultDesc,
}: {
  text: string
  field: string
  sort: string
  onSort: (next: string) => void
  defaultDesc?: boolean
}) {
  const isAsc = sort === field
  const isDesc = sort === "-" + field
  function click() {
    if (isAsc) onSort("-" + field)
    else if (isDesc) onSort(field)
    else onSort(defaultDesc ? "-" + field : field)
  }
  return (
    <button type="button" className="flex items-center gap-1 hover:text-foreground" onClick={click}>
      {text}
      {isAsc && <ArrowUp className="size-3" />}
      {isDesc && <ArrowDown className="size-3" />}
    </button>
  )
}

function IconAction({
  label: text,
  onClick,
  disabled,
  danger,
  children,
}: {
  label: string
  onClick: () => void
  disabled?: boolean
  danger?: boolean
  children: ReactNode
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className={`size-7 ${danger ? "text-destructive hover:text-destructive" : ""}`}
          aria-label={text}
          onClick={onClick}
          disabled={disabled}
        >
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{text}</TooltipContent>
    </Tooltip>
  )
}

function hostnameLabel(url?: string) {
  if (!url) return ""
  try {
    return new URL(url).hostname.replace(/^www\./, "")
  } catch {
    return ""
  }
}


function RowActions({
  row,
  can,
  onChanged,
  onDelete,
  onArchive,
  onChangeStatus,
  menuOpen,
  onMenuOpenChange,
}: {
  row: Application
  can: (scope: string) => boolean
  onChanged: () => void
  onDelete: () => void
  onArchive: () => void
  onChangeStatus: () => void
  menuOpen: boolean
  onMenuOpenChange: (open: boolean) => void
}) {
  const [busy, setBusy] = useState(false)

  async function decide(kind: "approve" | "reject") {
    setBusy(true)
    try {
      await api(kind === "approve" ? "/v1/approvals" : "/v1/approvals/reject", {
        method: "POST",
        body: { ids: [row.id], note: "" },
      })
      onChanged()
    } finally {
      setBusy(false)
    }
  }

  async function unarchive() {
    setBusy(true)
    try {
      await api(`/v1/applications/${encodeURIComponent(row.id)}/unarchive`, { method: "POST", body: { version: row.version } })
      onChanged()
    } finally {
      setBusy(false)
    }
  }

  const hasMenu = can("write")
  const href = safeURL(row.posting_url)

  return (
    <span className="flex w-[5.5rem] items-center justify-end gap-0.5">
      {shown(row) === "awaiting_review" && can("approve") && (
        <>
          <IconAction label="Approve" onClick={() => decide("approve")} disabled={busy}>
            <Check className="size-3.5" />
          </IconAction>
          <IconAction label="Reject" onClick={() => decide("reject")} disabled={busy} danger>
            <X className="size-3.5" />
          </IconAction>
        </>
      )}
      {hasMenu && (
        <DropdownMenu open={menuOpen} onOpenChange={onMenuOpenChange}>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="size-7" disabled={busy} aria-label="More actions">
              <MoreHorizontal className="size-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="end"
            className="min-w-48"
            // Keep focus off the trigger so Space still opens quick actions.
            onCloseAutoFocus={(e) => e.preventDefault()}
            // Letter keys act on the row: y approve, r reject, s status, a archive, d delete.
            onKeyDown={(e) => {
              if (e.ctrlKey || e.metaKey || e.altKey) return
              const k = e.key.toLowerCase()
              const review = shown(row) === "awaiting_review" && can("approve")
              if (k === "s") onChangeStatus()
              else if (href && k === "o") window.open(href, "_blank", "noopener,noreferrer")
              else if (review && k === "y") void decide("approve")
              else if (review && k === "r") void decide("reject")
              else if (k === "a") {
                if (row.archived_at) void unarchive()
                else onArchive()
              }
              else if (k === "d" || k === "delete" || k === "backspace") onDelete()
              else return
              e.preventDefault()
              onMenuOpenChange(false)
            }}
          >
            {shown(row) === "awaiting_review" && can("approve") && (
              <>
                <DropdownMenuItem onClick={() => decide("approve")}>
                  <Check /> Approve <Kbd className="ml-auto">Y</Kbd>
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => decide("reject")}>
                  <X /> Reject <Kbd className="ml-auto">R</Kbd>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
              </>
            )}
            <DropdownMenuItem onClick={onChangeStatus}>
              <ListChecks /> Change status <Kbd className="ml-auto">S</Kbd>
            </DropdownMenuItem>
            {href && (
              <DropdownMenuItem onClick={() => window.open(href, "_blank", "noopener,noreferrer")}>
                <ExternalLink /> Open posting <Kbd className="ml-auto">O</Kbd>
              </DropdownMenuItem>
            )}
            {row.archived_at ? (
              <DropdownMenuItem onClick={unarchive}>
                <ArchiveRestore /> Unarchive <Kbd className="ml-auto">A</Kbd>
              </DropdownMenuItem>
            ) : (
              <DropdownMenuItem onClick={onArchive}>
                <Archive /> Archive <Kbd className="ml-auto">A</Kbd>
              </DropdownMenuItem>
            )}
            <DropdownMenuItem onClick={onDelete} variant="destructive">
              <Trash2 /> Delete <Kbd className="ml-auto">D</Kbd>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </span>
  )
}

export function PipelineView({ initial }: { initial: PipelineInitial | null }) {
  const { can } = useAuth()
  const router = useRouter()
  const pathname = usePathname()
  const params = useSearchParams()
  const status = params.get("status") ?? getDefaultStatus()
  const category = params.get("category") || ""
  const q = params.get("q") || ""
  const sort = params.get("sort") || getDefaultSort()
  const archivedView = params.get("archived") === "1"
  const excludeParam = params.get("exclude") || ""
  const exclude = excludeParam.split(",").filter(Boolean)

  const [rows, setRows] = useState<Application[]>(initial?.rows ?? [])
  const [total, setTotal] = useState(initial?.total ?? 0)
  const [nextCursor, setNextCursor] = useState(initial?.nextCursor ?? "")
  const [loading, setLoading] = useState(initial == null)
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [deleteTarget, setDeleteTarget] = useState<Application | null>(null)
  const [archiveTarget, setArchiveTarget] = useState<Application | null>(null)
  const [cursor, setCursor] = useState<string | null>(null)
  const [menuFor, setMenuFor] = useState<string | null>(null)
  // Read after mount: the server render has no localStorage.
  const [funnel, setFunnel] = useState(false)
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setFunnel(getPref("funnel") === "on"), [])
  const [statusFor, setStatusFor] = useState<string | null>(null)
  const [bulkMenu, setBulkMenu] = useState(false)
  const [bulkConfirm, setBulkConfirm] = useState<"archive" | "delete" | null>(null)
  const [bulkBusy, setBulkBusy] = useState(false)
  const [checkingAll, setCheckingAll] = useState(false)
  const [fields, setFields] = useState<CustomField[]>(initial?.fields ?? [])
  const loadGen = useRef(0)
  const columnCount = 7 + fields.length

  // The server already fetched the first page (and fields) for this exact
  // URL — skip the redundant client refetch on mount, but still refetch
  // normally on every filter/sort change after that.
  const skipNextLoad = useRef(initial != null)
  const skipFieldsLoad = useRef(initial != null)

  useEffect(() => {
    if (skipFieldsLoad.current) {
      skipFieldsLoad.current = false
      return
    }
    api<Page<CustomField>>("/v1/fields?limit=100")
      .then((page) => setFields(page.data || []))
      .catch(() => setFields([]))
  }, [])

  async function load(cursor?: string) {
    const gen = ++loadGen.current
    setLoading(true)
    const qs = buildApplicationsQuery({ status, category, sort, archivedView, exclude, cursor })
    const key = qs.toString()
    // Show the last result for this filter straight away, then refresh it.
    const hit = !cursor && pageCache.get(key)
    if (hit) {
      setRows(hit.rows)
      setTotal(hit.total)
      setNextCursor(hit.nextCursor)
    }
    try {
      const page = await api<Page<Application>>("/v1/applications?" + key)
      if (gen !== loadGen.current) return
      if (!cursor) pageCache.set(key, { rows: page.data, total: page.page.total, nextCursor: page.page.next_cursor || "" })
      setRows((prev) => (cursor ? prev.concat(page.data) : page.data))
      setTotal(page.page.total)
      setNextCursor(page.page.next_cursor || "")
      setError(null)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load applications")
    } finally {
      if (gen === loadGen.current) setLoading(false)
    }
  }

  useEffect(() => {
    if (skipNextLoad.current) {
      skipNextLoad.current = false
      if (initial) {
        const key = buildApplicationsQuery({ status, category, sort, archivedView, exclude }).toString()
        pageCache.set(key, { rows: initial.rows, total: initial.total, nextCursor: initial.nextCursor })
      }
      return
    }
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, category, sort, archivedView, excludeParam])

  function patch(next: Record<string, string>) {
    const merged = new URLSearchParams(params.toString())
    for (const [key, value] of Object.entries(next)) {
      if (value || key === "status") merged.set(key, value)
      else merged.delete(key)
    }
    // A shallow URL update: useSearchParams follows it at once and load()
    // fetches the rows, where router.replace would re-render the server page
    // first and leave the filter showing the old choice meanwhile.
    window.history.replaceState(null, "", `${pathname}?${merged.toString()}`)
  }

  const visibleRows = useMemo(() => {
    let out = rows
    if (q) {
      const { free, qualifiers } = parseSearch(q)
      const base = qualifiers.length ? rows.filter((row) => matchesQualifiers(row, qualifiers)) : rows
      out = !free
        ? base
        : base
            .map((row) => {
              const haystack = [row.company, row.title, label(shown(row)), ...(row.tags || [])].filter(Boolean).join(" ")
              return { row, score: commandScore(haystack, free) }
            })
            .filter((entry) => entry.score > 0)
            .sort((a, b) => b.score - a.score)
            .map((entry) => entry.row)
    }
    if (sort === "category") {
      out = out
        .map((row, i) => ({ row, i, rank: CATEGORY_ORDER.indexOf(statusCategory(shown(row))) }))
        .sort((a, b) => a.rank - b.rank || a.i - b.i)
        .map((entry) => entry.row)
    }
    return out
  }, [rows, q, sort])

  function toggleSelected(id: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function toggleSelectAll() {
    setSelected((prev) => (prev.size === visibleRows.length ? new Set() : new Set(visibleRows.map((r) => r.id))))
  }

  const selectedRows = visibleRows.filter((r) => selected.has(r.id))
  // The highlighted row, dropped if a filter or search removed it.
  const cursorId = visibleRows.find((r) => r.id === cursor)?.id ?? null

  useEffect(() => {
    document.querySelector('[data-cursor="true"]')?.scrollIntoView({ block: "nearest" })
  }, [cursorId])

  // Keyboard handlers stand down while a row menu is open: it owns the keys.
  const idle = () => menuFor == null && statusFor == null && !bulkMenu
  const onControl = () => document.activeElement?.matches("button, a, [role='checkbox']") ?? false
  const move = (d: number) => {
    if (!visibleRows.length || !idle()) return false
    const i = visibleRows.findIndex((r) => r.id === cursorId)
    setCursor(visibleRows[Math.min(visibleRows.length - 1, Math.max(0, i < 0 ? 0 : i + d))].id)
  }
  async function changeStatus(row: Application, next: string) {
    if (next === shown(row)) return
    setError(null)
    try {
      const updated = await api<Application>(`/v1/applications/${encodeURIComponent(row.id)}`, {
        method: "PATCH",
        body: { version: row.version, status: next },
      })
      setRows((prev) => prev.map((r) => (r.id === row.id ? { ...r, ...updated } : r)))
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to change the status")
    }
  }
  // After the chooser changes a row's rounds: refetch it for the new mark.
  async function refreshRow(row: Application) {
    const updated = await api<Application>(`/v1/applications/${encodeURIComponent(row.id)}`)
    setRows((prev) => prev.map((r) => (r.id === row.id ? { ...r, ...updated } : r)))
  }
  // [ and ] walk the pipeline stages for the highlighted row.
  const step = (d: number) => {
    const row = visibleRows.find((r) => r.id === cursorId)
    if (!row || !idle() || !can("write")) return false
    const from = Math.min(6, Math.max(0, markOf(row).stage))
    const next = from + d >= 0 && from + d <= 6 ? stageTarget(row, from, from + d) : null
    if (!next) return false
    void changeStatus(row, next)
  }
  useShortcuts({
    rowStatusNext: () => step(1),
    rowStatusPrev: () => step(-1),
    rowNext: () => move(1),
    rowPrev: () => move(-1),
    rowOpen: () => (cursorId && idle() && !onControl() ? router.push(applicationHref(cursorId)) : false),
    rowSelect: () => (cursorId && idle() && !onControl() ? toggleSelected(cursorId) : false),
    // With rows selected the menu acts on all of them, else on the highlighted row.
    rowMenu: () => {
      if (!idle() || onControl() || !can("write")) return false
      if (selected.size > 0) setBulkMenu(true)
      else if (cursorId) setMenuFor(cursorId)
      else return false
    },
    rowClear: () => {
      if (!idle() || (!cursorId && selected.size === 0)) return false
      setCursor(null)
      setSelected(new Set())
    },
  })
  const canBulkApprove = can("approve") && selectedRows.length > 0 && selectedRows.every((r) => shown(r) === "awaiting_review")

  async function bulkDecide(kind: "approve" | "reject") {
    setBulkBusy(true)
    try {
      await api(kind === "approve" ? "/v1/approvals" : "/v1/approvals/reject", {
        method: "POST",
        body: { ids: [...selected], note: "" },
      })
      setSelected(new Set())
      await load()
    } finally {
      setBulkBusy(false)
    }
  }

  async function bulkStatus(next: string) {
    setBulkBusy(true)
    try {
      await Promise.all(
        selectedRows
          .filter((row) => shown(row) !== next)
          .map((row) =>
            api(`/v1/applications/${encodeURIComponent(row.id)}`, { method: "PATCH", body: { version: row.version, status: next } }),
          ),
      )
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to change the status")
    } finally {
      setBulkBusy(false)
    }
  }

  async function bulkArchive() {
    setBulkBusy(true)
    try {
      await Promise.all(
        selectedRows.map((row) =>
          api(`/v1/applications/${encodeURIComponent(row.id)}/archive`, { method: "POST", body: { version: row.version } }),
        ),
      )
      setSelected(new Set())
      await load()
    } finally {
      setBulkBusy(false)
    }
  }

  async function bulkUnarchive() {
    setBulkBusy(true)
    try {
      await Promise.all(
        selectedRows.map((row) =>
          api(`/v1/applications/${encodeURIComponent(row.id)}/unarchive`, { method: "POST", body: { version: row.version } }),
        ),
      )
      setSelected(new Set())
      await load()
    } finally {
      setBulkBusy(false)
    }
  }

  async function bulkDelete() {
    setBulkBusy(true)
    try {
      await Promise.all(selectedRows.map((row) => api(`/v1/applications/${encodeURIComponent(row.id)}`, { method: "DELETE" })))
      setSelected(new Set())
      await load()
    } finally {
      setBulkBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">
          {selected.size > 0
            ? `${selected.size} of ${visibleRows.length} selected`
            : loading && rows.length === 0
            ? "Loading applications"
            : q
              ? `${visibleRows.length} of ${total} application${total === 1 ? "" : "s"} match "${q}"`
              : `${total} application${total === 1 ? "" : "s"}`}
        </p>
        {selected.size > 0 ? (
          // Same row as the count, so selecting never pushes the table down.
          <div className="flex gap-2">
            <DropdownMenu open={bulkMenu} onOpenChange={setBulkMenu}>
              <DropdownMenuTrigger asChild>
                <Button size="sm" variant="outline" disabled={bulkBusy}>
                  Actions <ChevronDown className="size-3.5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                align="end"
                className="min-w-56"
                onCloseAutoFocus={(e) => e.preventDefault()}
                // y approve, r reject, a archive, d delete, c clear.
                onKeyDown={(e) => {
                  if (e.ctrlKey || e.metaKey || e.altKey) return
                  const k = e.key.toLowerCase()
                  if (canBulkApprove && k === "y") void bulkDecide("approve")
                  else if (canBulkApprove && k === "r") void bulkDecide("reject")
                  else if (k === "a") {
                    if (archivedView) void bulkUnarchive()
                    else setBulkConfirm("archive")
                  }
                  else if (k === "d" || k === "delete" || k === "backspace") setBulkConfirm("delete")
                  else if (k === "c") setSelected(new Set())
                  else return
                  e.preventDefault()
                  setBulkMenu(false)
                }}
              >
                {canBulkApprove && (
                  <>
                    <DropdownMenuItem onClick={() => bulkDecide("approve")}>
                      <Check /> Approve {selected.size} <Kbd className="ml-auto">Y</Kbd>
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => bulkDecide("reject")}>
                      <X /> Reject {selected.size} <Kbd className="ml-auto">R</Kbd>
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                  </>
                )}
                <DropdownMenuSub>
                  <DropdownMenuSubTrigger>
                    <ListChecks /> Change status
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent className="max-h-80">
                    {STATUS_OPTIONS.map(([v, text]) => (
                      <DropdownMenuItem key={v} onSelect={() => void bulkStatus(v)}>
                        {text}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuSubContent>
                </DropdownMenuSub>
                {archivedView ? (
                  <DropdownMenuItem onClick={bulkUnarchive}>
                    <ArchiveRestore /> Unarchive <Kbd className="ml-auto">A</Kbd>
                  </DropdownMenuItem>
                ) : (
                  <DropdownMenuItem onClick={() => setBulkConfirm("archive")}>
                    <Archive /> Archive <Kbd className="ml-auto">A</Kbd>
                  </DropdownMenuItem>
                )}
                <DropdownMenuItem onClick={() => setBulkConfirm("delete")} variant="destructive">
                  <Trash2 /> Delete <Kbd className="ml-auto">D</Kbd>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => setSelected(new Set())}>
                  <X /> Clear selection <Kbd className="ml-auto">C</Kbd>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            <Button size="sm" variant="ghost" onClick={() => setSelected(new Set())}>
              Clear
            </Button>
          </div>
        ) : (
        <div className="flex gap-2">
          {archivedView && (
            <Button size="sm" variant="ghost" onClick={() => patch({ archived: "" })}>
              <ArchiveRestore className="size-3.5" /> Exit archive view
            </Button>
          )}
          {!archivedView && <FilterSortBar status={status} category={category} sort={sort} exclude={exclude} onPatch={patch} />}
          {can("write") && !archivedView && (
            <Button size="sm" asChild>
              <Link href="/app/new">
                <Plus /> Add
              </Link>
            </Button>
          )}
        </div>
        )}
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {funnel && !archivedView && <StageFunnel status={status} refresh={rows} onPick={(s) => patch({ status: s, category: "" })} />}


      <div className="rounded-md border">
        {/* Fixed column widths: a longer status, a tag, or the approve/reject icons never shift the other columns. */}
        <Table className="min-w-[56rem] table-fixed">
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox
                  checked={visibleRows.length > 0 && selected.size === visibleRows.length}
                  onCheckedChange={toggleSelectAll}
                  aria-label="Select all"
                />
              </TableHead>
              <TableHead className="w-[22%]">
                <SortHeader text="Company" field="company" sort={sort} onSort={(next) => patch({ sort: next })} />
              </TableHead>
              <TableHead className="w-[22%]">
                <SortHeader text="Role" field="title" sort={sort} onSort={(next) => patch({ sort: next })} />
              </TableHead>
              <TableHead className="w-72">
                Status
              </TableHead>
              {fields.map((f) => (
                <TableHead key={f.id} className="w-36">
                  <SortHeader text={f.name} field={`field:${f.name}`} sort={sort} onSort={(next) => patch({ sort: next })} />
                </TableHead>
              ))}
              <TableHead className="w-44">
                <span className="flex items-center gap-1">
                  Listing
                  {can("write") && (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-5"
                          aria-label="Re-check all posting links"
                          disabled={checkingAll}
                          onClick={async () => {
                            setCheckingAll(true)
                            try {
                              await api("/v1/links/check", { method: "POST", body: { owner: "dashboard" } })
                              await load()
                            } finally {
                              setCheckingAll(false)
                            }
                          }}
                        >
                          <RefreshCw className={`size-3 ${checkingAll ? "animate-spin" : ""}`} />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>Re-check all links</TooltipContent>
                    </Tooltip>
                  )}
                </span>
              </TableHead>
              <TableHead className="w-24">
                <SortHeader text="Date" field="applied_date" sort={sort} onSort={(next) => patch({ sort: next })} defaultDesc />
              </TableHead>
              <TableHead className="w-[5.5rem]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {visibleRows.length === 0 && !loading && (
              <TableRow>
                <TableCell colSpan={columnCount} className="text-center text-muted-foreground">
                  No applications match.
                </TableCell>
              </TableRow>
            )}
            {visibleRows.map((row) => {
              const href = safeURL(row.posting_url)
              const when = row.applied_date || row.updated_at
              return (
                <Fragment key={row.id}>
                  <TableRow
                    data-cursor={row.id === cursorId}
                    onClick={() => setCursor(row.id)}
                    className="data-[cursor=true]:bg-muted data-[cursor=true]:shadow-[inset_2px_0_0_var(--primary)]"
                  >
                    <TableCell>
                      <Checkbox checked={selected.has(row.id)} onCheckedChange={() => toggleSelected(row.id)} aria-label="Select row" />
                    </TableCell>
                    <TableCell className="overflow-hidden font-medium">
                      <span className="flex items-center gap-2">
                        <ProgressMark {...markOf(row)} size={20} />
                        <CompanyAvatar company={row.company} companyId={row.company_id} />
                        {row.company || "—"}
                      </span>
                    </TableCell>
                    <TableCell className="overflow-hidden text-ellipsis">
                      <Link href={applicationHref(row.id)} className="hover:underline">
                        {row.title || "Untitled"}
                      </Link>
                    </TableCell>
                    <TableCell className="overflow-hidden">
                      <span className="flex items-center gap-1.5">
                        {can("write") ? (
                          <StatusChooser
                            app={row}
                            open={statusFor === row.id}
                            onOpenChange={(open) => setStatusFor(open ? row.id : null)}
                            onStatus={(next) => changeStatus(row, next)}
                            onRoundsChange={() => refreshRow(row)}
                          >
                            <button type="button" className="rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-label="Change status">
                              <StatusBadge status={shown(row)} />
                            </button>
                          </StatusChooser>
                        ) : (
                          <StatusBadge status={shown(row)} />
                        )}
                        {(row.tags || []).map((tag) => (
                          <span key={tag} className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                            {tag}
                          </span>
                        ))}
                      </span>
                    </TableCell>
                    <TableCell className="overflow-hidden">
                      <span className="flex items-center gap-1.5">
                        <LinkBadge status={row.link_status} />
                        {href ? (
                          <a
                            href={href}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-xs text-muted-foreground hover:text-foreground hover:underline"
                          >
                            {sourceLabel(row.source) || hostnameLabel(href)}
                          </a>
                        ) : (
                          <span className="text-xs text-muted-foreground">{sourceLabel(row.source) || "—"}</span>
                        )}
                      </span>
                    </TableCell>
                    {fields.map((f) => (
                      <TableCell key={f.id} className="text-muted-foreground">
                        {row.custom_fields?.[f.name] || "—"}
                      </TableCell>
                    ))}
                    <TableCell
                      className="text-muted-foreground"
                      title={row.applied_date ? "Applied " + row.applied_date : when ? "Updated " + when : ""}
                    >
                      {shortWhen(when)}
                    </TableCell>
                    <TableCell>
                      <RowActions
                        row={row}
                        can={can}
                        onChanged={() => load()}
                        onDelete={() => setDeleteTarget(row)}
                        onArchive={() => setArchiveTarget(row)}
                        onChangeStatus={() => setStatusFor(row.id)}
                        menuOpen={menuFor === row.id}
                        onMenuOpenChange={(open) => setMenuFor(open ? row.id : null)}
                      />
                    </TableCell>
                  </TableRow>
                </Fragment>
              )
            })}
          </TableBody>
        </Table>
      </div>

      {nextCursor && (
        <Button variant="outline" onClick={() => load(nextCursor)} disabled={loading}>
          Load more
        </Button>
      )}

      <ScrollJumpButton />

      <ConfirmDialog
        open={bulkConfirm != null}
        onOpenChange={(open) => !open && setBulkConfirm(null)}
        title={bulkConfirm === "delete" ? `Delete ${selected.size} applications` : `Archive ${selected.size} applications`}
        description={
          bulkConfirm === "delete"
            ? "This permanently deletes the selected applications. This can't be undone."
            : "The selected applications move out of the pipeline. You can restore them from the archive."
        }
        confirmLabel={bulkConfirm === "delete" ? "Delete" : "Archive"}
        danger={bulkConfirm === "delete"}
        busy={bulkBusy}
        onConfirm={async () => {
          const kind = bulkConfirm
          setBulkConfirm(null)
          if (kind === "delete") await bulkDelete()
          else if (kind === "archive") await bulkArchive()
        }}
      />

      <ConfirmDialog
        open={archiveTarget != null}
        onOpenChange={(open) => !open && setArchiveTarget(null)}
        title="Archive application"
        description={`${archiveTarget?.title || "This application"} at ${archiveTarget?.company || "this company"} moves out of the pipeline. You can restore it from the archive.`}
        confirmLabel="Archive"
        onConfirm={async () => {
          if (!archiveTarget) return
          await api(`/v1/applications/${encodeURIComponent(archiveTarget.id)}/archive`, {
            method: "POST",
            body: { version: archiveTarget.version },
          })
          setArchiveTarget(null)
          await load()
        }}
      />

      <ConfirmDialog
        open={deleteTarget != null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete application"
        description={`This permanently deletes ${deleteTarget?.title || "this application"} at ${deleteTarget?.company || "this company"}. This can't be undone.`}
        confirmLabel="Delete"
        danger
        onConfirm={async () => {
          if (!deleteTarget) return
          await api(`/v1/applications/${encodeURIComponent(deleteTarget.id)}`, { method: "DELETE" })
          setDeleteTarget(null)
          await load()
        }}
      />
    </div>
  )
}
