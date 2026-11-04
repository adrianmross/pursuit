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

// One /v1/features read per page load, shared by every caller of useFeature.
let featuresOnce: Promise<Feature[]> | null = null

export function useFeature(key: string) {
  const [on, setOn] = useState(false)
  useEffect(() => {
    featuresOnce ??= api<{ data: Feature[] }>("/v1/features").then(
      (r) => r.data,
      () => [],
    )
    let live = true
    void featuresOnce.then((fs) => live && setOn(!!fs.find((f) => f.key === key)?.available))
    return () => {
      live = false
    }
  }, [key])
  return on
}

// GET /v1/answers/similar: saved answers, round notes and documents close in meaning.
export type Match = {
  source: "answer" | "round" | "document"
  source_id: string
  title: string
  snippet: string
  score: number
  link?: string
  answer?: string
}

// Smart-search matches for q, 300ms after typing stops, from 3 characters.
// Empty when smart search isn't on for this user.
export function useSimilar(q: string, limit = 5) {
  const on = useFeature("smart_search")
  const [matches, setMatches] = useState<Match[]>([])
  const text = q.trim()
  const ready = on && text.length >= 3
  useEffect(() => {
    if (!ready) return
    let live = true
    const t = setTimeout(() => {
      api<{ data: Match[] }>(`/v1/answers/similar?q=${encodeURIComponent(text)}&limit=${limit}`)
        .then((r) => live && setMatches(r.data))
        .catch(() => live && setMatches([]))
    }, 300)
    return () => {
      live = false
      clearTimeout(t)
    }
  }, [ready, text, limit])
  return ready ? matches : []
}

// Send the browser to a Stripe page the server opens (checkout or portal).
export async function goToStripe(path: "/v1/billing/checkout" | "/v1/billing/portal") {
  const { url } = await api<{ url: string }>(path, { method: "POST" })
  window.location.assign(url)
}
