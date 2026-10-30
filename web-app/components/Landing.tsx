"use client"

import { useCallback, useEffect, useState } from "react"
import Link from "next/link"
import { ArrowLeft, ArrowRight, Check, ExternalLink, Search, X } from "lucide-react"
import { markOf, type Application } from "@/lib/types"
import { CompanyAvatar } from "@/components/CompanyAvatar"
import { ProgressMark } from "@/components/ProgressMark"
import { SiteFooter } from "@/components/SiteFooter"
import { StatusBadge } from "@/components/StatusBadge"
import { Button, buttonVariants } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { cn } from "@/lib/utils"

// Everything in the hero is scripted, not real daemon data. The terminal run
// stops at `approved` because this build enqueues submit work but never clicks Submit.
const SCRIPT: { text: string; kind: "cmd" | "out" | "note" }[] = [
  { kind: "cmd", text: "pursuit list --status to_apply --fields id,company,title" },
  { kind: "out", text: "a_4f2  Halcyon      Staff Platform Engineer" },
  { kind: "out", text: "a_91c  Northwind    Backend Engineer" },
  { kind: "cmd", text: "pursuit apply a_4f2 --async" },
  { kind: "out", text: "preparing → awaiting_review   (form filled from your standing answers)" },
  { kind: "note", text: "agent stops here — you decide" },
]

const PIPELINE: (Application & { source: string; when: string })[] = [
  { id: "a_4f2", company: "Halcyon", title: "Staff Platform Engineer", status: "to_apply", source: "Greenhouse", when: "2h ago" },
  { id: "a_91c", company: "Northwind", title: "Backend Engineer", status: "to_apply", source: "Ashby", when: "5h ago" },
  { id: "a_17e", company: "Fabrikam", title: "Mobile Engineer", status: "interviewing", source: "Lever", when: "1d ago", interviews: { 5: { done: 2, total: 3 } } },
  { id: "a_3b0", company: "Contoso", title: "Site Reliability Engineer", status: "screening", source: "Company site", when: "3d ago" },
  { id: "a_c55", company: "Tailspin", title: "Product Engineer", status: "offer", source: "Ashby", when: "Jan 16" },
  { id: "a_08d", company: "Litware", title: "Infrastructure Engineer", status: "applied", source: "Workday", when: "Jan 15" },
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

// The signed-in app's top bar, shrunk: wordmark, search, ⌘K.
function AppBar() {
  return (
    <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-3 border-b px-4 py-2">
      <span>
        <img src="/brand/hire-pursuit-wordmark.svg" alt="" className="h-4 w-auto dark:hidden" />
        <img src="/brand/hire-pursuit-wordmark-reverse.svg" alt="" className="hidden h-4 w-auto dark:block" />
      </span>
      <span className="flex h-7 w-40 items-center gap-2 rounded-md border px-2.5 text-xs text-muted-foreground sm:w-72">
        <Search className="size-3.5" /> Search
      </span>
      <kbd className="justify-self-end rounded border bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">⌘K</kbd>
    </div>
  )
}

function PipelineScene() {
  return (
    <div className="flex h-full flex-col bg-background">
      <AppBar />
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
              {PIPELINE.map((a, i) => (
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
              ))}
            </TableBody>
          </Table>
        </div>
      </div>
    </div>
  )
}

const ANSWERS: [string, string, string][] = [
  ["Are you authorized to work in the US?", "Yes", "standing answer"],
  ["Desired compensation", "$220K base", "standing answer"],
  ["Why Halcyon?", "Platform work at the scale of your build fleet…", "drafted by agent"],
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
            <CompanyAvatar company="Halcyon" className="mt-0.5 size-8 text-sm" />
            <div>
              <p className="font-display text-lg font-semibold leading-tight">Staff Platform Engineer</p>
              <p className="text-xs text-muted-foreground">Halcyon</p>
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
              ["Salary", "$210K–$245K"],
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
                <p className="mt-0.5 font-medium">boards.greenhouse.io/halcyon/4021</p>
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
            {ANSWERS.map(([q, a, src]) => (
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

const SCENES = [
  { label: "Dashboard", title: "Hire Pursuit — Pipeline", caption: "Every role you're chasing, at a glance." },
  { label: "CLI", title: "zsh — pursuit", caption: "An agent takes it from the terminal and prepares the form." },
  { label: "Review", title: "Hire Pursuit — Halcyon", caption: "Nothing goes out until you approve it." },
]
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
    if (reduce || paused || (scene === 1 && !typed)) return
    const t = setTimeout(() => {
      setTyped(false)
      setScene((scene + 1) % SCENES.length)
    }, scene === 1 ? 1800 : DWELL)
    return () => clearTimeout(t)
  }, [scene, paused, typed, reduce])

  const cli = scene === 1
  return (
    <section
      aria-roledescription="carousel"
      aria-label="Hire Pursuit, from the dashboard to the CLI and back"
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
          {scene === 1 && <Terminal onDone={done} />}
          {scene === 2 && <DetailScene />}
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
              {i === scene && !reduce && !paused && i !== 1 && (
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
  return (
    <div className="min-h-svh overflow-x-clip bg-background text-foreground">
      <header className="sticky top-0 z-20 border-b border-transparent bg-background/80 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
          <Link href="/" aria-label="Hire Pursuit">
            <img src="/brand/hire-pursuit-wordmark.svg" alt="Hire Pursuit" className="h-6 w-auto dark:hidden" />
            <img src="/brand/hire-pursuit-wordmark-reverse.svg" alt="" className="hidden h-6 w-auto dark:block" />
          </Link>
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
            <Link href="/sign-in" className={buttonVariants({ size: "sm" })}>
              Log in
            </Link>
          </nav>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-4 pb-24 sm:px-5">
        <section className="pt-10 text-center sm:pt-14">
          <img src="/brand/hire-pursuit-mark.svg" alt="" className="mx-auto mb-5 h-12 w-auto dark:hidden" />
          <img src="/brand/hire-pursuit-mark-reverse.svg" alt="" className="mx-auto mb-5 hidden h-12 w-auto dark:block" />
          <h1 className="font-heading text-5xl font-semibold leading-[0.95] tracking-tight text-balance sm:text-7xl">
            For your higher pursuits.
          </h1>
          <p className="mx-auto mt-6 max-w-xl text-lg text-balance text-muted-foreground">
            Every application, from first look to offer.
          </p>
          <div className="mb-12 mt-8 flex flex-wrap justify-center gap-3">
            <Link href="/sign-in" className={buttonVariants({ size: "lg" })}>
              Get started <ArrowRight />
            </Link>
            <a href="/docs/" className={buttonVariants({ size: "lg", variant: "outline" })}>
              Read the docs
            </a>
          </div>
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
