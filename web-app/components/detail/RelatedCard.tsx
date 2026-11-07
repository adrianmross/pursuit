"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { Link2, Unlink, X } from "lucide-react"
import { api, ApiError } from "@/lib/api-client"
import { applicationHref } from "@/lib/links"
import { displayed, markOf, outcomeKind, type Application, type Page } from "@/lib/types"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { ProgressMark } from "@/components/ProgressMark"
import { StatusBadge } from "@/components/StatusBadge"
import { Button } from "@/components/ui/button"
import { SectionCard } from "@/components/detail/SectionCard"
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
  const [rejecting, setRejecting] = useState<Application | null>(null)
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
    <SectionCard id="related" title="Related postings" summary={summary}>
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
                      <Action label="Reject this posting" disabled={busy} onClick={() => setRejecting(app)}>
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
        </>
      )}
      {error && <p className="text-xs text-destructive">{error}</p>}
    <RejectChoice
      open={!!rejecting}
      onOpenChange={(v) => !v && setRejecting(null)}
      name={rejecting?.title || "this posting"}
      company={application.company || ""}
      others={open.filter((a) => a.id !== rejecting?.id).length}
      busy={busy}
      onOne={() => rejecting && run(() => patch(rejecting, { status: "rejected" })).then(() => setRejecting(null))}
      onAll={() => run(() => Promise.all(open.map((a) => patch(a, { status: "rejected" })))).then(() => setRejecting(null))}
    />
    </SectionCard>
  )
}

// Rejecting one posting of a tracked group asks once: the others usually go
// with it, but rejecting just this one stays the default.
export function RejectChoice({ open, onOpenChange, name, company, others, busy, onOne, onAll }: { open: boolean; onOpenChange: (v: boolean) => void; name: string; company: string; others: number; busy?: boolean; onOne: () => void; onAll: () => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Reject {name}?</DialogTitle>
          <DialogDescription>
            It is tracked with {others} other posting{others === 1 ? "" : "s"} at {company}. One process usually ends for all of them. Each keeps its stage.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" disabled={busy} onClick={onAll}>
            Reject all {others + 1}
          </Button>
          <Button autoFocus variant="destructive" disabled={busy} onClick={onOne}>
            Reject this one
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
