"use client"

import { useCallback, useEffect, useId, useRef, useState } from "react"
import { usePathname, useRouter } from "next/navigation"
import { api } from "@adrianmross/appkit-ui"
import { applicationHref } from "@/lib/links"
import { PRODUCT_NAME } from "@/lib/brand"
import { setPref } from "@adrianmross/appkit-ui"
import { useAuth } from "@adrianmross/appkit-ui"
import { usePref } from "@adrianmross/appkit-ui"
import type { Application, Page } from "@/lib/types"
import { Button } from "@adrianmross/appkit-ui/ui/button"

// A step points at [data-tour=target] on a page matching `match`; `route` is where Next goes
// ("{app}" is the first application). `open` waits for the visitor to open an application.
type Step = { target: string; route: string; match: RegExp; title: string; body: string; hint?: string; needsApp?: boolean; open?: boolean }

const STEPS: Step[] = [
  { target: "get-started", route: "/start", match: /^\/start$/, title: "Your feed", body: "Prompts for your profile, an agent, the terminal and the browser extension. Dismiss what you don't need." },
  { target: "table", route: "/applications", match: /^\/applications$/, title: "Your applications", body: "Every posting you track, by stage. Search with the box above, or press ⌘P for the command palette.", hint: "Open any application to continue.", needsApp: true, open: true },
  { target: "details", route: "{app}", match: /^\/app\//, title: "One application", body: "Details holds the listing, then tabs for rounds, tasks, questions and notes. Each round's notes are one markdown document: a heading ending in ? is a question.", hint: "Try the tabs, it's all live.", needsApp: true },
  { target: "questions", route: "/profile/questions", match: /^\/profile\/questions$/, title: "Your answers", body: "Answer a question once and it fills in on every form that asks. Interview questions from your notes land here too, so a good answer is one click from reuse." },
  { target: "account", route: "/profile/questions", match: /^\/profile\/questions$/, title: "That's the tour", body: "Replay it any time from Settings › About." },
]

const PAD = 6
// Everything fades: out before a step changes, in once the page is ready and the target is found.
const FADE = 220
// A target that never shows up (a missing card) gets a centered note instead.
const GIVE_UP = 3000

export function TourOverlay() {
  const router = useRouter()
  const { isDemo } = useAuth()
  const pathname = usePathname()
  const [steps, setSteps] = useState<Step[] | null>(null)
  const [appHref, setAppHref] = useState("")
  const [i, setI] = useState(0)
  const [box, setBox] = useState<DOMRect | null>(null)
  const [shown, setShown] = useState(false)
  const [missed, setMissed] = useState(false)
  const prev = useRef(pathname)
  // Replay ("go") skips the welcome; a first join shows it.
  const [started, setStarted] = useState(usePref("tour") === "go")

  // An empty account has no table or application to point at.
  useEffect(() => {
    api<Page<Application>>("/v1/applications?limit=1")
      .then((p) => {
        const first = p.data?.[0]
        setAppHref(first ? applicationHref(first.id) : "")
        setSteps(STEPS.filter((s) => first || !s.needsApp))
      })
      .catch(() => setSteps(STEPS.filter((s) => !s.needsApp)))
  }, [])

  const step = started ? (steps?.[i] ?? null) : null
  const here = step ? step.match.test(pathname) : false

  // Fade out, then act: the next step, or the end.
  const go = useCallback(
    (n: number) => {
      setShown(false)
      setTimeout(() => setI(Math.max(0, Math.min(n, (steps?.length ?? 1) - 1))), FADE)
    },
    [steps],
  )
  const end = useCallback(() => {
    setShown(false)
    setTimeout(() => setPref("tour", "off"), FADE)
  }, [])

  // Take the visitor to the step's page, unless they are already on a matching one.
  useEffect(() => {
    if (step && !here) router.push(step.route.replace("{app}", appHref))
  }, [step, here, appHref, router])

  // Opening an application by hand is what the table step asks for.
  useEffect(() => {
    const came = prev.current !== pathname && /^\/app\//.test(pathname) && !/^\/app\//.test(prev.current)
    prev.current = pathname
    if (came && step?.open) go(i + 1)
  }, [pathname, step, i, go])

  // Find the target (it may mount late), then follow it through scroll, resize and reflow.
  useEffect(() => {
    if (!step || !here) return
    let el: Element | null = null
    let raf = 0
    let obs: ResizeObserver | undefined
    let watch: MutationObserver | undefined
    const measure = () => {
      raf = 0
      if (el) setBox(el.getBoundingClientRect())
    }
    const schedule = () => {
      if (!raf) raf = requestAnimationFrame(measure)
    }
    const attach = (found: Element) => {
      el = found
      watch?.disconnect()
      found.scrollIntoView({ block: "center", behavior: "smooth" })
      obs = new ResizeObserver(schedule)
      obs.observe(found)
      window.addEventListener("scroll", schedule, true)
      window.addEventListener("resize", schedule)
      schedule()
    }
    const giveUp = setTimeout(() => setMissed(true), GIVE_UP)
    const sel = `[data-tour="${step.target}"]`
    const now = document.querySelector(sel)
    if (now) attach(now)
    else {
      watch = new MutationObserver(() => {
        const found = document.querySelector(sel)
        if (found) attach(found)
      })
      watch.observe(document.body, { childList: true, subtree: true })
    }
    return () => {
      setBox(null)
      setShown(false)
      setMissed(false)
      clearTimeout(giveUp)
      cancelAnimationFrame(raf)
      obs?.disconnect()
      watch?.disconnect()
      window.removeEventListener("scroll", schedule, true)
      window.removeEventListener("resize", schedule)
    }
  }, [step, here])

  // Fade in once the target (or the note that stands in for it) is ready.
  const ready = box !== null || missed
  useEffect(() => {
    if (!ready) return
    let b = 0
    const a = requestAnimationFrame(() => (b = requestAnimationFrame(() => setShown(true))))
    return () => {
      cancelAnimationFrame(a)
      cancelAnimationFrame(b)
    }
  }, [ready, step])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") end()
      else if (!started) return
      else if (e.key === "ArrowRight") go(i + 1)
      else if (e.key === "ArrowLeft") go(i - 1)
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [i, go, end, started])

  if (!started) return <Welcome onStart={() => setStarted(true)} onSkip={() => setPref("tour", "off")} />
  if (!step || !steps || !ready) return null
  const last = i === steps.length - 1
  // Beside the target, below it when there is room and above otherwise; centered when it isn't on the page.
  const place: React.CSSProperties = box
    ? {
        left: Math.min(Math.max(16, box.left - PAD), Math.max(16, window.innerWidth - 336)),
        ...(box.bottom + 220 < window.innerHeight ? { top: box.bottom + PAD + 12 } : { bottom: Math.max(16, window.innerHeight - box.top + PAD + 12) }),
      }
    : { left: "50%", top: "40%", transform: "translateX(-50%)" }
  return (
    <div className="pointer-events-none fixed inset-0 z-[100] transition-opacity duration-300 motion-reduce:transition-none" style={{ opacity: shown ? 1 : 0 }}>
      {/* The dim is the hole's shadow, so the highlighted page stays clickable. The whole layer fades; nothing slides. */}
      {box ? (
        <div
          className="absolute rounded-lg ring-2 ring-primary"
          style={{ top: box.top - PAD, left: box.left - PAD, width: box.width + PAD * 2, height: box.height + PAD * 2, boxShadow: "0 0 0 9999px rgb(0 0 0 / 0.5)" }}
        />
      ) : (
        <div className="absolute inset-0 bg-black/50" />
      )}
      <div
        role="dialog"
        aria-label={step.title}
        className="pointer-events-auto absolute w-80 max-w-[calc(100vw-2rem)] rounded-lg border bg-popover p-4 text-popover-foreground shadow-lg"
        style={place}
      >
        <h2 className="font-medium">{step.title}</h2>
        <p className="mt-1 text-sm text-muted-foreground">{step.body}{isDemo && step.target === "account" && " Your changes here are a throwaway sandbox."}</p>
        {step.hint && <p className="mt-2 text-sm font-medium">{step.hint}</p>}
        <div className="mt-3 flex items-center gap-2">
          <div className="flex gap-1" aria-label={`Step ${i + 1} of ${steps.length}`}>
            {steps.map((s, n) => (
              <button
                key={s.target}
                type="button"
                aria-label={`Go to step ${n + 1}`}
                onClick={() => go(n)}
                className={`size-1.5 rounded-full transition-colors ${n === i ? "bg-primary" : "bg-muted-foreground/30 hover:bg-muted-foreground/60"}`}
              />
            ))}
          </div>
          <Button variant="ghost" size="sm" className="ml-auto" onClick={end}>
            Skip
          </Button>
          {i > 0 && (
            <Button variant="outline" size="sm" onClick={() => go(i - 1)}>
              Back
            </Button>
          )}
          <Button size="sm" autoFocus onClick={() => (last ? end() : go(i + 1))}>
            {last ? "Done" : "Next"}
          </Button>
        </div>
      </div>
    </div>
  )
}

