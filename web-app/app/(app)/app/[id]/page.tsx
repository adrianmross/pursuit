import { Suspense } from "react"
import { notFound } from "next/navigation"
import { EXPORT_MODE, pursuitApiAuthed, ApiError } from "@/lib/pursuit-api"
import type { Application, CustomField, Page } from "@/lib/types"
import { CreateApplicationView } from "@/components/detail/CreateApplicationView"
import { DetailShell } from "@/components/detail/DetailShell"
import { EventsSection } from "@/components/detail/EventsSection"
import { ArtifactsSection } from "@/components/detail/ArtifactsSection"
import { QuestionsSection } from "@/components/detail/QuestionsSection"
import { EventsSkeleton, ArtifactsSkeleton, QuestionsSkeleton } from "@/components/detail/SectionSkeletons"

export default async function ApplicationDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params

  if (id === "new") return <CreateApplicationView />

  if (EXPORT_MODE) return <DetailShell id={id} initial={null} />

  let application: Application
  let fields: CustomField[]
  try {
    const [app, fieldsPage] = await Promise.all([
      pursuitApiAuthed<Application>(`/v1/applications/${encodeURIComponent(id)}`),
      pursuitApiAuthed<Page<CustomField>>("/v1/fields?limit=100"),
    ])
    application = app
    fields = fieldsPage.data || []
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) notFound()
    throw err
  }

  return (
    <DetailShell id={id} initial={{ application, fields }}>
      <Suspense fallback={<QuestionsSkeleton />}>
        <QuestionsSection id={id} />
      </Suspense>
      <Suspense fallback={<ArtifactsSkeleton />}>
        <ArtifactsSection id={id} />
      </Suspense>
      <Suspense fallback={<EventsSkeleton />}>
        <EventsSection id={id} />
      </Suspense>
    </DetailShell>
  )
}
