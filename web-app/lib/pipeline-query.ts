import { statusesInCategory, type StatusCategory } from "@/lib/types"

export type PipelineFilters = {
  status: string
  category: string
  sort: string
  archivedView: boolean
  exclude: string[]
  cursor?: string
}

// Shared between the server-side initial fetch (app/(app)/pipeline/page.tsx)
// and the client-side refetch on filter/sort change (PipelineView) — same
// query for the same filters, so hydration matches what the server sent.
export function buildApplicationsQuery(filters: PipelineFilters): URLSearchParams {
  const qs = new URLSearchParams()
  qs.set("limit", "50")
  if (filters.sort && filters.sort !== "category") qs.set("sort", filters.sort)
  if (filters.archivedView) qs.set("archived", "true")
  else if (filters.category) for (const st of statusesInCategory(filters.category as StatusCategory)) qs.append("status", st)
  else if (filters.status) qs.set("status", filters.status)
  for (const st of filters.exclude) qs.append("status_not", st)
  if (filters.cursor) qs.set("cursor", filters.cursor)
  return qs
}
