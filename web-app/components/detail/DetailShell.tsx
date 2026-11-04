"use client"

import { useEffect, useRef, useState, type ReactNode } from "react"
import { useRouter } from "next/navigation"
import { api, ApiError } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { OUTLINE_TABS, type Pref } from "@/lib/settings"
import { usePref } from "@/lib/use-pref"
import type { Application, Artifact, CustomField, Event, Page } from "@/lib/types"
import { DetailHeader } from "@/components/detail/DetailHeader"
import { ListingCard } from "@/components/detail/ListingCard"
import { RejectChoice, RelatedCard } from "@/components/detail/RelatedCard"
import { OutlineCard } from "@/components/detail/OutlineCard"
import { EventsHistoryCard } from "@/components/detail/EventsHistoryCard"
import { ArtifactsCard } from "@/components/detail/ArtifactsCard"
import { SectionError } from "@/components/detail/SectionError"
import { AttributesSkeleton, ArtifactsSkeleton, EventsSkeleton, HeaderSkeleton } from "@/components/detail/SectionSkeletons"

export type DetailInitial = { application: Application; fields: CustomField[] }

// A server-rendered card that Settings, Details can hide.
export function ShowIf({ pref, children }: { pref: Pref; children: ReactNode }) {
  return usePref(pref) === "off" ? null : children
}

// One resource, fetched independently of the others — this is the actual
// fix for the reported bug: a slow/failed `events` call (say) no longer
// blocks `artifacts`, `questions`, or the page shell the way the old
// Promise.all-of-5 did, and each has its own retry instead of the whole
// page being stuck on "Loading…" forever.
function useSection<T>(path: string, enabled: boolean) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState(false)
  const [tick, setTick] = useState(0)

  useEffect(() => {
    if (!enabled) return
    let cancelled = false
    // Resets for this fetch (a prior failure shouldn't linger through a retry).
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setError(false)
    api<T>(path)
      .then((result) => {
        if (!cancelled) setData(result)
      })
      .catch(() => {
        if (!cancelled) setError(true)
      })
    return () => {
      cancelled = true
    }
  }, [path, enabled, tick])

  return { data, error, retry: () => setTick((t) => t + 1) }
}

