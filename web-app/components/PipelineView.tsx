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
  Filter,
  MoreHorizontal,
  Plus,
  RefreshCw,
  SlidersHorizontal,
  Trash2,
  X,
} from "lucide-react"
import { api, ApiError } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { getDefaultSort, getDefaultStatus } from "@/lib/settings"
import { buildApplicationsQuery } from "@/lib/pipeline-query"
import { applicationHref } from "@/lib/links"
import {
  CATEGORY_LABELS,
  CATEGORY_ORDER,
  FILTERS,
  label,
  MORE,
  SORTS,
  sourceLabel,
  statusCategory,
  markOf,
  shown,
  type Application,
  type CustomField,
  type Page,
  type Question,
  type StatusCategory,
} from "@/lib/types"
import { CompanyAvatar } from "@/components/CompanyAvatar"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { PreparedAnswers } from "@/components/PreparedAnswers"
import { ProgressMark } from "@/components/ProgressMark"
import { StatusBadge } from "@/components/StatusBadge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Slider } from "@/components/ui/slider"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

const PRIMARY_STATUSES = FILTERS.filter(([value]) => value)
// "All" sits at position 0 on the slider, then each primary status in order.
const SLIDER_POSITIONS: [string, string][] = [["", "All"], ...PRIMARY_STATUSES]

export type PipelineInitial = {
  rows: Application[]
  total: number
  nextCursor: string
  fields: CustomField[]
}

