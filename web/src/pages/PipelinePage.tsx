import { Fragment, useEffect, useMemo, useRef, useState } from "react"
import { Link, useSearchParams } from "react-router-dom"
import commandScore from "command-score"
import {
  Check,
  ChevronDown,
  ChevronRight,
  ExternalLink,
  MoreHorizontal,
  Pencil,
  Plus,
  RefreshCw,
  SlidersHorizontal,
  Trash2,
  X,
} from "lucide-react"
import { api, ApiError } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { getDefaultSort, getDefaultStatus } from "@/lib/settings"
import { FILTERS, MORE, SORTS, label, statusTone, type Application, type Page, type Question } from "@/lib/types"
import { CompanyAvatar } from "@/components/CompanyAvatar"
import { ApplicationDialog } from "@/components/ApplicationDialog"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { PreparedAnswers } from "@/components/PreparedAnswers"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

const PRIMARY_STATUSES = FILTERS.filter(([value]) => value)

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

function StatusBadge({ status }: { status: string }) {
  const tone = statusTone(status)
  const variant = tone === "bad" ? "destructive" : tone === "default" ? "secondary" : "outline"
  return <Badge variant={variant}>{label(status)}</Badge>
}

function LinkBadge({ status, onHoldRefresh, disabled }: { status?: string; onHoldRefresh: () => void; disabled?: boolean }) {
  const dead = status === "closed"
  const live = status === "live"
  const text = dead ? "Dead link" : live ? "Posting is live" : status === "unknown" ? "Link status unknown" : "Link not checked"
  const holdTimer = useRef<number | undefined>(undefined)
  const [holding, setHolding] = useState(false)

  function start() {
    if (disabled) return
    setHolding(true)
    holdTimer.current = window.setTimeout(() => {
      setHolding(false)
      onHoldRefresh()
    }, 600)
  }
  function cancel() {
    setHolding(false)
    window.clearTimeout(holdTimer.current)
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          role="button"
          tabIndex={0}
          aria-label={text + " — hold to re-check"}
          onPointerDown={start}
          onPointerUp={cancel}
          onPointerLeave={cancel}
          onKeyDown={(e) => e.key === "Enter" && onHoldRefresh()}
          className={`inline-block size-2.5 cursor-pointer rounded-full transition-transform ${holding ? "scale-150" : ""} ${dead ? "bg-destructive" : live ? "bg-emerald-500" : "bg-muted-foreground/40"}`}
        />
      </TooltipTrigger>
      <TooltipContent>{text} — hold to re-check</TooltipContent>
    </Tooltip>
  )
}