export function DetailShell({ id, initial, children }: { id: string; initial: DetailInitial | null; children?: ReactNode }) {
  const router = useRouter()
  const { can } = useAuth()
  const [application, setApplication] = useState<Application | null>(initial?.application ?? null)
  const [fields, setFields] = useState<CustomField[]>(initial?.fields ?? [])
  const [notFound, setNotFound] = useState(false)
  // The newest server copy, read by queued saves so each sends the version
  // the previous one returned.
  const latest = useRef<Application | null>(application)
  const queue = useRef<Promise<unknown>>(Promise.resolve())
  const path = `/v1/applications/${encodeURIComponent(id)}`
  const [fieldError, setFieldError] = useState<string | null>(null)
  const writable = can("write")

  // Export mode only (initial is null there — no server to have fetched
  // this already): load the core record + fields client-side, same two
  // calls the hosted Server Component awaits.
  useEffect(() => {
    if (initial != null) return
    let cancelled = false
    Promise.all([api<Application>(`/v1/applications/${encodeURIComponent(id)}`), api<Page<CustomField>>("/v1/fields?limit=100")])
      .then(([app, fieldsPage]) => {
        if (cancelled) return
        latest.current = app
        setApplication(app)
        setFields(fieldsPage.data || [])
      })
      .catch((err) => {
        if (!cancelled && err instanceof ApiError && err.status === 404) setNotFound(true)
      })
    return () => {
      cancelled = true
    }
  }, [id, initial])

  // Export mode's independent per-section fetches (hosted mode gets these
  // as Suspense-streamed Server Components passed in via `children` instead).
  const exportMode = initial == null
  const events = useSection<Page<Event>>(`/v1/applications/${encodeURIComponent(id)}/events?limit=100`, exportMode)
  const artifacts = useSection<Page<Artifact>>(`/v1/applications/${encodeURIComponent(id)}/artifacts`, exportMode)

  function accept(app: Application) {
    latest.current = app
    setApplication(app)
  }

  async function reload() {
    accept(await api<Application>(path))
  }

  // Every write to the record runs through one queue, so overlapping saves
  // can't send the same version and lose each other. A 409 means someone
  // else changed it: refetch and apply this field's edit on top, once.
  function enqueue(send: (version: number) => Promise<Application>) {
    const run = queue.current.then(async () => {
      try {
        accept(await send(latest.current?.version ?? 0))
      } catch (err) {
        if (!(err instanceof ApiError && err.status === 409)) throw err
        await reload()
        accept(await send(latest.current?.version ?? 0))
      }
    })
    queue.current = run.catch(() => {})
    return run
  }

  function patchApp(patch: Record<string, unknown>) {
    return enqueue((version) => api<Application>(path, { method: "PATCH", body: { version, ...patch } }))
  }

  // Rejecting an application that is tracked with others asks first whether
  // the rest go too; rejecting just this one stays the default.
  const [rejectAsk, setRejectAsk] = useState<{ others: Application[]; send: () => Promise<unknown> } | null>(null)
  async function guardReject(patch: Record<string, unknown>, send: () => Promise<unknown>) {
    const app = latest.current
    if (patch.status !== "rejected" || !app?.company_id) return send()
    const primary = app.tracked_by || app.id
    const page = await api<Page<Application>>(`/v1/applications?company_id=${encodeURIComponent(app.company_id)}&limit=100`).catch(() => null)
    const others = (page?.data ?? []).filter((a) => a.id !== app.id && (a.id === primary || a.tracked_by === primary) && !["rejected", "withdrawn", "ghosted", "failed"].includes(a.outcome || ""))
    if (!others.length) return send()
    setRejectAsk({ others, send })
  }
  const guardedPatch = (patch: Record<string, unknown>) => guardReject(patch, () => patchApp(patch))

  // Status changes from the header keep their visible error.
  function onSaveField(patch: Record<string, string>) {
    setFieldError(null)
    guardReject(patch, () => patchApp(patch)).catch((err) => setFieldError(err instanceof ApiError ? err.message : "Failed to save"))
  }

  function onSaveCustomFieldValue(fieldID: string, value: string) {
    return enqueue(() => api<Application>(`${path}/fields/${encodeURIComponent(fieldID)}`, { method: "POST", body: { value } }))
  }

  // Bumped after the header's chooser changes rounds so the rounds card refetches.
  const [roundsTick, setRoundsTick] = useState(0)
  async function onRoundsChange() {
    setRoundsTick((n) => n + 1)
    await reload()
  }

  // Settings, Details: which cards show, and which Details tabs have a card of their own.
  const showListing = usePref("cardListing") === "on"
  const showRelated = usePref("cardRelated") === "on"
  const showArtifacts = usePref("cardArtifacts") === "on"
  const showHistory = usePref("cardHistory") === "on"
  const places = [usePref("tabRounds"), usePref("tabTasks"), usePref("tabQuestions"), usePref("tabNotes")]
  // A change in one outline card refetches the others.
  const [outlineTick, setOutlineTick] = useState(0)
  async function onOutlineChange() {
    setOutlineTick((n) => n + 1)
    await reload()
  }

  async function onRecheckLink() {
    if (!application) return
    await api(`/v1/applications/${encodeURIComponent(application.id)}/check-link`, { method: "POST", body: {} })
    await reload()
  }

  async function onArchive() {
    if (!application) return
    await api(`/v1/applications/${encodeURIComponent(application.id)}/archive`, {
      method: "POST",
      body: { version: application.version },
    })
    await reload()
  }

  async function onUnarchive() {
    if (!application) return
    await api(`/v1/applications/${encodeURIComponent(application.id)}/unarchive`, {
      method: "POST",
      body: { version: application.version },
    })
    await reload()
  }

  async function onDecide(kind: "approve" | "reject", note: string) {
    if (!application) return
    await api(kind === "approve" ? "/v1/approvals" : "/v1/approvals/reject", {
      method: "POST",
      body: { ids: [application.id], note },
    })
    await reload()
  }

  async function onDelete() {
    if (!application) return
    await api(`/v1/applications/${encodeURIComponent(application.id)}`, { method: "DELETE" })
    router.push("/applications")
  }

  if (notFound) return <p className="text-sm text-destructive">That application doesn&apos;t exist.</p>

  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-4">
      {!application ? (
        <>
          <HeaderSkeleton />
          <AttributesSkeleton />
        </>
      ) : (
        <>
          <DetailHeader
            application={application}
            writable={writable}
            canApprove={can("approve")}
            onSaveField={onSaveField}
            onPatch={guardedPatch}
            onArchive={onArchive}
            onUnarchive={onUnarchive}
            onDecide={onDecide}
            onDelete={onDelete}
            onRoundsChange={onRoundsChange}
          />
          {showListing && (
            <ListingCard
              application={application}
              fields={fields}
              writable={writable}
              onSaveField={patchApp}
              onSaveCustomFieldValue={onSaveCustomFieldValue}
              onRecheckLink={onRecheckLink}
              fieldError={fieldError}
            />
          )}
          {/* Details under the Listing, then each tab Settings moved out to a card of its own. */}
          {([undefined, ...OUTLINE_TABS.filter((_, i) => places[i] === "card").map(([v]) => v)] as const).map((only) => (
            <OutlineCard key={`${only}-${roundsTick}`} only={only} tick={outlineTick} application={application} writable={writable} onChange={onOutlineChange} onSaveNotes={(notes) => patchApp({ notes })} onPatch={patchApp} />
          ))}
          {showRelated && <RelatedCard application={application} writable={writable} onChange={reload} />}
        </>
      )}

      <RejectChoice
        open={!!rejectAsk}
        onOpenChange={(v) => !v && setRejectAsk(null)}
        name={application?.title || "this application"}
        company={application?.company || ""}
        others={rejectAsk?.others.length ?? 0}
        onOne={() => {
          const ask = rejectAsk
          setRejectAsk(null)
          void ask?.send()
        }}
        onAll={() => {
          const ask = rejectAsk
          setRejectAsk(null)
          if (!ask) return
          void Promise.all([ask.send(), ...ask.others.map((a) => api(`/v1/applications/${encodeURIComponent(a.id)}`, { method: "PATCH", body: { version: a.version, status: "rejected" } }))]).then(() => reload())
        }}
      />

      {exportMode ? (
        <>
          {!showArtifacts ? null : artifacts.error ? (
            <SectionError label="artifacts" onRetry={artifacts.retry} />
          ) : artifacts.data == null ? (
            <ArtifactsSkeleton />
          ) : (
            <ArtifactsCard appId={id} artifacts={artifacts.data.data || []} />
          )}
          {!showHistory ? null : events.error ? (
            <SectionError label="history" onRetry={events.retry} />
          ) : events.data == null ? (
            <EventsSkeleton />
          ) : (
            <EventsHistoryCard events={events.data.data || []} />
          )}
        </>
      ) : (
        children
      )}
    </div>
  )
}
