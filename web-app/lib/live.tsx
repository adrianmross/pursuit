"use client"

// Keeps every open dashboard page current. One long poll on GET /v1/events
// (the same feed `pursuit events --follow` reads) bumps a revision that views
// add to their fetch effects; returning to the tab bumps it too, which covers
// changes that record no event (standing answers, documents). A bump waits
// while you are typing, so a refetch never lands on a half-made edit.

import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from "react"
import { api } from "@adrianmross/appkit-ui"
import { useAuth } from "@adrianmross/appkit-ui"
import type { Application, Event, Page } from "@/lib/types"

export type LiveEvent = Event & { app?: Pick<Application, "id" | "title" | "company" | "status"> }

type Live = {
  revision: number
  connected: boolean
  events: LiveEvent[]
  unread: number
  markRead: () => void
  // An event this browser's token caused: the person already saw it happen.
  mine: (ev: Event) => boolean
}

const LiveContext = createContext<Live>({ revision: 0, connected: false, events: [], unread: 0, markRead: () => {}, mine: () => false })

export const useLive = () => useContext(LiveContext)
export const useRevision = () => useContext(LiveContext).revision

const SEEN_KEY = "pursuit:notifications-seen"
const WAIT_MS = 25_000

function editing() {
  const el = document.activeElement
  return Boolean(el && el.matches("input:not([type=checkbox]):not([type=radio]), textarea, select, [contenteditable=true]"))
}

function readSeen() {
  try {
    return localStorage.getItem(SEEN_KEY) || ""
  } catch {
    return ""
  }
}

export function LiveProvider({ children, onEvent }: { children: ReactNode; onEvent?: (ev: LiveEvent) => void }) {
  const { me } = useAuth()
  const [revision, setRevision] = useState(0)
  const [connected, setConnected] = useState(false)
  const [events, setEvents] = useState<LiveEvent[]>([])
  // ponytail: read state lives in this browser's localStorage, not per account; move to the server if it must sync.
  const [seen, setSeen] = useState(() => (typeof window === "undefined" ? "" : readSeen()))
  const apps = useRef(new Map<string, LiveEvent["app"]>())
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent
  const myToken = me?.token?.id

  // One refetch at a time; while a field has focus it waits for the blur.
  const pending = useRef(false)
  function bump() {
    if (!editing()) {
      setRevision((r) => r + 1)
      return
    }
    if (pending.current) return
    pending.current = true
    document.addEventListener(
      "focusout",
      () => {
        pending.current = false
        setTimeout(bump, 0)
      },
      { once: true },
    )
  }

  async function withApps(list: Event[]): Promise<LiveEvent[]> {
    const missing = [...new Set(list.map((e) => e.application_id).filter((id): id is string => !!id && !apps.current.has(id)))]
    if (missing.length) {
      const res = await api<{ results: { status: number; body: Application }[] }>("/v1/batch", {
        method: "POST",
        body: { requests: missing.map((id) => ({ method: "GET", path: `/v1/applications/${encodeURIComponent(id)}?fields=id,title,company,status` })) },
      }).catch(() => null)
      res?.results.forEach((r, i) => apps.current.set(missing[i], r.status === 200 ? r.body : undefined))
    }
    return list.map((e) => ({ ...e, app: e.application_id ? apps.current.get(e.application_id) : undefined }))
  }

  useEffect(() => {
    let stopped = false
    let after = ""

    async function loop() {
      try {
        const first = await api<Page<Event>>("/v1/events?limit=30&sort=-occurred_at")
        const recent = await withApps(first.data || [])
        if (stopped) return
        setEvents(recent)
        after = recent[0]?.id || ""
        // First visit with notifications: what already happened isn't news.
        if (!readSeen() && after) {
          setSeen(after)
          try {
            localStorage.setItem(SEEN_KEY, after)
          } catch {
            // Private mode: the badge starts from zero on each visit.
          }
        }
        setConnected(true)
      } catch {
        setConnected(false)
      }
      while (!stopped) {
        if (document.hidden) {
          await new Promise<void>((resolve) => document.addEventListener("visibilitychange", () => resolve(), { once: true }))
          continue
        }
        try {
          const qs = new URLSearchParams({ wait_ms: String(WAIT_MS), limit: "50" })
          if (after) qs.set("after", after)
          const page = await api<Page<Event>>("/v1/events?" + qs, { timeoutMs: WAIT_MS + 10_000 })
          if (stopped) return
          setConnected(true)
          const fresh = (page.data || []).filter((e) => e.id && e.id !== after)
          if (!fresh.length) continue
          after = fresh[fresh.length - 1].id || after
          const enriched = await withApps(fresh)
          setEvents((prev) => [...enriched.reverse(), ...prev].slice(0, 100))
          enriched.forEach((e) => onEventRef.current?.(e))
          bump()
        } catch {
          if (stopped) return
          setConnected(false)
          // A server restart or a lost network: back off, then pick up where we left off.
          await new Promise((r) => setTimeout(r, 5_000))
        }
      }
    }
    void loop()

    // Back on the tab: catch what no event records.
    let last = Date.now()
    const back = () => {
      if (document.hidden || Date.now() - last < 5_000) return
      last = Date.now()
      bump()
    }
    document.addEventListener("visibilitychange", back)
    window.addEventListener("focus", back)
    return () => {
      stopped = true
      document.removeEventListener("visibilitychange", back)
      window.removeEventListener("focus", back)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- one loop per signed-in page
  }, [])

  const mine = (ev: Event) => Boolean(myToken && ev.actor?.token_id === myToken)
  const seenAt = events.find((e) => e.id === seen)?.occurred_at || ""
  const unread = events.filter((e) => !mine(e) && (!seen || (e.occurred_at || "") > seenAt) && e.id !== seen).length

  function markRead() {
    const top = events[0]?.id || ""
    setSeen(top)
    try {
      localStorage.setItem(SEEN_KEY, top)
    } catch {
      // Private mode: read state lasts for this page only.
    }
  }

  return <LiveContext.Provider value={{ revision, connected, events, unread, markRead, mine }}>{children}</LiveContext.Provider>
}
