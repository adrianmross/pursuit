"use client"

import { useEffect, useState } from "react"
import { apiHeaders, apiUrl } from "@/lib/api-client"
import { usePref } from "@/lib/use-pref"

// ponytail: in-memory only, no eviction — fine for a session's worth of companies.
// "default" sends nothing, so the server's default applies.
const SERVICE_PARAM: Record<string, string> = { on: "?service=1", off: "?service=0" }

const faviconCache = new Map<string, string | null>()

// Drop a company's cached icon after its icon_url changed; avatars mounted
// later (or remounted by a key change) refetch.
export function forgetFavicon(companyId: string) {
  for (const k of [...faviconCache.keys()]) if (k.startsWith(`${companyId}:`)) faviconCache.delete(k)
}

function useFavicon(companyId?: string) {
  // The service is part of the key, so changing it in Settings refetches.
  const service = usePref<string>("faviconService")
  const key = companyId ? `${companyId}:${service}` : ""
  const [got, setGot] = useState<{ key: string; url: string | null }>({ key, url: key ? faviconCache.get(key) ?? null : null })
  const url = got.key === key ? got.url : key ? faviconCache.get(key) ?? null : null
  const setUrl = (u: string | null) => setGot({ key, url: u })

  useEffect(() => {
    if (!companyId || faviconCache.has(key)) return
    let cancelled = false
    fetch(apiUrl(`/v1/companies/${encodeURIComponent(companyId)}/favicon${SERVICE_PARAM[service] ?? ""}`), { headers: apiHeaders() })
      .then((res) => (res.ok ? res.blob() : Promise.reject()))
      .then((blob) => {
        const objectUrl = URL.createObjectURL(blob)
        faviconCache.set(key, objectUrl)
        if (!cancelled) setUrl(objectUrl)
      })
      .catch(() => {
        faviconCache.set(key, null)
      })
    return () => {
      cancelled = true
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])

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
