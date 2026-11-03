"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { api, ApiError } from "@/lib/api-client"
import { applicationHref } from "@/lib/links"
import { displayed, outcomeKind, type Application, type Page } from "@/lib/types"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { StatusBadge } from "@/components/StatusBadge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

// Other postings at the same company. One process often covers several: track
// them with this application instead of applying to each, mark one not
// considered (rejected) on its own, or close the whole group at once.
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

  const row = (app: Application, action?: React.ReactNode) => (
    <li key={app.id} className="flex items-center gap-2 py-1.5 text-sm">
      <Link href={applicationHref(app.id)} className="min-w-0 flex-1 truncate hover:underline">
        {app.title || "Untitled"}
      </Link>
      <StatusBadge status={displayed(app)} />
      {writable && action}
    </li>
  )

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Related postings</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {application.tracked_by ? (
          <div className="flex items-center gap-2 text-sm">
            <span className="text-muted-foreground">Tracked with</span>
            <Link href={applicationHref(application.tracked_by)} className="min-w-0 flex-1 truncate hover:underline">
              {primary?.title || application.tracked_by}
            </Link>
            {writable && (
              <Button size="xs" variant="ghost" disabled={busy} onClick={() => run(() => patch(application, { tracked_by: "" }))}>
                Stop tracking
              </Button>
            )}
          </div>
        ) : (
          <>
            {tracked.length > 0 && (
              <ul className="divide-y">
                {tracked.map((app) =>
                  row(
                    app,
                    <>
                      {outcomeKind(app.outcome) !== "stop" && (
                        <Button size="xs" variant="ghost" disabled={busy} onClick={() => run(() => patch(app, { status: "rejected" }))}>
                          Not considered
                        </Button>
                      )}
                      <Button size="xs" variant="ghost" disabled={busy} onClick={() => run(() => patch(app, { tracked_by: "" }))}>
                        Untrack
                      </Button>
                    </>,
                  ),
                )}
              </ul>
            )}
            {trackable.length > 0 && (
              <div className="flex flex-col gap-1">
                <p className="text-xs text-muted-foreground">Also at {application.company}</p>
                <ul className="divide-y">
                  {trackable.map((app) =>
                    row(
                      app,
                      <Button size="xs" variant="ghost" disabled={busy} onClick={() => run(() => patch(app, { tracked_by: application.id }))}>
                        Track with this
                      </Button>,
                    ),
                  )}
                </ul>
              </div>
            )}
            {writable && tracked.length > 0 && open.length > 0 && (
              <Button size="sm" variant="outline" className="self-start" disabled={busy} onClick={() => setConfirm(true)}>
                Reject all related
              </Button>
            )}
          </>
        )}
        {error && <p className="text-xs text-destructive">{error}</p>}
      </CardContent>
      <ConfirmDialog
        open={confirm}
        onOpenChange={setConfirm}
        title="Reject all related?"
        description={`Marks ${open.length} application${open.length === 1 ? "" : "s"} at ${application.company} rejected: this one and the postings tracked with it.`}
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
