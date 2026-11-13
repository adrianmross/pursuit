"use client"

import { useCallback, useEffect, useState } from "react"
import Link from "next/link"
import { ArrowLeft, ArrowRight, Bell, Check, ExternalLink, Search, X } from "lucide-react"
import { PRODUCT_NAME } from "@/lib/brand"
import { markOf, type Application } from "@/lib/types"
import { CompanyAvatar } from "@/components/CompanyAvatar"
import { ProgressMark } from "@/components/ProgressMark"
import { SiteFooter } from "@/components/SiteFooter"
import { StatusBadge } from "@/components/StatusBadge"
import { ThemeToggle } from "@adrianmross/appkit-ui"
import { Button, buttonVariants } from "@adrianmross/appkit-ui/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@adrianmross/appkit-ui/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@adrianmross/appkit-ui/ui/table"
import { cn } from "@adrianmross/appkit-ui"
import { useAuth } from "@adrianmross/appkit-ui"

// Everything in the hero is scripted, not real daemon data. The terminal run
// stops at `approved` because this build enqueues submit work but never clicks Submit.
const SCRIPT: { text: string; kind: "cmd" | "out" | "note" }[] = [
  { kind: "cmd", text: "pursuit list --status to_apply --fields id,company,title" },
  { kind: "out", text: "a_91c  Northwind  Backend Engineer" },
  { kind: "cmd", text: "pursuit apply a_91c --async" },
  { kind: "out", text: "preparing → awaiting_review   (form filled from your standing answers)" },
  { kind: "note", text: "agent stops here — you decide" },
]

const PIPELINE: (Application & { source: string; when: string })[] = [
  { id: "a_91c", company: "Northwind", title: "Backend Engineer", status: "to_apply", source: "Ashby", when: "12m ago" },
  { id: "a_4f2", company: "Acme Robotics", title: "Staff Platform Engineer", status: "to_apply", source: "Company site", when: "2h ago" },
  { id: "a_17e", company: "Fabrikam", title: "Mobile Engineer", status: "interviewing", source: "Lever", when: "1d ago", interviews: { 5: { done: 2, total: 3 } } },
  { id: "a_3b0", company: "Contoso", title: "Site Reliability Engineer", status: "screening", source: "Company site", when: "3d ago" },
  { id: "a_c55", company: "Tailspin", title: "Product Engineer", status: "offer", source: "Ashby", when: "Nov 5" },
  { id: "a_08d", company: "Litware", title: "Infrastructure Engineer", status: "applied", source: "Workday", when: "Oct 20" },
]

function Terminal({ onDone }: { onDone: () => void }) {
  const [lines, setLines] = useState<string[]>([])
  const [typing, setTyping] = useState("")

  useEffect(() => {
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches
    let dead = false
    const sleep = (ms: number) => new Promise((r) => setTimeout(r, reduce ? 0 : ms))
    async function run() {
      await sleep(500)
      for (const { kind, text } of SCRIPT) {
        if (dead) return
        if (kind === "cmd") {
          for (let c = 1; c <= text.length && !dead; c++) {
            setTyping(text.slice(0, c))
            await sleep(24)
          }
          await sleep(250)
          setTyping("")
          setLines((l) => [...l, "$ " + text])
        } else {
          await sleep(kind === "note" ? 700 : 380)
          setLines((l) => [...l, (kind === "note" ? "# " : "  ") + text])
        }
      }
      if (!dead) onDone()
    }
    void run()
    return () => {
      dead = true
    }
  }, [onDone])

  return (
    <pre className="h-full whitespace-pre-wrap px-5 py-4 font-mono text-[11px] leading-6 text-[#e6e7e4] sm:text-[13px] sm:leading-7">
      {lines.map((l, i) => (
        <div key={i} className={l[0] === "#" ? "text-brand-gold" : l[0] === " " ? "text-[#a3a7a4]" : ""}>
          {l}
        </div>
      ))}
      {(typing !== "" || lines.length === 0 || lines.length === SCRIPT.length) && (
        <div>
          $ {typing}
          <span className="ml-0.5 inline-block h-4 w-2 translate-y-0.5 animate-pulse bg-[#e6e7e4] motion-reduce:animate-none" />
        </div>
      )}
    </pre>
  )
}

