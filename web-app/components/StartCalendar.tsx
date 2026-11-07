"use client"

import { useEffect, useMemo, useRef, useState } from "react"
import Link from "next/link"
import { ChevronLeft, ChevronRight, Maximize2, Minimize2 } from "lucide-react"
import { api } from "@/lib/api-client"
import { cn } from "@/lib/utils"
import type { Application, Interview, Page, Task } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import { Card, CardContent } from "@/components/ui/card"

type Kind = "round" | "task" | "applied"
type CalEvent = { day: string; kind: Kind; title: string; where: string; href: string; done: boolean }

// Each kind is its own colour, like calendars in Apple Calendar.
const DOT: Record<Kind, string> = { round: "bg-blue-500", task: "bg-amber-500", applied: "bg-emerald-500" }
const BAR: Record<Kind, string> = {
  round: "bg-blue-500/15 text-blue-800 dark:text-blue-200",
  task: "bg-amber-500/15 text-amber-800 dark:text-amber-200",
  applied: "bg-emerald-500/15 text-emerald-800 dark:text-emerald-200",
}
const KINDS: [Kind, string][] = [
  ["round", "Round"],
  ["task", "Task due"],
  ["applied", "Applied"],
]

const ymd = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`
const parseBody = <T,>(b: unknown) => (typeof b === "string" ? JSON.parse(b) : b) as T
const firstOf = (d: Date) => new Date(d.getFullYear(), d.getMonth(), 1)

// Rounds, due tasks and applied dates across applications, by day.
function useEvents() {
  const [events, setEvents] = useState<CalEvent[]>([])
  useEffect(() => {
    let live = true
    ;(async () => {
      const page = await api<Page<Application>>("/v1/applications?limit=200&fields=id,company,title,applied_date,interviews,tasks")
      const apps = page.data || []
      const out: CalEvent[] = apps
        .filter((a) => a.applied_date)
        .map((a) => ({ day: a.applied_date!.slice(0, 10), kind: "applied", title: a.company || a.title || "Applied", where: a.title || "", href: `/app/${a.id}`, done: false }))
      const any = (m?: Record<string, { total: number }>) => Object.values(m ?? {}).some((c) => c.total > 0)
      // ponytail: one batch of 100, so the 50 busiest applications; page the batch if that runs short.
      const busy = apps.filter((a) => any(a.interviews) || any(a.tasks)).slice(0, 50)
      const requests = busy.flatMap((a) => [
        { method: "GET", path: `/v1/applications/${encodeURIComponent(a.id)}/rounds` },
        { method: "GET", path: `/v1/applications/${encodeURIComponent(a.id)}/tasks` },
      ])
      if (requests.length) {
        const r = await api<{ results: { status: number; body: unknown }[] }>("/v1/batch", { method: "POST", body: { requests } })
        busy.forEach((a, i) => {
          const name = a.company || a.title || ""
          const href = `/app/${a.id}`
          const rounds = r.results[2 * i]?.status === 200 ? parseBody<Page<Interview>>(r.results[2 * i].body).data || [] : []
          const tasks = r.results[2 * i + 1]?.status === 200 ? parseBody<Page<Task>>(r.results[2 * i + 1].body).data || [] : []
          rounds.forEach((x, n) => x.scheduled_at && out.push({ day: x.scheduled_at.slice(0, 10), kind: "round", title: name, where: x.kind || `Round ${n + 1}`, href, done: !!x.done_at }))
          tasks.forEach((t) => t.due_at && !t.done_at && out.push({ day: t.due_at.slice(0, 10), kind: "task", title: t.title, where: name, href, done: false }))
        })
      }
      if (live) setEvents(out)
    })().catch(() => {})
    return () => {
      live = false
    }
  }, [])
  return events
}

// The week at a glance, like Apple Calendar: the dates, with a dot for each
// kind of thing on a day. Arrows or a sideways swipe move a week; Expand opens
// the month grid with each day's events as bars. Pick a day and what's on it
// drops down below, a row at a time.
export function StartCalendar() {
  const events = useEvents()
  const today = ymd(new Date())
  const [anchor, setAnchor] = useState(() => new Date())
  const [picked, setPicked] = useState<string | null>(today)
  const [expanded, setExpanded] = useState(false)
  const swipe = useRef(0)
  const byDay = useMemo(() => {
    const m = new Map<string, CalEvent[]>()
    events.forEach((e) => m.set(e.day, [...(m.get(e.day) ?? []), e]))
    return m
  }, [events])
  const month = firstOf(anchor)
  const start = expanded ? new Date(month.getFullYear(), month.getMonth(), 1 - month.getDay()) : new Date(anchor.getFullYear(), anchor.getMonth(), anchor.getDate() - anchor.getDay())
  const weeks = expanded ? Math.ceil((month.getDay() + new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate()) / 7) : 1
  const days = Array.from({ length: weeks * 7 }, (_, i) => new Date(start.getFullYear(), start.getMonth(), start.getDate() + i))
  // A week is labelled by its Wednesday, so a week across two months reads as the one it's mostly in.
  const label = expanded ? month : days[3]
  const shift = (n: number) => setAnchor(expanded ? new Date(anchor.getFullYear(), anchor.getMonth() + n, 1) : new Date(anchor.getFullYear(), anchor.getMonth(), anchor.getDate() + 7 * n))
  const onWheel = (e: React.WheelEvent) => {
    if (expanded || Math.abs(e.deltaX) <= Math.abs(e.deltaY)) return
    swipe.current += e.deltaX
    if (Math.abs(swipe.current) < 60) return
    shift(Math.sign(swipe.current))
    swipe.current = -Math.sign(swipe.current) * 400 // ponytail: crude debounce so one swipe moves one week
  }
  const weekdays = days.slice(0, 7).map((d) => d.toLocaleDateString(undefined, expanded ? { weekday: "short" } : { weekday: "narrow" }))
  const list = picked ? (byDay.get(picked) ?? []) : []
  const unit = expanded ? "month" : "week"

  return (
    <Card className={cn("gap-3", expanded ? "py-4" : "bg-transparent py-0 ring-0")} onWheel={onWheel}>
      <div className={cn("flex items-center gap-2", expanded ? "px-4" : "px-1.5")}>
        <h2 className="text-base">
          <span className="font-semibold">{label.toLocaleDateString(undefined, { month: "long" })}</span> <span className="text-muted-foreground">{label.getFullYear()}</span>
        </h2>
        <span className="flex-1" />
        <ButtonGroup>
          <Button variant="outline" size="icon-sm" aria-label={`Previous ${unit}`} onClick={() => shift(-1)}>
            <ChevronLeft />
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setAnchor(new Date())
              setPicked(today)
            }}
          >
            Today
          </Button>
          <Button variant="outline" size="icon-sm" aria-label={`Next ${unit}`} onClick={() => shift(1)}>
            <ChevronRight />
          </Button>
        </ButtonGroup>
        <Button variant="ghost" size="icon-sm" aria-label={expanded ? "Show the week" : "Show the month"} title={expanded ? "Show the week" : "Show the month"} onClick={() => setExpanded(!expanded)}>
          {expanded ? <Minimize2 /> : <Maximize2 />}
        </Button>
      </div>
      <CardContent className={cn("flex flex-col gap-3", expanded ? "px-3" : "px-0")}>
        <div className={cn("grid grid-cols-7 text-[11px] text-muted-foreground", expanded ? "text-right" : "text-center")}>
          {weekdays.map((w, i) => (
            <span key={i} className="px-1.5">
              {w}
            </span>
          ))}
        </div>
        <div className={cn("-mt-2 grid grid-cols-7", expanded && "overflow-hidden rounded-lg border")}>
          {days.map((d) => {
            const key = ymd(d)
            const items = byDay.get(key) ?? []
            const kinds = KINDS.filter(([k]) => items.some((e) => e.kind === k))
            const out = expanded && d.getMonth() !== month.getMonth()
            const pick = () => setPicked(key === picked ? null : key)
            const number = (
              <span
                className={cn(
                  "flex size-7 items-center justify-center rounded-full text-xs tabular-nums",
                  out && "text-muted-foreground/50",
                  key === picked && key !== today && "bg-foreground font-semibold text-background",
                  key === today && "bg-red-500 font-semibold text-white",
                )}
              >
                {d.getDate()}
              </span>
            )
            if (!expanded)
              return (
                <button key={key} type="button" onClick={pick} aria-label={d.toLocaleDateString(undefined, { dateStyle: "full" })} aria-pressed={key === picked} className="flex flex-col items-center gap-0.5 rounded-md py-1 outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring">
                  {number}
                  <span className="flex h-1.5 gap-0.5">
                    {kinds.map(([k]) => (
                      <span key={k} className={cn("size-1.5 rounded-full", DOT[k])} />
                    ))}
                  </span>
                </button>
              )
            return (
              <button key={key} type="button" onClick={pick} aria-pressed={key === picked} className={cn("flex min-h-24 min-w-0 flex-col items-stretch gap-0.5 border-b p-1 text-left outline-none not-[:nth-child(7n)]:border-r hover:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset", (d.getDay() === 0 || d.getDay() === 6) && "bg-muted/30")}>
                <span className="self-end">{number}</span>
                {items.slice(0, 3).map((e, i) => (
                  <span key={i} className={cn("truncate rounded-[4px] px-1.5 py-px text-[11px] leading-4 font-medium", BAR[e.kind], e.done && "line-through")}>
                    {e.title}
                  </span>
                ))}
                {items.length > 3 && <span className="px-1.5 text-[11px] text-muted-foreground">{items.length - 3} more</span>}
              </button>
            )
          })}
        </div>
        {/* The picked day drops down when there's something on it, each row fading in after the last. Keyed by day so a new pick replays it. */}
        {list.length > 0 && (
          <div key={picked} className="-mt-1 flex flex-col gap-0.5">
            {list.map((e, i) => (
              <Link key={i} href={e.href} style={{ animationDelay: `${i * 60}ms` }} className="flex items-center gap-2.5 rounded-md px-1.5 py-1 text-sm duration-300 animate-in fill-mode-both fade-in slide-in-from-top-1 hover:bg-muted">
                <span className={cn("h-4 w-1 shrink-0 rounded-full", DOT[e.kind])} />
                <span className={cn("min-w-0 flex-1 truncate", e.done && "text-muted-foreground line-through")}>
                  {e.title}
                  {e.where && <span className="text-muted-foreground"> · {e.where}</span>}
                </span>
                <span className="shrink-0 text-xs text-muted-foreground">{KINDS.find(([k]) => k === e.kind)![1]}</span>
              </Link>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
