import { useEffect, useState } from "react"
import { getToken } from "@/lib/api"

// ponytail: in-memory only, no eviction — fine for a session's worth of companies.
const faviconCache = new Map<string, string | null>()

function useFavicon(companyId?: string) {
  const [url, setUrl] = useState<string | null>(companyId ? faviconCache.get(companyId) ?? null : null)

  useEffect(() => {
    if (!companyId || faviconCache.has(companyId)) return
    let cancelled = false
    fetch(`/v1/companies/${encodeURIComponent(companyId)}/favicon`, {
      headers: { Authorization: "Bearer " + getToken() },
    })
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
  const favicon = useFavicon(companyId)
  const name = company?.trim() || "?"

  if (favicon) {
    return (
      <img
        src={favicon}
        alt=""
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
