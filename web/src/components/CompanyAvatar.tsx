import { useEffect, useState } from "react"
import { getToken } from "@/lib/api"

const COLORS = [
  "bg-rose-500",
  "bg-orange-500",
  "bg-amber-500",
  "bg-lime-500",
  "bg-emerald-500",
  "bg-teal-500",
  "bg-cyan-500",
  "bg-blue-500",
  "bg-violet-500",
  "bg-fuchsia-500",
]

function hash(value: string) {
  let h = 0
  for (let i = 0; i < value.length; i++) h = (h * 31 + value.charCodeAt(i)) >>> 0
  return h
}

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

  const color = COLORS[hash(name) % COLORS.length]
  return (
    <span
      className={`inline-flex size-6 shrink-0 items-center justify-center rounded-full text-[11px] font-medium text-white ${color} ${className}`}
      aria-hidden="true"
    >
      {name[0]!.toUpperCase()}
    </span>
  )
}
