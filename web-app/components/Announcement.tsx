"use client"

import { useState } from "react"
import { ArrowRight, Megaphone, X } from "lucide-react"
import { api } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import announcements from "@/content/announcements.json"
import releases from "@/lib/releases.generated.json"
import { Button } from "@/components/ui/button"
import { Item, ItemActions, ItemContent, ItemDescription, ItemMedia, ItemTitle } from "@/components/ui/item"

type Entry = { id: string; at: string; title: string; body?: string; href?: string }

// Minor releases (generated from CHANGELOG.md) and hand-written announcements
// share one feed; only the newest shows, and dismissing it records its time on
// the account, which also covers everything older.
const latest = ([...releases, ...announcements] as Entry[]).sort((a, b) => b.at.localeCompare(a.at))[0]

export function Announcement() {
  const { me, refresh } = useAuth()
  const [gone, setGone] = useState(false)
  // Without a signed-in user there is nowhere to remember the dismissal.
  if (!latest || gone || !me?.user || (me.user.announcements_seen ?? "") >= latest.at) return null

  const dismiss = () => {
    setGone(true)
    api("/v1/me", { method: "PATCH", body: { announcements_seen: latest.at } }).then(refresh, () => setGone(false))
  }
  return (
    <Item variant="outline">
      <ItemMedia variant="icon">
        <Megaphone />
      </ItemMedia>
      <ItemContent>
        <ItemTitle>{latest.title}</ItemTitle>
        {latest.body && <ItemDescription>{latest.body}</ItemDescription>}
      </ItemContent>
      <ItemActions>
        {latest.href && (
          <Button variant="outline" size="sm" asChild>
            <a href={latest.href} target="_blank" rel="noopener noreferrer">
              What&apos;s new <ArrowRight />
            </a>
          </Button>
        )}
        <Button variant="ghost" size="icon-sm" aria-label="Dismiss" onClick={dismiss}>
          <X />
        </Button>
      </ItemActions>
    </Item>
  )
}
