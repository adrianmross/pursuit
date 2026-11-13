"use client"

import { useEffect, useRef, useState } from "react"
import Link from "next/link"
import { usePathname, useRouter } from "next/navigation"
import { Archive, ArrowLeft, Bot, IdCard, ListChecks, MessageSquareText, Plus } from "lucide-react"
import {
  api,
  AppShell as Shell,
  CaughtUp,
  SearchBox,
  useAuth,
  useLive,
  useShell,
  type Event,
  type EventText,
  type LiveEvent,
  type PaletteGroup,
  type SearchSource,
  type ShellProps,
} from "@adrianmross/appkit-ui"
import { Button } from "@adrianmross/appkit-ui/ui/button"
import { Die, Sketch, Track } from "@/components/Sketch"
import { TrailArt } from "@/components/TourArt"
import { SECTIONS } from "@/components/settings/sections"
import { applicationHref } from "@/lib/links"
import { QUALIFIERS } from "@/lib/search"
import { label, statusesInCategory, type Application, type Page } from "@/lib/types"

// Every signed-in page but the table goes back to it.
// ponytail: one parent for all; map pathname -> parent once pages nest.
const PARENT = "/applications"
// The page shown before this one, if it was inside the app.
let lastPath = ""

// ponytail: appkit-ui does not export TourStep yet; take it from the shell's props until it does.
type TourStep = Extract<NonNullable<ShellProps["tour"]>["steps"], unknown[]>[number]

// A feed event as Pursuit's API sends it, plus the application the shell looked up for it.
type AppEvent = LiveEvent & {
  application_id?: string
  to_status?: string
  app?: Pick<Application, "id" | "title" | "company" | "status">
}

// One line a person reads: what happened, to which role. Offers are celebrated plainly.
const EVENTS: EventText = {
  describe(e) {
    const ev = e as AppEvent
    const role = ev.app?.title || "An application"
    if (ev.to_status === "offer") return { text: `Offer from ${ev.app?.company || "them"}. Nice.`, win: true }
    if (ev.title === "Application created" || ev.kind === "imported") return { text: `Saved ${role}.`, win: false }
    if (ev.kind === "status_change" && ev.to_status) return { text: `${role} moved to ${label(ev.to_status)}.`, win: false }
    return { text: ev.title ? `${role}: ${ev.title}` : role, win: false }
  },
  href: (e) => ((e as AppEvent).application_id ? applicationHref((e as AppEvent).application_id!) : undefined),
  meta: (e) => [(e as AppEvent).app?.company, e.actor?.name, ago(e.occurred_at)].filter(Boolean).join(" · "),
}

const rtf = new Intl.RelativeTimeFormat("en", { numeric: "auto", style: "short" })

function ago(iso?: string) {
  if (!iso) return ""
  const s = (new Date(iso).getTime() - Date.now()) / 1000
  if (s > -45) return "just now"
  for (const [unit, n] of [["minute", 60], ["hour", 3600], ["day", 86400]] as const) {
    if (-s < n * (unit === "day" ? 7 : unit === "hour" ? 24 : 60)) return rtf.format(Math.round(s / n), unit)
  }
  return new Date(iso).toLocaleDateString()
}

const emptyArt = (
  <Sketch viewBox="0 0 200 40" roughness={2} className="w-40 text-foreground">
    <g transform="translate(-2 -152)">
      <Track y={164} count={7} />
    </g>
    <Die x={174} y={22} size={24} tilt={14} pips={1} />
  </Sketch>
)

// The bell's first tab: applications waiting on the person.
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
  if (!rows.length) return <CaughtUp art={emptyArt}>Nothing is waiting on you.</CaughtUp>
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

const SEARCH: SearchSource<Application> = {
  path: PARENT,
  label: "Applications",
  qualifiers: QUALIFIERS,
  load: () => api<Page<Application>>("/v1/applications?limit=200&sort=-updated_at").then((p) => p.data || []),
  text: (a) => [a.company, a.title, label(a.status)].filter(Boolean).join(" "),
  item: (a) => ({ id: a.id, text: a.title || "Untitled", hint: a.company, href: applicationHref(a.id) }),
}

