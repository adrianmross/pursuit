"use client"

import { useCallback, useEffect, useState } from "react"
import { api } from "@/lib/api-client"

// GET /v1/features. available = configured && entitled && enabled; the
// server enforces it, this only decides what to show.
export type Feature = {
  key: string
  name: string
  description: string
  premium: boolean
  experimental: boolean
  configured: boolean
  entitled: boolean
  enabled: boolean
  available: boolean
}

export type Billing = {
  enabled: boolean
  status: string
  active: boolean
  // Every feature without a plan: the owner, or PURSUIT_COMP_EMAILS.
  comped?: boolean
  current_period_end?: string
  cancel_at_period_end: boolean
}

export function useFeatures() {
  const [features, setFeatures] = useState<Feature[] | null>(null)
  const load = useCallback(() => {
    api<{ data: Feature[] }>("/v1/features")
      .then((r) => setFeatures(r.data))
      .catch(() => setFeatures([]))
  }, [])
  useEffect(load, [load])
  const set = async (key: string, enabled: boolean) => {
    const next = await api<Feature>(`/v1/features/${key}`, { method: "PATCH", body: { enabled } })
    setFeatures((fs) => fs?.map((f) => (f.key === key ? next : f)) ?? null)
  }
  return { features, set, reload: load }
}

// Send the browser to a Stripe page the server opens (checkout or portal).
// code is an invite's promotion code, applied at checkout.
export async function goToStripe(path: "/v1/billing/checkout" | "/v1/billing/portal", code?: string) {
  const { url } = await api<{ url: string }>(path, { method: "POST", body: code ? { code } : undefined })
  window.location.assign(url)
}