function StatusSlider({ status, onChange }: { status: string; onChange: (value: string) => void }) {
  const index = Math.max(
    0,
    SLIDER_POSITIONS.findIndex(([value]) => value === status),
  )
  return (
    <div className="flex flex-col gap-3 px-1">
      <Slider
        min={0}
        max={SLIDER_POSITIONS.length - 1}
        step={1}
        value={[index]}
        onValueChange={([next]) => onChange(SLIDER_POSITIONS[next]![0])}
        className="[&_[data-slot=slider-thumb]]:size-4 [&_[data-slot=slider-thumb]]:border-0 [&_[data-slot=slider-thumb]]:bg-foreground [&_[data-slot=slider-thumb]]:shadow-md [&_[data-slot=slider-track]]:h-1.5"
      />
      <div className="flex justify-between">
        {SLIDER_POSITIONS.map(([value, text], i) => (
          <button
            key={value || "all"}
            type="button"
            onClick={() => onChange(value)}
            className={`text-[11px] leading-tight ${i === index ? "font-medium text-foreground" : "text-muted-foreground"}`}
          >
            {text}
          </button>
        ))}
      </div>
    </div>
  )
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

function FilterSortBar({
  status,
  category,
  sort,
  onPatch,
}: {
  status: string
  category: string
  sort: string
  onPatch: (next: Record<string, string>) => void
}) {
  const [moreOpen, setMoreOpen] = useState(false)
  const active = status !== "" || category !== ""

  function toggleStatus(value: string) {
    onPatch({ status: status === value ? "" : value, category: "" })
  }

  function toggleCategory(value: StatusCategory) {
    onPatch({ category: category === value ? "" : value, status: "" })
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="relative gap-1.5">
          <SlidersHorizontal className="size-3.5" /> Filter &amp; sort
          {active && <span className="absolute -right-1 -top-1 size-2 rounded-full bg-primary" />}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80">
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <span className="text-xs font-medium text-muted-foreground">Sort</span>
            <Select value={sort} onValueChange={(value) => onPatch({ sort: value })}>
              <SelectTrigger className="w-full" aria-label="Sort">
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
          <div className="flex flex-col gap-1.5">
            <span className="text-xs font-medium text-muted-foreground">Category</span>
            <div className="flex flex-wrap gap-1.5">
              {CATEGORY_ORDER.map((cat) => (
                <Button
                  key={cat}
                  size="sm"
                  variant={category === cat ? "default" : "outline"}
                  className="h-7 text-xs"
                  onClick={() => toggleCategory(cat)}
                >
                  {CATEGORY_LABELS[cat]}
                </Button>
              ))}
            </div>
          </div>
          <div className="flex flex-col gap-1.5">
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium text-muted-foreground">Status</span>
              {active && (
                <button
                  type="button"
                  className="text-xs text-muted-foreground underline"
                  onClick={() => onPatch({ status: "", category: "" })}
                >
                  Reset to all
                </button>
              )}
            </div>
            <StatusSlider status={status} onChange={(value) => onPatch({ status: value, category: "" })} />
            <Collapsible open={moreOpen} onOpenChange={setMoreOpen}>
              <CollapsibleTrigger className="mt-1 text-xs text-muted-foreground underline">
                {moreOpen ? "Fewer statuses" : "More statuses"}
              </CollapsibleTrigger>
              <CollapsibleContent className="mt-1.5 flex flex-wrap gap-1.5">
                {MORE.map(([value, text]) => (
                  <Button
                    key={value}
                    size="sm"
                    variant={status === value ? "default" : "outline"}
                    className="h-7 text-xs"
                    onClick={() => toggleStatus(value)}
                  >
                    {text}
                  </Button>
                ))}
              </CollapsibleContent>
            </Collapsible>
          </div>
        </div>
      </PopoverContent>
    </Popover>
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

function StatusExcludeFilter({ exclude, onChange }: { exclude: string[]; onChange: (next: string[]) => void }) {
  const active = exclude.length > 0
  function toggle(value: string) {
    onChange(exclude.includes(value) ? exclude.filter((v) => v !== value) : [...exclude, value])
  }
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" className="relative size-5" aria-label="Exclude statuses">
          <Filter className="size-3" />
          {active && <span className="absolute -right-0.5 -top-0.5 size-1.5 rounded-full bg-primary" />}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-56">
        <div className="flex flex-col gap-1.5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-muted-foreground">Exclude statuses</span>
            {active && (
              <button type="button" className="text-xs text-muted-foreground underline" onClick={() => onChange([])}>
                Clear
              </button>
            )}
          </div>
          <div className="flex max-h-48 flex-col gap-1 overflow-y-auto">
            {FILTERS.concat(MORE)
              .filter(([value]) => value)
              .map(([value, text]) => (
                <label key={value} className="flex items-center gap-2 text-sm">
                  <Checkbox checked={exclude.includes(value)} onCheckedChange={() => toggle(value)} />
                  {text}
                </label>
              ))}
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}

function ExpandedRow({ row, columnCount }: { row: Application; columnCount: number }) {
  const [questions, setQuestions] = useState<Question[] | null>(null)

  useEffect(() => {
    api<Page<Question>>(`/v1/applications/${encodeURIComponent(row.id)}/questions`)
      .then((page) => setQuestions(page.data || []))
      .catch(() => setQuestions([]))
  }, [row.id])

  return (
    <TableRow className="bg-muted/30">
      <TableCell colSpan={columnCount} className="py-3">
        <div className="flex flex-col gap-2">
          <span className="text-xs font-medium text-muted-foreground">Prepared answers</span>
          {questions == null ? (
            <p className="text-sm text-muted-foreground">Loading…</p>
          ) : (
            <PreparedAnswers questions={questions} />
          )}
          {row.notes && <p className="text-sm">{row.notes}</p>}
        </div>
      </TableCell>
    </TableRow>
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
}: {
  row: Application
  can: (scope: string) => boolean
  onChanged: () => void
  onDelete: () => void
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

  async function archive() {
    setBusy(true)
    try {
      await api(`/v1/applications/${encodeURIComponent(row.id)}/archive`, { method: "POST", body: { version: row.version } })
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

  return (
    <span className="flex items-center justify-end gap-0.5">
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
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="size-7" disabled={busy} aria-label="More actions">
              <MoreHorizontal className="size-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {row.archived_at ? (
              <DropdownMenuItem onClick={unarchive}>
                <ArchiveRestore /> Unarchive
              </DropdownMenuItem>
            ) : (
              <DropdownMenuItem onClick={archive}>
                <Archive /> Archive
              </DropdownMenuItem>
            )}
            <DropdownMenuItem onClick={onDelete} variant="destructive">
              <Trash2 /> Delete
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
  const [expanded, setExpanded] = useState<string | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [deleteTarget, setDeleteTarget] = useState<Application | null>(null)
  const [bulkBusy, setBulkBusy] = useState(false)
  const [checkingAll, setCheckingAll] = useState(false)
  const [fields, setFields] = useState<CustomField[]>(initial?.fields ?? [])
  const loadGen = useRef(0)
  const columnCount = 8 + fields.length

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
    try {
      const page = await api<Page<Application>>("/v1/applications?" + qs.toString())
      if (gen !== loadGen.current) return
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
    router.replace(`${pathname}?${merged.toString()}`, { scroll: false })
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
          {loading && rows.length === 0
            ? "Loading applications"
            : q
              ? `${visibleRows.length} of ${total} application${total === 1 ? "" : "s"} match "${q}"`
              : `${total} application${total === 1 ? "" : "s"}`}
        </p>
        <div className="flex gap-2">
          {archivedView && (
            <Button size="sm" variant="ghost" onClick={() => patch({ archived: "" })}>
              <ArchiveRestore className="size-3.5" /> Exit archive view
            </Button>
          )}
          {!archivedView && <FilterSortBar status={status} category={category} sort={sort} onPatch={patch} />}
          {can("write") && !archivedView && (
            <Button size="sm" asChild>
              <Link href="/app/new">
                <Plus /> Add
              </Link>
            </Button>
          )}
        </div>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {selected.size > 0 && (
        <div className="flex items-center gap-2 rounded-md border bg-muted/40 px-3 py-2">
          <span className="text-sm">{selected.size} selected</span>
          <div className="ml-auto flex gap-2">
            {canBulkApprove && (
              <>
                <Button size="sm" variant="outline" disabled={bulkBusy} onClick={() => bulkDecide("approve")}>
                  Approve
                </Button>
                <Button size="sm" variant="outline" disabled={bulkBusy} onClick={() => bulkDecide("reject")}>
                  Reject
                </Button>
              </>
            )}
            {can("write") && (
              <>
                {archivedView ? (
                  <Button size="sm" variant="outline" disabled={bulkBusy} onClick={bulkUnarchive}>
                    Unarchive
                  </Button>
                ) : (
                  <Button size="sm" variant="outline" disabled={bulkBusy} onClick={bulkArchive}>
                    Archive
                  </Button>
                )}
                <Button size="sm" variant="destructive" disabled={bulkBusy} onClick={bulkDelete}>
                  Delete
                </Button>
              </>
            )}
            <Button size="sm" variant="ghost" onClick={() => setSelected(new Set())}>
              Clear
            </Button>
          </div>
        </div>
      )}

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-8">
                <Checkbox
                  checked={visibleRows.length > 0 && selected.size === visibleRows.length}
                  onCheckedChange={toggleSelectAll}
                  aria-label="Select all"
                />
              </TableHead>
              <TableHead className="w-6" />
              <TableHead>
                <SortHeader text="Company" field="company" sort={sort} onSort={(next) => patch({ sort: next })} />
              </TableHead>
              <TableHead>
                <SortHeader text="Role" field="title" sort={sort} onSort={(next) => patch({ sort: next })} />
              </TableHead>
              <TableHead>
                <span className="flex items-center gap-1">
                  Status
                  <StatusExcludeFilter exclude={exclude} onChange={(next) => patch({ exclude: next.join(",") })} />
                </span>
              </TableHead>
              {fields.map((f) => (
                <TableHead key={f.id}>
                  <SortHeader text={f.name} field={`field:${f.name}`} sort={sort} onSort={(next) => patch({ sort: next })} />
                </TableHead>
              ))}
              <TableHead>
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
              <TableHead>
                <SortHeader text="Date" field="applied_date" sort={sort} onSort={(next) => patch({ sort: next })} defaultDesc />
              </TableHead>
              <TableHead className="w-10" />
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
              const isExpanded = expanded === row.id
              return (
                <Fragment key={row.id}>
                  <TableRow>
                    <TableCell>
                      <Checkbox checked={selected.has(row.id)} onCheckedChange={() => toggleSelected(row.id)} aria-label="Select row" />
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-6"
                        aria-label={isExpanded ? "Collapse" : "Show prepared answers"}
                        onClick={() => setExpanded(isExpanded ? null : row.id)}
                      >
                        {isExpanded ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                      </Button>
                    </TableCell>
                    <TableCell className="font-medium">
                      <span className="flex items-center gap-2">
                        <ProgressMark {...markOf(row)} size={20} />
                        <CompanyAvatar company={row.company} companyId={row.company_id} />
                        {row.company || "—"}
                      </span>
                    </TableCell>
                    <TableCell>
                      <Link href={applicationHref(row.id)} className="hover:underline">
                        {row.title || "Untitled"}
                      </Link>
                    </TableCell>
                    <TableCell>
                      <span className="flex items-center gap-1.5">
                        <StatusBadge status={shown(row)} />
                        {(row.tags || []).map((tag) => (
                          <span key={tag} className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                            {tag}
                          </span>
                        ))}
                      </span>
                    </TableCell>
                    <TableCell>
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
                      <RowActions row={row} can={can} onChanged={() => load()} onDelete={() => setDeleteTarget(row)} />
                    </TableCell>
                  </TableRow>
                  {isExpanded && <ExpandedRow row={row} columnCount={columnCount} />}
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
