"use client"

import { useEffect, useState } from "react"
import { api } from "@/lib/api-client"
import type { Application, Page } from "@/lib/types"
import { HomeCta } from "@/components/HomeCta"

// Export-mode only: there's no server to resolve this ahead of render, so
// it's fetched client-side, same as the old Vite app did.
export function HomeCtaClient() {
  const [hasApplications, setHasApplications] = useState(false)

  useEffect(() => {
    let cancelled = false
    api<Page<Application>>("/v1/applications?limit=1")
      .then((page) => {
        if (!cancelled) setHasApplications(page.page.total > 0)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [])

  return <HomeCta hasApplications={hasApplications} />
}
