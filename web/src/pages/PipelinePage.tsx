import { useEffect, useMemo, useRef, useState } from "react"
import { Link, useSearchParams } from "react-router-dom"
import commandScore from "command-score"
import { Check, ExternalLink, MoreHorizontal, RefreshCw, X } from "lucide-react"
import { api, ApiError } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { FILTERS, MORE, SORTS, label, statusTone, type Application, type Page } from "@/lib/types"
import { CompanyAvatar } from "@/components/CompanyAvatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

const STATUSES = FILTERS.concat(MORE)

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

function LinkDot({ status }: { status?: string }) {
  const dead = status === "closed"
  const text = dead ? "Dead link" : status === "live" ? "Posting is live" : status === "unknown" ? "Link status unknown" : "Link not checked"
  return (
    <span
      title={text}
      role="img"
      aria-label={text}
      className={`inline-block size-2 rounded-full ${dead ? "bg-destructive" : status === "live" ? "bg-emerald-500" : "bg-muted-foreground/40"}`}
    />
  )
}

function RowActions({ row, can, onChanged }: { row: Application; can: (scope: string) => boolean; onChanged: () => void }) {
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
          <DropdownMenuItem onClick={recheckLink}>
            <RefreshCw /> Re-check link
          </DropdownMenuItem>
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
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function PipelinePage() {
  const { can } = useAuth()
  const [params, setParams] = useSearchParams()
  const status = params.get("status") || ""
  const q = params.get("q") || ""
  const sort = params.get("sort") || "-updated_at"

  const [rows, setRows] = useState<Application[]>([])
  const [total, setTotal] = useState(0)
  const [nextCursor, setNextCursor] = useState("")
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [checkingLinks, setCheckingLinks] = useState(false)
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

  async function recheckAllLinks() {
    setCheckingLinks(true)
    try {
      await api("/v1/links/check", { method: "POST", body: { owner: "dashboard" } })
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to re-check links")
    } finally {
      setCheckingLinks(false)
    }
  }

  function patch(next: Record<string, string>) {
    const merged = new URLSearchParams(params)
    for (const [key, value] of Object.entries(next)) {
      if (value) merged.set(key, value)
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

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="flex items-center gap-1.5">
            <h1 className="text-xl font-semibold">Pipeline</h1>
            {can("write") && (
              <Button
                variant="ghost"
                size="icon"
                className="size-6"
                title="Re-check all posting links"
                aria-label="Re-check all posting links"
                onClick={recheckAllLinks}
                disabled={checkingLinks}
              >
                <RefreshCw className={checkingLinks ? "animate-spin" : ""} />
              </Button>
            )}
          </div>
          <p className="text-sm text-muted-foreground">
            {loading && rows.length === 0
              ? "Loading applications"
              : q
                ? `${visibleRows.length} of ${total} application${total === 1 ? "" : "s"} match "${q}"`
                : `${total} application${total === 1 ? "" : "s"}`}
          </p>
        </div>
        <div className="flex gap-2">
          <Select value={sort} onValueChange={(value) => patch({ sort: value })}>
            <SelectTrigger className="w-44" aria-label="Sort">
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

      <div className="flex flex-wrap gap-1.5">
        {STATUSES.map(([value, text]) => (
          <Button
            key={value}
            size="sm"
            variant={status === value ? "default" : "outline"}
            onClick={() => patch({ status: value })}
          >
            {text}
          </Button>
        ))}
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Company</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Link</TableHead>
              <TableHead>Date</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {visibleRows.length === 0 && !loading && (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground">
                  No applications match.
                </TableCell>
              </TableRow>
            )}
            {visibleRows.map((row) => {
              const href = safeURL(row.posting_url)
              const when = row.applied_date || row.updated_at
              return (
                <TableRow key={row.id}>
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
                    {(row.tags || []).length > 0 && (
                      <span className="ml-2 inline-flex gap-1">
                        {row.tags!.map((tag) => (
                          <Badge key={tag} variant="secondary" className="text-[10px]">
                            {tag}
                          </Badge>
                        ))}
                      </span>
                    )}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={row.status} />
                  </TableCell>
                  <TableCell>
                    <span className="flex items-center gap-2">
                      <LinkDot status={row.link_status} />
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
                    <RowActions row={row} can={can} onChanged={() => load()} />
                  </TableCell>
                </TableRow>
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
    </div>
  )
}