function FilterSortBar({
  status,
  sort,
  onPatch,
}: {
  status: string
  sort: string
  onPatch: (next: Record<string, string>) => void
}) {
  const [moreOpen, setMoreOpen] = useState(false)
  const active = status !== ""

  function toggleStatus(value: string) {
    onPatch({ status: status === value ? "" : value })
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
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium text-muted-foreground">Status</span>
              {active && (
                <button type="button" className="text-xs text-muted-foreground underline" onClick={() => onPatch({ status: "" })}>
                  Reset to all
                </button>
              )}
            </div>
            <div className="flex flex-wrap gap-1.5">
              {PRIMARY_STATUSES.map(([value, text]) => (
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
            </div>
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

function ExpandedRow({ row }: { row: Application }) {
  const [questions, setQuestions] = useState<Question[] | null>(null)

  useEffect(() => {
    api<Page<Question>>(`/v1/applications/${encodeURIComponent(row.id)}/questions`)
      .then((page) => setQuestions(page.data || []))
      .catch(() => setQuestions([]))
  }, [row.id])

  return (
    <TableRow className="bg-muted/30">
      <TableCell colSpan={8} className="py-3">
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

function RowActions({
  row,
  can,
  onChanged,
  onEdit,
  onDelete,
}: {
  row: Application
  can: (scope: string) => boolean
  onChanged: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  const [busy, setBusy] = useState(false)
  const href = safeURL(row.posting_url)

  async function recheckLink() {
    setBusy(true)
    try {
      await api(`/v1/applications/${encodeURIComponent(row.id)}/check-link`, { method: "POST", body: {} })
      onChanged()
    } finally {
      setBusy(false)
    }
  }

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
      await api(`/v1/applications/${encodeURIComponent(row.id)}`, {
        method: "PATCH",
        body: { version: row.version, status: "withdrawn" },
      })
      onChanged()
    } finally {
      setBusy(false)
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="size-7" disabled={busy} aria-label="Actions">
          <MoreHorizontal />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {href && (
          <DropdownMenuItem asChild>
            <a href={href} target="_blank" rel="noopener noreferrer">
              <ExternalLink /> Open posting
            </a>
          </DropdownMenuItem>
        )}
        {can("write") && (
          <>
            <DropdownMenuItem onClick={onEdit}>
              <Pencil /> Edit details
            </DropdownMenuItem>
            <DropdownMenuItem onClick={recheckLink}>
              <RefreshCw /> Re-check link
            </DropdownMenuItem>
          </>
        )}
        {row.status === "awaiting_review" && can("approve") && (
          <>
            <DropdownMenuItem onClick={() => decide("approve")}>
              <Check /> Approve
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => decide("reject")} variant="destructive">
              <X /> Reject
            </DropdownMenuItem>
          </>
        )}
        {can("write") && row.status !== "withdrawn" && (
          <DropdownMenuItem onClick={archive}>Archive</DropdownMenuItem>
        )}
        {can("write") && (
          <DropdownMenuItem onClick={onDelete} variant="destructive">
            <Trash2 /> Delete
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function PipelinePage() {
  const { can } = useAuth()
  const [params, setParams] = useSearchParams()
  const status = params.get("status") ?? getDefaultStatus()
  const q = params.get("q") || ""
  const sort = params.get("sort") || getDefaultSort()

  const [rows, setRows] = useState<Application[]>([])
  const [total, setTotal] = useState(0)
  const [nextCursor, setNextCursor] = useState("")
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [expanded, setExpanded] = useState<string | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [dialogApp, setDialogApp] = useState<Application | null | undefined>(undefined)
  const [deleteTarget, setDeleteTarget] = useState<Application | null>(null)
  const [bulkBusy, setBulkBusy] = useState(false)
  const loadGen = useRef(0)

  async function load(cursor?: string) {
    const gen = ++loadGen.current
    setLoading(true)
    const qs = new URLSearchParams()
    qs.set("limit", "50")
    if (sort) qs.set("sort", sort)
    if (status) qs.set("status", status)
    if (cursor) qs.set("cursor", cursor)
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
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, sort])

  useEffect(() => {
    if (params.get("new") === "1") {
      setDialogApp(null)
      const next = new URLSearchParams(params)
      next.delete("new")
      setParams(next, { replace: true })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function patch(next: Record<string, string>) {
    const merged = new URLSearchParams(params)
    for (const [key, value] of Object.entries(next)) {
      if (value || key === "status") merged.set(key, value)
      else merged.delete(key)
    }
    setParams(merged, { replace: true })
  }

  const visibleRows = useMemo(() => {
    if (!q) return rows
    return rows
      .map((row) => {
        const haystack = [row.company, row.title, label(row.status), ...(row.tags || [])].filter(Boolean).join(" ")
        return { row, score: commandScore(haystack, q) }
      })
      .filter((entry) => entry.score > 0)
      .sort((a, b) => b.score - a.score)
      .map((entry) => entry.row)
  }, [rows, q])

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
  const canBulkApprove = can("approve") && selectedRows.length > 0 && selectedRows.every((r) => r.status === "awaiting_review")

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
          api(`/v1/applications/${encodeURIComponent(row.id)}`, { method: "PATCH", body: { version: row.version, status: "withdrawn" } }),
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
          <FilterSortBar status={status} sort={sort} onPatch={patch} />
          {can("write") && (
            <Button size="sm" onClick={() => setDialogApp(null)}>
              <Plus /> Add
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
                <Button size="sm" variant="outline" disabled={bulkBusy} onClick={bulkArchive}>
                  Archive
                </Button>
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
              <TableHead>Company</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>
                <span className="flex items-center gap-1">
                  Link
                  {can("write") && (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-5"
                          aria-label="Re-check all posting links"
                          onClick={async () => {
                            await api("/v1/links/check", { method: "POST", body: { owner: "dashboard" } })
                            await load()
                          }}
                        >
                          <RefreshCw className="size-3" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>Re-check all links</TooltipContent>
                    </Tooltip>
                  )}
                </span>
              </TableHead>
              <TableHead>Date</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {visibleRows.length === 0 && !loading && (
              <TableRow>
                <TableCell colSpan={8} className="text-center text-muted-foreground">
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
                        <CompanyAvatar company={row.company} />
                        {row.company || "—"}
                      </span>
                    </TableCell>
                    <TableCell>
                      <Link to={`/app/${encodeURIComponent(row.id)}`} className="hover:underline">
                        {row.title || "Untitled"}
                      </Link>
                    </TableCell>
                    <TableCell>
                      <span className="flex items-center gap-1.5">
                        <StatusBadge status={row.status} />
                        {(row.tags || []).map((tag) => (
                          <span key={tag} className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                            {tag}
                          </span>
                        ))}
                      </span>
                    </TableCell>
                    <TableCell>
                      <span className="flex items-center gap-2">
                        <LinkBadge
                          status={row.link_status}
                          disabled={!can("write")}
                          onHoldRefresh={async () => {
                            await api(`/v1/applications/${encodeURIComponent(row.id)}/check-link`, { method: "POST", body: {} })
                            await load()
                          }}
                        />
                        {href && (
                          <a href={href} target="_blank" rel="noopener noreferrer" className="text-muted-foreground hover:text-foreground">
                            Open
                          </a>
                        )}
                      </span>
                    </TableCell>
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
                        onEdit={() => setDialogApp(row)}
                        onDelete={() => setDeleteTarget(row)}
                      />
                    </TableCell>
                  </TableRow>
                  {isExpanded && <ExpandedRow row={row} />}
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

      <ApplicationDialog
        open={dialogApp !== undefined}
        onOpenChange={(open) => !open && setDialogApp(undefined)}
        application={dialogApp}
        onSaved={() => load()}
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
