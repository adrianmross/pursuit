"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { Bell } from "lucide-react"
import { api } from "@adrianmross/appkit-ui"
import { applicationHref } from "@/lib/links"
import { useLive, type LiveEvent } from "@/lib/live"
import { label, type Application, type Page } from "@/lib/types"
import { cn } from "@adrianmross/appkit-ui"
import { Die, Sketch, Track } from "@/components/Sketch"
import { Button } from "@adrianmross/appkit-ui/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@adrianmross/appkit-ui/ui/popover"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@adrianmross/appkit-ui/ui/tabs"

const rtf = new Intl.RelativeTimeFormat("en", { numeric: "auto", style: "short" })

export function ago(iso?: string) {
  if (!iso) return ""
  const s = (new Date(iso).getTime() - Date.now()) / 1000
  if (s > -45) return "just now"
  for (const [unit, n] of [["minute", 60], ["hour", 3600], ["day", 86400]] as const) {
    if (-s < n * (unit === "day" ? 7 : unit === "hour" ? 24 : 60)) return rtf.format(Math.round(s / n), unit)
  }
  return new Date(iso).toLocaleDateString()
}

// One line a person reads: what happened, to which role. Offers are celebrated plainly.
export function describe(ev: LiveEvent) {
  const role = ev.app?.title || "An application"
  if (ev.to_status === "offer") return { text: `Offer from ${ev.app?.company || "them"}. Nice.`, win: true }
  if (ev.title === "Application created" || ev.kind === "imported") return { text: `Saved ${role}.`, win: false }
  if (ev.kind === "status_change" && ev.to_status) return { text: `${role} moved to ${label(ev.to_status)}.`, win: false }
  return { text: ev.title ? `${role}: ${ev.title}` : role, win: false }
}

export function eventMeta(ev: LiveEvent) {
  return [ev.app?.company, ev.actor?.name, ago(ev.occurred_at)].filter(Boolean).join(" · ")
}

function CaughtUp({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-2 px-6 py-8 text-center">
      <Sketch viewBox="0 0 200 40" roughness={2} className="w-40 text-foreground">
        <g transform="translate(-2 -152)">
          <Track y={164} count={7} />
        </g>
        <Die x={174} y={22} size={24} tilt={14} pips={1} />
      </Sketch>
      <p className="font-heading text-base font-semibold">{children}</p>
    </div>
  )
}

function YourMove({ open }: { open: boolean }) {
  const { revision } = useLive()
  const [rows, setRows] = useState<Application[] | null>(null)
  useEffect(() => {
    if (!open) return
    void api<Page<Application>>("/v1/applications?status=awaiting_review,needs_input&limit=20&sort=-updated_at")
      .then((p) => setRows(p.data || []))
      .catch(() => setRows([]))
  }, [open, revision])
  if (rows === null) return <p className="px-4 py-6 text-sm text-muted-foreground">Loading…</p>
  if (!rows.length) return <CaughtUp>Nothing is waiting on you.</CaughtUp>
  return (
    <ul className="divide-y">
      {rows.map((app) => (
        <li key={app.id} className="flex items-start gap-3 px-4 py-3">
          <span className="mt-1.5 size-2 shrink-0 rounded-full bg-foreground" aria-hidden />
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <p className="text-sm">
              <b className="font-semibold">{app.title}</b>{" "}
              {app.status === "needs_input" ? "needs an answer from you." : "is prepared and waiting for you."}
            </p>
            <p className="text-xs text-muted-foreground">
              {app.company} · {label(app.status)}
            </p>
            <Button size="sm" className="mt-1 self-start" asChild>
              <Link href={applicationHref(app.id)}>{app.status === "needs_input" ? "Answer" : "Review"}</Link>
            </Button>
          </div>
        </li>
      ))}
    </ul>
  )
}

function Everything() {
  const { events, mine } = useLive()
  const others = events.filter((e) => !mine(e))
  if (!others.length) return <CaughtUp>No news yet.</CaughtUp>
  return (
    <ul className="max-h-96 divide-y overflow-y-auto">
      {others.map((ev) => {
        const { text, win } = describe(ev)
        return (
          <li key={ev.id} className="px-4 py-3">
            <Link href={ev.application_id ? applicationHref(ev.application_id) : "#"} className="flex items-start gap-3 rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring">
              <span className={cn("mt-1.5 size-2 shrink-0 rounded-full", win ? "bg-brand-gold ring-1 ring-gold-ink" : "bg-muted-foreground/40")} aria-hidden />
              <span className="flex min-w-0 flex-1 flex-col gap-1">
                <span className="text-sm">{text}</span>
                <span className={cn("text-xs", win ? "text-gold-ink" : "text-muted-foreground")}>{eventMeta(ev)}</span>
              </span>
            </Link>
          </li>
        )
      })}
    </ul>
  )
}

export function NotificationBell() {
  const { unread, markRead, connected } = useLive()
  const [open, setOpen] = useState(false)
  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) markRead()
      }}
    >
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" className="relative" aria-label={unread ? `Notifications, ${unread} new` : "Notifications"}>
          <Bell />
          {unread > 0 && (
            <span className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-foreground px-1 text-[10px] font-semibold text-background">
              {unread > 9 ? "9+" : unread}
            </span>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-96 p-0">
        <div className="flex items-center gap-2 border-b px-4 py-3">
          <span className="flex-1 font-heading text-base font-semibold">Notifications</span>
          <span className="flex items-center gap-1.5 text-xs text-muted-foreground" title={connected ? "Updates arrive as they happen" : "Reconnecting"}>
            <span className={cn("size-1.5 rounded-full", connected ? "bg-foreground" : "bg-muted-foreground/40")} aria-hidden />
            {connected ? "Live" : "Offline"}
          </span>
          <Button variant="link" size="sm" className="h-auto px-0" onClick={markRead}>
            Mark all read
          </Button>
        </div>
        <Tabs defaultValue="move" className="gap-0">
          <TabsList className="mx-4 mt-3">
            <TabsTrigger value="move">Your move</TabsTrigger>
            <TabsTrigger value="all">Everything</TabsTrigger>
          </TabsList>
          <TabsContent value="move">
            <YourMove open={open} />
          </TabsContent>
          <TabsContent value="all">
            <Everything />
          </TabsContent>
        </Tabs>
      </PopoverContent>
    </Popover>
  )
}
