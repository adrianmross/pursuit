"use client"

import { useState } from "react"
import { api } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import authored from "@/content/announcements.json"
import generated from "@/lib/releases.generated.json"

// `release` marks an entry as the announcement for that version, replacing the
// one generated from CHANGELOG.md. `href` defaults to the changelog.
type Entry = { id: string; at: string; title: string; body?: string; href?: string; release?: string }

const written = authored as Entry[]
const replaced = new Set(written.map((e) => e.release))
const latest = ([...(generated as Entry[]).filter((r) => !replaced.has(r.id.slice(1))), ...written]).sort((a, b) => b.at.localeCompare(a.at))[0]

// The newest announcement while the signed-in account hasn't dismissed it.
// Dismissing saves its time on the account (PATCH /v1/me), which also covers
// everything older, on every device.
export function useAnnouncement() {
  const { me, refresh } = useAuth()
  const [gone, setGone] = useState(false)
  const due = latest && !gone && me?.user && (me.user.announcements_seen ?? "") < latest.at
  return {
    item: due ? { ...latest, href: latest.href ?? "/docs/changelog" } : null,
    dismiss: () => {
      setGone(true)
      api("/v1/me", { method: "PATCH", body: { announcements_seen: latest.at } }).then(refresh, () => setGone(false))
    },
  }
}