// A step points at [data-tour=target] on a page matching `match`. An empty
// account has no table or application to point at, so those steps go.
async function tourSteps(isDemo: boolean): Promise<TourStep[]> {
  const first = await api<Page<Application>>("/v1/applications?limit=1").then((p) => p.data?.[0], () => undefined)
  const steps: (TourStep & { needsApp?: boolean })[] = [
    { target: "get-started", route: "/start", match: /^\/start$/, title: "Your feed", body: "Prompts for your profile, an agent, the terminal and the browser extension. Dismiss what you don't need." },
    { target: "table", route: "/applications", match: /^\/applications$/, title: "Your applications", body: "Every posting you track, by stage. Search with the box above, or press ⌘P for the command palette.", hint: "Open any application to continue.", needsApp: true, advanceOn: /^\/app\// },
    { target: "details", route: first ? applicationHref(first.id) : "", match: /^\/app\//, title: "One application", body: "Details holds the listing, then tabs for rounds, tasks, questions and notes. Each round's notes are one markdown document: a heading ending in ? is a question.", hint: "Try the tabs, it's all live.", needsApp: true },
    { target: "questions", route: "/profile/questions", match: /^\/profile\/questions$/, title: "Your answers", body: "Answer a question once and it fills in on every form that asks. Interview questions from your notes land here too, so a good answer is one click from reuse." },
    { target: "account", route: "/profile/questions", match: /^\/profile\/questions$/, title: "That's the tour", body: "Replay it any time from Settings › About." + (isDemo ? " Your changes here are a throwaway sandbox." : "") },
  ]
  return steps.filter((s) => first || !s.needsApp).map(({ needsApp: _, ...step }) => step)
}

// Things that open Settings from the address: a return from Stripe Checkout
// and an invite link. They need the shell, so they render under it.
function AddressActions() {
  const router = useRouter()
  const { openSettings } = useShell()
  // Back from Stripe Checkout: read the plan now rather than wait for the
  // webhook, then drop the marker from the address.
  useEffect(() => {
    const url = new URL(window.location.href)
    if (!url.searchParams.has("billing")) return
    const done = () => {
      url.searchParams.delete("billing")
      router.replace(url.pathname + url.search)
    }
    if (url.searchParams.get("billing") === "success") api("/v1/billing/sync", { method: "POST" }).catch(() => {}).finally(done)
    else done()
  }, [router])
  // An invite link (?invite=CODE) opens the plan, which offers to accept it.
  useEffect(() => {
    if (new URLSearchParams(window.location.search).has("invite")) openSettings("plan")
    // eslint-disable-next-line react-hooks/exhaustive-deps -- once, on arrival
  }, [])
  return null
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const { isDemo } = useAuth()
  const router = useRouter()
  const pathname = usePathname()
  useEffect(() => () => void (lastPath = pathname), [pathname])
  // History keeps the table's filters and scroll, but only when the table is
  // what history would return to; otherwise open it fresh.
  // ponytail: lastPath is wrong after a browser Forward; history.state indexes would fix it.
  const goBack = () => (lastPath === PARENT ? router.back() : router.push(PARENT))

  // The palette lists recent applications, fetched each time it opens.
  const [apps, setApps] = useState<Application[]>([])
  const loadApps = () =>
    api<Page<Application>>("/v1/applications?limit=50&sort=-updated_at")
      .then((page) => setApps(page.data || []))
      .catch(() => setApps([]))
  async function goThroughPending() {
    const qs = statusesInCategory("action")
      .map((st) => "status=" + encodeURIComponent(st))
      .join("&")
    const page = await api<Page<Application>>(`/v1/applications?limit=1&sort=updated_at&${qs}`)
    const next = page.data[0]
    if (next) router.push(applicationHref(next.id))
  }
  const palette: PaletteGroup[] = [
    {
      heading: "Actions",
      items: [
        { label: "Profile", icon: <IdCard />, onSelect: () => router.push("/profile") },
        { label: "Questions", value: "Questions standing answers", icon: <MessageSquareText />, onSelect: () => router.push("/profile/questions") },
        { label: "Connect an agent", icon: <Bot />, onSelect: () => router.push("/agents") },
        { label: "Add application", icon: <Plus />, onSelect: () => router.push("/app/new") },
        { label: "View archived", icon: <Archive />, onSelect: () => router.push("/applications?archived=1") },
        { label: "Go through pending", icon: <ListChecks />, onSelect: () => void goThroughPending() },
      ],
    },
    {
      heading: "Applications",
      items: apps.map((app) => ({
        id: app.id,
        value: `${app.company || ""} ${app.title || ""} ${label(app.status)}`,
        label: (
          <span>
            {app.title || "Untitled"}
            <span className="text-muted-foreground"> — {app.company || "Unknown company"}</span>
          </span>
        ),
        onSelect: () => router.push(applicationHref(app.id)),
      })),
    },
  ]

  // What the feed shows about each event's application, looked up once per application.
  const known = useRef(new Map<string, AppEvent["app"]>())
  async function enrich(list: Event[]): Promise<LiveEvent[]> {
    const events = list as AppEvent[]
    const missing = [...new Set(events.map((e) => e.application_id).filter((id): id is string => !!id && !known.current.has(id)))]
    if (missing.length) {
      const res = await api<{ results: { status: number; body: Application }[] }>("/v1/batch", {
        method: "POST",
        body: { requests: missing.map((id) => ({ method: "GET", path: `/v1/applications/${encodeURIComponent(id)}?fields=id,title,company,status` })) },
      }).catch(() => null)
      res?.results.forEach((r, i) => known.current.set(missing[i], r.status === 200 ? r.body : undefined))
    }
    return events.map((e) => ({ ...e, app: e.application_id ? known.current.get(e.application_id) : undefined }))
  }

  return (
    <Shell
      // The mark goes to the start page, or from the start page to the table, like the link beside it.
      home={(path) => (path === "/start" ? { href: PARENT, label: "Applications" } : { href: "/start", label: "Start" })}
      nav={
        pathname === PARENT ? (
          // The table's own way back, so the logo isn't the only road to the start page.
          <Button variant="ghost" size="sm" className="shrink-0 text-muted-foreground" asChild>
            <Link href="/start" aria-label="Back to start">
              <ArrowLeft />
              <span className="hidden md:inline">Start</span>
            </Link>
          </Button>
        ) : (
          <Button variant="ghost" size="sm" className="shrink-0 text-muted-foreground" asChild>
            <Link
              href={PARENT}
              aria-label="Back to applications"
              onClick={(e) => {
                if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return
                e.preventDefault()
                goBack()
              }}
            >
              <ArrowLeft />
              <span className="hidden md:inline">Applications</span>
            </Link>
          </Button>
        )
      }
      // Short only on a medium screen; a phone's opened field has the room.
      search={({ compact, onBlurEmpty }) => (
        <SearchBox source={SEARCH} placeholder={compact && !onBlurEmpty ? "Search" : "Search applications"} onBlurEmpty={onBlurEmpty} />
      )}
      menu={[
        { label: "View archived", icon: <Archive />, onSelect: () => router.push("/applications?archived=1") },
        { label: "Profile", icon: <IdCard />, onSelect: () => router.push("/profile") },
        { label: "Questions", icon: <MessageSquareText />, onSelect: () => router.push("/profile/questions") },
        { label: "Connect an agent", icon: <Bot />, onSelect: () => router.push("/agents") },
      ]}
      sections={SECTIONS}
      palette={palette}
      onPaletteOpen={loadApps}
      palettePlaceholder="Jump to an application or run a command…"
      events={EVENTS}
      notificationsFirst={{ label: "Your move", render: (open) => <YourMove open={open} /> }}
      notificationsEmpty={<CaughtUp art={emptyArt}>No news yet.</CaughtUp>}
      enrich={enrich}
      shortcuts={({ focusSearch }) => ({
        // Not while typing in some other field; the search box itself is fine.
        searchApps: () =>
          document.activeElement?.matches("input:not([data-search]), textarea, select, [contenteditable='true']") ? false : focusSearch("global"),
        // Only on an application page; anywhere else Esc is left alone.
        back: () => (window.location.pathname.startsWith("/app/") ? goBack() : false),
        home: () => router.push("/start"),
        pipeline: () => router.push("/applications"),
        archived: () => router.push("/applications?archived=1"),
        new: () => router.push("/app/new"),
      })}
      tour={{
        steps: () => tourSteps(isDemo),
        welcome: {
          body: "Track every application from posting to offer. Take a two-minute tour of how it works, or jump straight in.",
          art: <TrailArt />,
        },
      }}
    >
      <AddressActions />
      {children}
    </Shell>
  )
}
