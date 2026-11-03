"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { Link2, Unlink, X } from "lucide-react"
import { api, ApiError } from "@/lib/api-client"
import { applicationHref } from "@/lib/links"
import { displayed, markOf, outcomeKind, type Application, type Page } from "@/lib/types"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { ProgressMark } from "@/components/ProgressMark"
import { StatusBadge } from "@/components/StatusBadge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Item, ItemActions, ItemContent, ItemGroup, ItemMedia } from "@/components/ui/item"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

function Action({ label, onClick, disabled, children }: { label: string; onClick: () => void; disabled: boolean; children: React.ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button variant="ghost" size="icon-xs" aria-label={label} disabled={disabled} onClick={onClick}>
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}

// Other postings at the same company. One process often covers several: track
// them with this application instead of applying to each, reject one on its
// own, or reject the whole group at once.
export function RelatedCard({
  application,
  writable,
  onChange,
}: {
  application: Application
  writable: boolean
  onChange: () => Promise<void>
}) {
  const [others, setOthers] = useState<Application[]>([])
  const [tick, setTick] = useState(0)
  const [busy, setBusy] = useState(false)
  const [confirm, setConfirm] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!application.company_id) return
    let cancelled = false
    api<Page<Application>>(`/v1/applications?company_id=${encodeURIComponent(application.company_id)}&limit=100`)
      .then((page) => {
        if (!cancelled) setOthers((page.data || []).filter((a) => a.id !== application.id))
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [application.company_id, application.id, tick])

  const primary = application.tracked_by ? others.find((a) => a.id === application.tracked_by) : undefined
  const tracked = others.filter((a) => a.tracked_by === application.id)
  // Track only postings that stand alone and have nothing tracked with them.
  const trackable = others.filter((a) => !a.tracked_by && !others.some((o) => o.tracked_by === a.id))
  const open = [application, ...tracked].filter((a) => outcomeKind(a.outcome) !== "stop")
  const openTracked = open.length - (open.includes(application) ? 1 : 0)

  if (!others.length && !application.tracked_by) return null

  async function run(work: () => Promise<unknown>) {
    setBusy(true)
    setError(null)
    try {
      await work()
      await onChange()
      setTick((n) => n + 1)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save")
    } finally {
      setBusy(false)
    }
  }

  const patch = (app: Application, body: Record<string, string>) =>
    api(`/v1/applications/${encodeURIComponent(app.id)}`, { method: "PATCH", body: { version: app.version, ...body } })

  const row = (app: Application, actions?: React.ReactNode) => (
    <Item key={app.id} variant="outline" size="sm" role="listitem">
      <ItemMedia>
        <ProgressMark {...markOf(app)} size={20} />
      </ItemMedia>
      <ItemContent className="min-w-0 flex-row items-center gap-3">
        <Link href={applicationHref(app.id)} className="min-w-0 flex-1 truncate text-sm font-medium hover:underline">
          {app.title || "Untitled"}
        </Link>
        <StatusBadge status={displayed(app)} />
      </ItemContent>
      {writable && actions && <ItemActions>{actions}</ItemActions>}
    </Item>
  )

  const summary = application.tracked_by ? "Tracked" : tracked.length ? `${tracked.length} tracked` : ""

  return (
    <Card>
      <CardHeader className="flex flex-row items-baseline justify-between">
        <CardTitle className="text-base">Related postings</CardTitle>
        {summary && <span className="text-xs text-muted-foreground">{summary}</span>}
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {application.tracked_by ? (
          <>
            <p className="text-xs text-muted-foreground">One process at {application.company} covers this posting.</p>
            <ItemGroup className="gap-2">
              {primary
                ? row(
                    primary,
                    <Action label="Stop tracking" disabled={busy} onClick={() => run(() => patch(application, { tracked_by: "" }))}>
                      <Unlink className="size-3.5" />
                    </Action>,
                  )
                : null}
            </ItemGroup>
          </>
        ) : (
          <>
            {tracked.length > 0 && (
              <ItemGroup className="gap-2">
                {tracked.map((app) =>
                  row(
                    app,
                    <>
                      {outcomeKind(app.outcome) !== "stop" && (
                        <Action label="Reject this posting" disabled={busy} onClick={() => run(() => patch(app, { status: "rejected" }))}>
                          <X className="size-3.5" />
                        </Action>
                      )}
                      <Action label="Stop tracking" disabled={busy} onClick={() => run(() => patch(app, { tracked_by: "" }))}>
                        <Unlink className="size-3.5" />
                      </Action>
                    </>,
                  ),
                )}
              </ItemGroup>
            )}
            {trackable.length > 0 && (
              <div className="flex flex-col gap-2">
                <p className="text-xs text-muted-foreground">Also at {application.company}. Track one here instead of applying to it.</p>
                <ItemGroup className="gap-2">
                  {trackable.map((app) =>
                    row(
                      app,
                      <Action label="Track with this application" disabled={busy} onClick={() => run(() => patch(app, { tracked_by: application.id }))}>
                        <Link2 className="size-3.5" />
                      </Action>,
                    ),
                  )}
                </ItemGroup>
              </div>
            )}
            {writable && tracked.length > 0 && open.length > 0 && (
              <Button size="sm" variant="outline" className="self-start" disabled={busy} onClick={() => setConfirm(true)}>
                <X />
                Reject all
              </Button>
            )}
          </>
        )}
        {error && <p className="text-xs text-destructive">{error}</p>}
      </CardContent>
      <ConfirmDialog
        open={confirm}
        onOpenChange={setConfirm}
        title={`Reject all at ${application.company}?`}
        description={`Rejects ${open.includes(application) ? "this application and " : ""}${openTracked} tracked posting${openTracked === 1 ? "" : "s"}. Each keeps its stage.`}
        confirmLabel="Reject all"
        danger
        busy={busy}
        onConfirm={() =>
          run(async () => {
            await Promise.all(open.map((app) => patch(app, { status: "rejected" })))
            setConfirm(false)
          })
        }
      />
    </Card>
  )
}