// The signed-in app's top bar, shrunk: wordmark, search, ⌘P.
function AppBar({ bell }: { bell?: boolean }) {
  return (
    <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-3 border-b px-4 py-2">
      <span>
        <img src="/brand/wordmark.svg" alt="" className="h-4 w-auto dark:hidden" />
        <img src="/brand/wordmark-reverse.svg" alt="" className="hidden h-4 w-auto dark:block" />
      </span>
      <span className="flex h-7 w-40 items-center gap-2 rounded-md border px-2.5 text-xs text-muted-foreground sm:w-72">
        <Search className="size-3.5" /> Search
      </span>
      <span className="flex items-center gap-2 justify-self-end">
        {bell && (
          <span className="relative">
            <Bell className="size-4" />
            <span className="absolute -top-1 -right-1 flex size-3.5 items-center justify-center rounded-full bg-foreground text-[9px] font-semibold text-background">
              1
            </span>
          </span>
        )}
        <kbd className="rounded border bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">⌘P</kbd>
      </span>
    </div>
  )
}

// Later scenes move a row on: `status` overrides a row's status by id.
function PipelineScene({ bell, status }: { bell?: boolean; status?: Record<string, Application["status"]> }) {
  return (
    <div className="flex h-full flex-col bg-background">
      <AppBar bell={bell} />
      <div className="flex flex-col gap-3 p-4">
        <div className="flex items-center justify-between text-xs">
          <span className="text-muted-foreground">{PIPELINE.length} applications</span>
          <span className={buttonVariants({ size: "sm" })}>+ Add</span>
        </div>
        <div className="overflow-hidden rounded-[10px] border bg-card">
          <Table className="text-xs">
            <TableHeader>
              <TableRow>
                <TableHead className="w-8" />
                <TableHead>Company</TableHead>
                <TableHead className="hidden sm:table-cell">Role</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="hidden md:table-cell">Listing</TableHead>
                <TableHead className="hidden sm:table-cell">Date</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {PIPELINE.map((p, i) => {
                const a = status?.[p.id] ? { ...p, status: status[p.id] } : p
                return (
                <TableRow
                  key={a.id}
                  className={cn("animate-in fade-in slide-in-from-bottom-1 fill-mode-both", i === 0 && "bg-muted/60")}
                  style={{ animationDelay: `${i * 70}ms` }}
                >
                  <TableCell>
                    <ProgressMark {...markOf(a)} size={18} />
                  </TableCell>
                  <TableCell>
                    <span className="flex items-center gap-2 font-medium">
                      <CompanyAvatar company={a.company} className="size-5 text-[10px]" />
                      {a.company}
                    </span>
                  </TableCell>
                  <TableCell className="hidden max-w-36 truncate sm:table-cell">{a.title}</TableCell>
                  <TableCell>
                    <StatusBadge status={a.status} />
                  </TableCell>
                  <TableCell className="hidden text-muted-foreground md:table-cell">
                    <span className="flex items-center gap-1.5">
                      <i className="size-1.5 rounded-full bg-emerald-500" />
                      {a.source}
                    </span>
                  </TableCell>
                  <TableCell className="hidden text-muted-foreground sm:table-cell">{a.when}</TableCell>
                </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      </div>
    </div>
  )
}

// What the agent prepared for Northwind, from your standing answers and a draft.
const AGENT_ANSWERS: [string, string, string][] = [
  ["Authorized to work in the US?", "Yes", "standing answer"],
  ["Why Northwind?", "Small team, real traffic, and room to…", "drafted by agent"],
]

function DetailScene() {
  return (
    <div className="flex h-full flex-col bg-background">
      <AppBar />
      <div className="mx-auto flex w-full max-w-2xl flex-col gap-3 p-4 text-left">
        <ArrowLeft className="size-4 text-muted-foreground" />
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex items-start gap-3">
            <ProgressMark stage={2} size={34} />
            <CompanyAvatar company="Northwind" className="mt-0.5 size-8 text-sm" />
            <div>
              <p className="font-display text-lg font-semibold leading-tight">Backend Engineer</p>
              <p className="text-xs text-muted-foreground">Northwind · prepared by agent-1</p>
            </div>
          </div>
          <div className="flex items-center gap-1.5">
            <StatusBadge status="awaiting_review" />
            <span className={cn(buttonVariants({ size: "sm" }), "animate-pulse motion-reduce:animate-none")}>
              <Check /> Approve
            </span>
            <span className={buttonVariants({ size: "icon-sm", variant: "ghost" })}>
              <X className="text-destructive" />
            </span>
          </div>
        </div>
        <Card size="sm" className="animate-in fade-in slide-in-from-bottom-2 fill-mode-both">
          <CardContent className="grid grid-cols-3 gap-3 text-xs">
            {[
              ["Location", "Remote · US"],
              ["Salary", "$150K–$175K"],
              ["Fit", "strong"],
            ].map(([k, v]) => (
              <div key={k}>
                <p className="text-muted-foreground">{k}</p>
                <p className="mt-0.5 font-medium">{v}</p>
              </div>
            ))}
            <div className="col-span-3 flex items-center justify-between">
              <div>
                <p className="text-muted-foreground">Posting URL</p>
                <p className="mt-0.5 font-medium">jobs.ashbyhq.com/northwind/backend-engineer</p>
              </div>
              <ExternalLink className="size-3.5 text-muted-foreground" />
            </div>
          </CardContent>
        </Card>
        <Card size="sm" className="animate-in fade-in slide-in-from-bottom-2 fill-mode-both [animation-delay:120ms]">
          <CardHeader>
            <CardTitle className="text-xs">Prepared answers</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-2 text-xs">
            {AGENT_ANSWERS.map(([q, a, src]) => (
              <div key={q} className="flex items-baseline justify-between gap-3 border-t pt-2 first:border-t-0 first:pt-0 last:max-sm:hidden">
                <span className="text-muted-foreground">{q}</span>
                <span className="truncate text-right font-medium">
                  {a} <span className="ml-1 hidden font-normal text-muted-foreground/70 sm:inline">· {src}</span>
                </span>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

// What the extension fills on the Acme form.
const FIELDS: [string, string, string][] = [
  ["First name", "Ada", "your profile"],
  ["Last name", "Lovelace", "your profile"],
  ["Email", "ada@example.com", "your profile"],
  ["Résumé", "ada-lovelace.pdf", "your résumé"],
  ["Authorized to work in the US?", "Yes", "standing answer"],
  ["What would you build first?", "A merge-to-robot pipeline that…", "drafted by agent"],
]

// An application page with the browser extension's panel beside it: the panel
// fills each field in turn from what you've already told Pursuit.
function FormScene({ onDone }: { onDone: () => void }) {
  const [reduce] = useState(
    () => typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches,
  )
  const [filled, setFilled] = useState(reduce ? FIELDS.length : 0)

  useEffect(() => {
    if (reduce) {
      onDone()
      return
    }
    let n = 0
    const t = setInterval(() => {
      n += 1
      setFilled(n)
      if (n >= FIELDS.length) {
        clearInterval(t)
        onDone()
      }
    }, 650)
    return () => clearInterval(t)
  }, [onDone, reduce])

  const done = filled >= FIELDS.length
  return (
    <div className="flex h-full flex-col bg-background">
      <div className="grid flex-1 gap-4 overflow-hidden p-4 text-left sm:grid-cols-[1fr_13rem]">
        <div className="flex flex-col gap-2.5 text-xs">
          <p className="font-display text-sm font-semibold">Apply for Staff Platform Engineer</p>
          {FIELDS.map(([label, value], i) => (
            <div key={label}>
              <p className="text-muted-foreground">{label}</p>
              <p
                className={cn(
                  "mt-0.5 h-7 truncate rounded-md border px-2 py-1.5 transition-colors duration-300",
                  i < filled ? "border-brand-forest/40 bg-brand-forest/5" : "text-transparent",
                )}
              >
                {i < filled ? value : "."}
              </p>
            </div>
          ))}
        </div>
        <Card size="sm" className="hidden self-start sm:flex">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-xs">
              <ProgressMark stage={2} size={20} /> Pursuit
            </CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-2 text-xs">
            <p className="font-medium">Staff Platform Engineer</p>
            <p className="text-muted-foreground">Acme Robotics · saved</p>
            <div className="h-1.5 overflow-hidden rounded-full bg-muted">
              <div
                className="h-full rounded-full bg-brand-forest transition-[width] duration-500"
                style={{ width: `${(filled / FIELDS.length) * 100}%` }}
              />
            </div>
            <p className="text-muted-foreground">
              {done ? `Filled ${FIELDS.length} of ${FIELDS.length}. You press Submit.` : `Filling ${Math.min(filled + 1, FIELDS.length)} of ${FIELDS.length}…`}
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

// The agent finishes preparing Acme, the bell lights, and opening the
// notification lands on the review where you approve.
function ApproveScene({ onDone }: { onDone: () => void }) {
  const [reduce] = useState(
    () => typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches,
  )
  const [phase, setPhase] = useState(reduce ? 2 : 0)

  useEffect(() => {
    if (reduce) {
      onDone()
      return
    }
    const t = [setTimeout(() => setPhase(1), 900), setTimeout(() => setPhase(2), 3200), setTimeout(onDone, 5600)]
    return () => t.forEach(clearTimeout)
  }, [onDone, reduce])

  if (phase === 2) return <DetailScene />
  return (
    <div className="relative h-full">
      <PipelineScene bell status={{ a_91c: "awaiting_review", a_4f2: "applied" }} />
      {phase === 1 && (
        <div className="absolute top-11 right-3 w-[min(22rem,calc(100%-1.5rem))] rounded-lg border bg-popover text-left text-popover-foreground shadow-lg animate-in fade-in slide-in-from-top-2">
          <div className="flex items-center gap-2 border-b px-4 py-2.5">
            <span className="flex-1 font-heading text-sm font-semibold">Notifications</span>
            <span className="text-xs text-muted-foreground">Live</span>
          </div>
          <div className="flex items-start gap-3 px-4 py-3">
            <span className="mt-1.5 size-2 shrink-0 rounded-full bg-foreground" aria-hidden />
            <div className="flex flex-col gap-1">
              <p className="text-sm">
                <b className="font-semibold">Backend Engineer</b> is prepared and waiting for you.
              </p>
              <p className="text-xs text-muted-foreground">Northwind · prepared by agent-1 · just now</p>
              <span className={cn(buttonVariants({ size: "sm" }), "mt-1 self-start animate-pulse motion-reduce:animate-none")}>
                Review
              </span>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

const SCENES = [
  { label: "Track", title: `${PRODUCT_NAME} — Pipeline`, caption: "Every role you're chasing, in one place." },
  { label: "Autofill", title: "Acme Robotics — Apply", caption: "Open an application and Pursuit fills it from everything you've told it before." },
  { label: "Automate", title: "zsh — pursuit", caption: "Or hand the queue to an agent from the terminal." },
  { label: "Approve", title: `${PRODUCT_NAME} — Pipeline`, caption: "Your agent prepares it and you get a notification. Nothing goes out until you approve." },
]
// The scenes that play a script and advance when it finishes.
const SCRIPTED = [1, 2, 3]
const DWELL = 5000

function Showcase() {
  const [scene, setScene] = useState(0)
  const [paused, setPaused] = useState(false)
  const [typed, setTyped] = useState(false)
  const [reduce] = useState(
    () => typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches,
  )

  const done = useCallback(() => setTyped(true), [])

  function go(i: number) {
    if (i === scene) return
    setTyped(false)
    setScene(i)
  }

  // Dashboard and review dwell on a timer; the CLI scene waits for its script.
  useEffect(() => {
    if (reduce || paused || (SCRIPTED.includes(scene) && !typed)) return
    const t = setTimeout(() => {
      setTyped(false)
      setScene((scene + 1) % SCENES.length)
    }, SCRIPTED.includes(scene) ? 1800 : DWELL)
    return () => clearTimeout(t)
  }, [scene, paused, typed, reduce])

  const cli = scene === 2
  return (
    <section
      aria-roledescription="carousel"
      aria-label={`${PRODUCT_NAME}: track, autofill, automate, approve`}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
      className="relative isolate mx-auto w-full max-w-4xl"
    >
      <div
        aria-hidden
        className="pointer-events-none absolute -inset-x-16 -inset-y-10 -z-10 bg-[radial-gradient(ellipse_at_center,color-mix(in_oklch,var(--brand-forest)_22%,transparent),transparent_65%)]"
      />
      <div
        className={cn(
          "overflow-hidden rounded-xl text-left shadow-2xl shadow-black/10 ring-1 transition-colors duration-500",
          cli ? "bg-[#0e0f0f] ring-white/10" : "bg-card ring-foreground/10",
        )}
      >
        <div
          className={cn(
            "flex items-center gap-1.5 border-b px-4 py-2.5 text-xs transition-colors duration-500",
            cli ? "border-white/10 text-[#e6e7e4]" : "text-foreground",
          )}
        >
          <span className="size-2.5 rounded-full bg-current opacity-20" />
          <span className="size-2.5 rounded-full bg-current opacity-20" />
          <span className="size-2.5 rounded-full bg-current opacity-20" />
          <b className="mx-auto pr-10 font-medium opacity-60">{SCENES[scene].title}</b>
        </div>
        <div
          key={scene}
          role="group"
          aria-roledescription="slide"
          aria-label={`${scene + 1} of ${SCENES.length}: ${SCENES[scene].label}`}
          className={cn(
            "h-[460px] overflow-hidden sm:h-[420px] animate-in fade-in duration-500",
            cli ? "slide-in-from-bottom-6 zoom-in-[0.98]" : "zoom-in-[0.99]",
          )}
        >
          {scene === 0 && <PipelineScene />}
          {scene === 1 && <FormScene onDone={done} />}
          {scene === 2 && <Terminal onDone={done} />}
          {scene === 3 && <ApproveScene onDone={done} />}
        </div>
      </div>

      <div className="mt-5 flex flex-col items-center gap-3">
        <div className="flex gap-1 rounded-full border bg-card p-1">
          {SCENES.map((s, i) => (
            <Button
              key={s.label}
              variant="ghost"
              size="sm"
              aria-current={i === scene}
              onClick={() => go(i)}
              className={cn(
                "relative overflow-hidden rounded-full px-4",
                i === scene && "bg-muted text-foreground",
              )}
            >
              {s.label}
              {i === scene && !reduce && !paused && !SCRIPTED.includes(i) && (
                <span
                  aria-hidden
                  className="absolute inset-x-3 bottom-0.5 h-px origin-left bg-foreground/40"
                  style={{ animation: `grow ${DWELL}ms linear both` }}
                />
              )}
            </Button>
          ))}
        </div>
        <p aria-live={paused || reduce ? "polite" : "off"} className="px-4 text-sm text-balance text-muted-foreground">
          {SCENES[scene].caption}
        </p>
      </div>
    </section>
  )
}

const DOCS = [
  { href: "/docs/getting-started", title: "Getting started", body: "From your first sign-in to your first prepared application." },
  { href: "/docs/cli", title: "CLI reference", body: "Every pursuit command, its flags, and the JSON it prints." },
  { href: "/docs/api-reference", title: "API reference", body: "The HTTP API behind the dashboard and the CLI." },
]

const link = "text-sm font-medium text-foreground/70 transition-colors hover:text-foreground"

export function Landing() {
  // Signed-in visitors skip /sign-in and go straight to their start page.
  const { me } = useAuth()
  const entry = me ? "/start" : "/sign-in"
  return (
    <div className="min-h-svh overflow-x-clip bg-background text-foreground">
      <header className="sticky top-0 z-20 border-b border-transparent bg-background/80 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-end px-5">
          <nav className="flex items-center gap-5">
            <a href="/docs/" className={link}>
              Docs
            </a>
            <a href="/reference" className={cn(link, "hidden sm:inline")}>
              Reference
            </a>
            <a href="https://github.com/adrianmross/pursuit" className={cn(link, "hidden sm:inline")}>
              GitHub
            </a>
            <ThemeToggle />
            <Link href={entry} className={buttonVariants({ size: "sm" })}>
              {me ? "Open app" : "Log in"}
            </Link>
          </nav>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-4 pb-24 sm:px-5">
        <section className="pt-10 text-center sm:pt-14">
          <img src="/brand/mark.svg" alt={PRODUCT_NAME} className="mx-auto mb-5 h-12 w-auto dark:hidden" />
          <img src="/brand/mark-reverse.svg" alt={PRODUCT_NAME} className="mx-auto mb-5 hidden h-12 w-auto dark:block" />
          <h1 className="font-heading text-4xl font-semibold leading-[1.05] tracking-tight text-balance sm:text-5xl">
            For your higher pursuits.
          </h1>
          <p className="mx-auto mt-5 max-w-xl text-lg text-balance text-muted-foreground">
            Job applications for the modern era.
          </p>
          <Link
            href={entry}
            className="group mb-12 mt-8 inline-flex items-center gap-2 rounded-sm font-display text-2xl font-semibold tracking-tight underline decoration-transparent decoration-2 underline-offset-[6px] transition-[text-decoration-color] hover:decoration-current focus-visible:decoration-current focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring"
          >
            Get started
            <ArrowRight className="size-5 transition-transform group-hover:translate-x-1 group-focus-visible:translate-x-1 motion-reduce:transition-none" />
          </Link>
          <Showcase />
        </section>

        <nav aria-label="Documentation" className="mx-auto mt-24 grid max-w-4xl border-t sm:grid-cols-3 sm:divide-x">
          {DOCS.map(({ href, title, body }, i) => (
            <a
              key={href}
              href={href}
              className="group flex flex-col gap-2 border-b py-7 transition-colors sm:border-b-0 sm:px-8 sm:first:pl-0 sm:last:pr-0"
            >
              <span className="font-mono text-xs text-muted-foreground">0{i + 1}</span>
              <span className="flex items-center gap-2 font-heading text-lg font-semibold tracking-tight">
                {title}
                <ArrowRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-1 group-hover:text-foreground" />
              </span>
              <span className="text-sm leading-relaxed text-muted-foreground">{body}</span>
            </a>
          ))}
        </nav>
      </main>

      <SiteFooter />
    </div>
  )
}
