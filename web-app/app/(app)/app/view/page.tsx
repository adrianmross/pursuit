"use client"

import { Suspense } from "react"
import { useSearchParams } from "next/navigation"
import { DetailShell } from "@/components/detail/DetailShell"

// Export-mode-only route — see lib/links.ts. Hosted builds never link
// here (they use the real /app/[id] dynamic route) but this still has to
// build successfully either way since it's one shared codebase.
function ViewById() {
  const id = useSearchParams().get("id") || ""
  if (!id) return <p className="text-sm text-destructive">No application id given.</p>
  return <DetailShell id={id} initial={null} />
}

export default function ApplicationViewPage() {
  return (
    <Suspense>
      <ViewById />
    </Suspense>
  )
}