// A trail of three stops to a flag, built in order: each stop pops, the dotted line grows to the next,
// then the pole grows and the flag unfurls last. The dots are one fixed pattern revealed by a growing
// mask, so they never shift while it draws. Reduced motion shows it finished.
const SEGMENTS = ["M16 92 C60 92 70 40 112 52", "M112 52 C154 64 170 100 208 62", "M208 62 C246 24 262 22 292 30"]
const STOPS = [[16, 92], [112, 52], [208, 62]]
const STEP = 0.5 // seconds from one stop to the next

function TrailArt() {
  const id = useId().replace(/:/g, "")
  const at = (s: number) => ({ animationDelay: `${s}s` })
  return (
    <svg viewBox="0 0 320 120" className="h-28 w-full text-primary" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" aria-hidden>
      <defs>
        {SEGMENTS.map((d, n) => (
          <mask key={d} id={`${id}-${n}`} maskUnits="userSpaceOnUse" x="0" y="0" width="320" height="120">
            <path d={d} pathLength="1" stroke="white" strokeWidth="10" strokeDasharray="1" className="motion-safe:animate-[tour-line_0.4s_ease-in-out_both]" style={at(0.3 + n * STEP)} />
          </mask>
        ))}
      </defs>
      {SEGMENTS.map((d, n) => (
        <path key={d} d={d} strokeDasharray="0.1 7" strokeWidth="3.5" className="opacity-70" mask={`url(#${id}-${n})`} />
      ))}
      {STOPS.map(([x, y], n) => (
        <circle key={n} cx={x} cy={y} r="6" className="fill-background [transform-box:fill-box] [transform-origin:center] motion-safe:animate-[tour-pop_0.35s_ease-out_both]" style={at(n * STEP)} />
      ))}
      <rect x="291" y="6" width="2" height="24" rx="1" className="fill-current [transform-box:fill-box] [transform-origin:50%_100%] motion-safe:animate-[tour-grow_0.3s_ease-out_both]" style={at(0.3 + 3 * STEP)} />
      <path d="M293 6 L313 13 L293 20 Z" strokeWidth="1" className="fill-current [transform-box:fill-box] [transform-origin:0_50%] motion-safe:animate-[tour-unfurl_0.4s_ease-out_both]" style={at(0.55 + 3 * STEP)} />
    </svg>
  )
}

