"use client"

import { useEffect, useState } from "react"
import { apiHeaders, apiUrl } from "@/lib/api-client"
import { getPref } from "@/lib/settings"

// ponytail: in-memory only, no eviction — fine for a session's worth of companies.
// "default" sends nothing, so the server's default applies.
const SERVICE_PARAM: Record<string, string> = { on: "?service=1", off: "?service=0" }

const faviconCache = new Map<string, string | null>()

// Drop a company's cached icon after its icon_url changed; avatars mounted
// later (or remounted by a key change) refetch.
export function forgetFavicon(companyId: string) {
  faviconCache.delete(companyId)
}

function useFavicon(companyId?: string) {
  const [url, setUrl] = useState<string | null>(companyId ? faviconCache.get(companyId) ?? null : null)

  useEffect(() => {
    if (!companyId || faviconCache.has(companyId)) return
    let cancelled = false
    fetch(apiUrl(`/v1/companies/${encodeURIComponent(companyId)}/favicon${SERVICE_PARAM[getPref<string>("faviconService")] ?? ""}`), { headers: apiHeaders() })
      .then((res) => (res.ok ? res.blob() : Promise.reject()))
      .then((blob) => {
        const objectUrl = URL.createObjectURL(blob)
        faviconCache.set(companyId, objectUrl)
        if (!cancelled) setUrl(objectUrl)
      })
      .catch(() => {
        faviconCache.set(companyId, null)
      })
    return () => {
      cancelled = true
    }
  }, [companyId])

  return url
}

export function CompanyAvatar({
  company,
  companyId,
  className = "",
}: {
  company?: string
  companyId?: string
  className?: string
}) {
  const fetched = useFavicon(companyId)
  const [broken, setBroken] = useState<string | null>(null)
  const favicon = fetched && fetched !== broken ? fetched : null
  const name = company?.trim() || "?"

  if (favicon) {
    return (
      // eslint-disable-next-line @next/next/no-img-element -- fetched as an authenticated blob URL, next/image can't optimize it
      <img
        src={favicon}
        alt=""
        onError={() => setBroken(favicon)}
        className={`inline-flex size-6 shrink-0 rounded-full object-contain ${className}`}
      />
    )
  }

  return (
    <span
      className={`inline-flex size-6 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-medium text-foreground ${className}`}
      aria-hidden="true"
    >
      {name[0]!.toUpperCase()}
    </span>
  )
}
