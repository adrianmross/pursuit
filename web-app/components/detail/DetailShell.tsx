"use client"

import { useEffect, useState, type ReactNode } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { ArrowLeft } from "lucide-react"
import { api, ApiError } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import type { Application, Artifact, CustomField, Event, Page, Question } from "@/lib/types"
import { DetailHeader } from "@/components/detail/DetailHeader"
import { DetailAttributes } from "@/components/detail/DetailAttributes"
import { DetailNotes } from "@/components/detail/DetailNotes"
import { InterviewsCard } from "@/components/detail/InterviewsCard"
import { EventsHistoryCard } from "@/components/detail/EventsHistoryCard"
import { ArtifactsCard } from "@/components/detail/ArtifactsCard"
import { QuestionsCard } from "@/components/detail/QuestionsCard"
import { SectionError } from "@/components/detail/SectionError"
import { AttributesSkeleton, ArtifactsSkeleton, EventsSkeleton, HeaderSkeleton, QuestionsSkeleton } from "@/components/detail/SectionSkeletons"

export type DetailInitial = { application: Application; fields: CustomField[] }

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
  const questions = useSection<Page<Question>>(`/v1/applications/${encodeURIComponent(id)}/questions`, exportMode)

  async function reload() {
    const app = await api<Application>(`/v1/applications/${encodeURIComponent(id)}`)
    setApplication(app)
  }

  async function onSaveField(patch: Record<string, string>) {
    if (!application) return
    setFieldError(null)
    try {
      const updated = await api<Application>(`/v1/applications/${encodeURIComponent(application.id)}`, {
        method: "PATCH",
        body: { version: application.version, ...patch },
      })
      setApplication(updated)
    } catch (err) {
      setFieldError(err instanceof ApiError ? err.message : "Failed to save")
    }
  }

  async function onSaveCustomFieldValue(fieldID: string, value: string) {
    if (!application) return
    try {
      const updated = await api<Application>(
        `/v1/applications/${encodeURIComponent(application.id)}/fields/${encodeURIComponent(fieldID)}`,
        { method: "POST", body: { value } },
      )
      setApplication(updated)
    } catch (err) {
      setFieldError(err instanceof ApiError ? err.message : "Failed to save")
    }
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

  async function onSaveNotes(notes: string) {
    if (!application) return
    const updated = await api<Application>(`/v1/applications/${encodeURIComponent(application.id)}`, {
      method: "PATCH",
      body: { version: application.version, notes },
    })
    setApplication(updated)
  }

  async function onDelete() {
    if (!application) return
    await api(`/v1/applications/${encodeURIComponent(application.id)}`, { method: "DELETE" })
    router.push("/applications")
  }

  if (notFound) return <p className="text-sm text-destructive">That application doesn&apos;t exist.</p>

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-4">
      <Link href="/applications" aria-label="Back to pipeline" className="text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" />
      </Link>

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
            onArchive={onArchive}
            onUnarchive={onUnarchive}
            onDecide={onDecide}
            onDelete={onDelete}
          />
          <DetailAttributes
            application={application}
            fields={fields}
            writable={writable}
            onSaveField={onSaveField}
            onSaveCustomFieldValue={onSaveCustomFieldValue}
            onRecheckLink={onRecheckLink}
            fieldError={fieldError}
          />
          <InterviewsCard appId={application.id} current={application.stage ?? 0} writable={writable} onChange={reload} />
          <DetailNotes notes={application.notes || ""} writable={writable} onSave={onSaveNotes} />
        </>
      )}

      {exportMode ? (
        <>
          {questions.error ? (
            <SectionError label="prepared answers" onRetry={questions.retry} />
          ) : questions.data == null ? (
            <QuestionsSkeleton />
          ) : (
            <QuestionsCard questions={questions.data.data || []} />
          )}
          {artifacts.error ? (
            <SectionError label="artifacts" onRetry={artifacts.retry} />
          ) : artifacts.data == null ? (
            <ArtifactsSkeleton />
          ) : (
            <ArtifactsCard appId={id} artifacts={artifacts.data.data || []} />
          )}
          {events.error ? (
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
