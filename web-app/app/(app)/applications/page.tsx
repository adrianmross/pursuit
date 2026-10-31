import { Suspense } from "react"
import { EXPORT_MODE, pursuitApiAuthed } from "@/lib/pursuit-api"
import { buildApplicationsQuery } from "@/lib/pipeline-query"
import { getDefaultSort, getDefaultStatus } from "@/lib/settings"
import type { Application, CustomField, Page } from "@/lib/types"
import { PipelineView, type PipelineInitial } from "@/components/PipelineView"

export default async function PipelinePage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>
}) {
  if (EXPORT_MODE) {
    // No server at request time — PipelineView fetches client-side, same
    // as the old Vite app. useSearchParams() needs a Suspense boundary to
    // statically prerender.
    return (
      <Suspense>
        <PipelineView initial={null} />
      </Suspense>
    )
  }

  const sp = await searchParams
  const one = (v: string | string[] | undefined) => (Array.isArray(v) ? v[0] : v)
  const status = one(sp.status) ?? getDefaultStatus()
  const category = one(sp.category) || ""
  const sort = one(sp.sort) || getDefaultSort()
  const archivedView = one(sp.archived) === "1"
  const exclude = (one(sp.exclude) || "").split(",").filter(Boolean)

  const qs = buildApplicationsQuery({ status, category, sort, archivedView, exclude })
  const [appsPage, fieldsPage] = await Promise.all([
    pursuitApiAuthed<Page<Application>>("/v1/applications?" + qs.toString()),
    pursuitApiAuthed<Page<CustomField>>("/v1/fields?limit=100"),
  ])

  const initial: PipelineInitial = {
    rows: appsPage.data,
    total: appsPage.page.total,
    nextCursor: appsPage.page.next_cursor || "",
    fields: fieldsPage.data || [],
  }

  return (
    <Suspense>
      <PipelineView initial={initial} />
    </Suspense>
  )
}