function Welcome({ onStart, onSkip }: { onStart: () => void; onSkip: () => void }) {
  const [shown, setShown] = useState(false)
  useEffect(() => {
    const id = requestAnimationFrame(() => setShown(true))
    return () => cancelAnimationFrame(id)
  }, [])
  const leave = (then: () => void) => {
    setShown(false)
    setTimeout(then, FADE)
  }
  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50 p-4 transition-opacity duration-300 motion-reduce:transition-none" style={{ opacity: shown ? 1 : 0 }}>
      <div role="dialog" aria-modal="true" aria-labelledby="tour-welcome" className="w-full max-w-md rounded-xl border bg-popover p-6 text-center text-popover-foreground shadow-xl">
        <TrailArt />
        <h2 id="tour-welcome" className="mt-2 flex flex-col items-center gap-2">
          <span className="text-sm text-muted-foreground">Welcome to</span>
          {/* eslint-disable-next-line @next/next/no-img-element -- drawn SVG wordmark, not a photo next/image needs to optimize */}
          <img src="/brand/wordmark.svg" alt={PRODUCT_NAME} className="h-7 w-auto dark:hidden" />
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src="/brand/wordmark-reverse.svg" alt="" aria-hidden className="hidden h-7 w-auto dark:block" />
        </h2>
        <p className="mt-4 text-sm text-muted-foreground">Track every application from posting to offer. Take a two-minute tour of how it works, or jump straight in.</p>
        <div className="mt-5 flex flex-col gap-2 sm:flex-row-reverse">
          <Button className="flex-1" autoFocus onClick={() => leave(onStart)}>
            Take the tutorial
          </Button>
          <Button variant="outline" className="flex-1" onClick={() => leave(onSkip)}>
            Skip for now
          </Button>
        </div>
        <p className="mt-3 text-xs text-muted-foreground">You can come back to it any time in Settings › About.</p>
      </div>
    </div>
  )
}
