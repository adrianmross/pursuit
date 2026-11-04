// The in-page panel, loaded by content.ts on postings and forms. A person or
// an agent driving the browser uses the same buttons: the panel lives in an
// open shadow root on [data-pursuit-panel], and after a fill that element
// carries data-pursuit-state, -filled and -missed. It never clicks Submit.

import { useEffect, useRef, useState, type ReactNode } from "react"
import { createRoot } from "react-dom/client"
import { ExternalLink, X } from "lucide-react"
import { answerFor, autofillDocument, normalizeKey, questionFor, type FillReport } from "./autofill"
import { extractJob } from "./extract"
import { hasForm } from "./detect"
import { decodeFile, type App } from "./api"
import { ProgressMark } from "@/components/ProgressMark"
import { StatusBadge } from "@/components/StatusBadge"
import { Die, HoldIcon, Sketch } from "@/components/Sketch"
import { Button } from "@/components/ui/button"
import { statusPieces } from "@/lib/status"
import hirePursuit from "../../web-app/public/brand/hire-pursuit-wordmark-reverse.svg"

// The launcher shows just "pursuit": crop the wordmark's view past "hire".
const wordmark = hirePursuit.replace(/viewBox="[^"]*" width="\d+" height="\d+"/, 'viewBox="220 -6 331.5 94"').replace('aria-label="hire pursuit"', 'aria-label="pursuit"')

declare const __PANEL_CSS__: string

type Doc = { id: string; kind: string; filename: string; content_type: string; created_at: string }
type AppRow = App & { created_at?: string }

class ApiError extends Error {
  code?: string
}

function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  return new Promise((resolve, reject) => {
    chrome.runtime.sendMessage({ type: "api", method, path, body }, (res) => {
      if (chrome.runtime.lastError || !res) return reject(new Error(chrome.runtime.lastError?.message || "Pursuit is reloading. Try again."))
      if (res.ok) return resolve(res.data as T)
      reject(Object.assign(new ApiError(res.error), { code: res.code }))
    })
  })
}

// The form often lives at the posting URL plus /apply or /application.
function candidates(url: string): string[] {
  const bare = url.replace(/\/(apply|application)\/?(?=[?#]|$)/, "")
  return bare === url ? [url] : [url, bare]
}

function source(url: string): string {
  const host = new URL(url).hostname
  if (host.includes("myworkdayjobs")) return "workday"
  return ["greenhouse", "ashby", "lever"].find((s) => host.includes(s)) || "company_site"
}

function newest<T extends { created_at: string }>(rows?: T[]): T | undefined {
  return rows?.slice().sort((a, b) => b.created_at.localeCompare(a.created_at))[0]
}

function ago(iso?: string): string {
  if (!iso) return ""
  const min = Math.round((Date.now() - new Date(iso).getTime()) / 60000)
  if (min < 1) return "just now"
  if (min < 60) return `${min} min ago`
  const h = Math.round(min / 60)
  return h < 24 ? `${h} h ago` : new Date(iso).toLocaleDateString()
}

// The tailored résumé on this application, else the newest base résumé.
async function resumeFile(app?: App): Promise<File | undefined> {
  if (app) {
    const arts = await call<{ data: Doc[] }>("GET", `/v1/applications/${app.id}/artifacts`)
    const pdf = newest(arts.data?.filter((a) => a.kind === "resume_pdf"))
    if (pdf) {
      const art = await call<{ body: string; filename: string; content_type: string }>("GET", `/v1/artifacts/${pdf.id}`)
      return decodeFile(art.body, art.filename, art.content_type || "application/pdf", true)
    }
  }
  const docs = await call<{ data: Doc[] }>("GET", "/v1/documents")
  const base = newest(docs.data?.filter((d) => d.kind === "base_resume"))
  if (!base) return undefined
  const doc = await call<{ body: string; filename: string; content_type: string }>("GET", `/v1/documents/${base.id}`)
  return decodeFile(doc.body, doc.filename, doc.content_type, !/^text\/|json/i.test(doc.content_type || ""))
}

// A hand-drawn edge around the panel: low-frequency noise bends the line in a
// few long, smooth waves instead of roughing up every pixel.
function RoughFrame() {
  return (
    <svg aria-hidden className="pointer-events-none absolute inset-0 size-full overflow-visible">
      <filter id="pursuit-rough" x="-5%" y="-5%" width="110%" height="110%">
        <feTurbulence type="fractalNoise" baseFrequency="0.006" numOctaves="1" seed="3" />
        <feDisplacementMap in="SourceGraphic" scale="4" />
      </filter>
      <rect x="2" y="2" rx="14" filter="url(#pursuit-rough)" className="fill-card stroke-foreground" strokeWidth="2" style={{ width: "calc(100% - 4px)", height: "calc(100% - 4px)" }} />
    </svg>
  )
}

function Tick({ children, faded }: { children: ReactNode; faded?: boolean }) {
  return (
    <div className={`flex items-center gap-2 text-sm animate-in fade-in slide-in-from-top-1 ${faded ? "opacity-40" : ""}`}>
      <Sketch viewBox="0 0 20 20" roughness={1.5} className="size-4 shrink-0">
        <path d="M3 11 L8 16 L17 4" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" />
      </Sketch>
      <span className="min-w-0 truncate">{children}</span>
    </div>
  )
}

// How long each filled field stays on screen before the next one.
const PACE_MS = 350

// The roll: the die spins and the bar fills while the form fills, the field just
// filled ticks in, then it lands on "Rolled n of m" and the list settles.
function Roll({ report, left }: { report: FillReport; left: number | null }) {
  const rolling = left !== null
  const filled = report.filled.length
  const total = filled + report.missed.length
  // While rolling, fields not reached yet count against the bar, so it only grows.
  const out = total + (left ?? 0)
  const pips = rolling ? (filled % 6) + 1 : Math.max(1, Math.min(6, Math.round((6 * filled) / Math.max(1, total))))
  const fields = report.filled.filter((f) => !report.attached || !/resume|résumé|cv/i.test(f))
  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex items-center gap-2.5">
        <Sketch viewBox="0 0 44 44" roughness={2} className={`size-8 shrink-0 ${rolling ? "animate-[spin_1.1s_linear_infinite] motion-reduce:animate-none" : ""}`}>
          <Die x={22} y={22} size={30} tilt={-8} pips={pips} />
        </Sketch>
        <span className="flex-1 text-sm">{rolling ? "Rolling…" : "Rolled"}</span>
        <b className="text-sm font-semibold">{rolling ? filled : `${filled} of ${total}`}</b>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-border" role="progressbar" aria-valuemin={0} aria-valuemax={out} aria-valuenow={filled} aria-label="Fields filled">
        <div className="h-full bg-foreground transition-[width] duration-300 ease-out" style={{ width: `${(100 * filled) / Math.max(1, out)}%` }} />
      </div>
      <div className="flex flex-col gap-1.5">
        {report.attached && <Tick>Attached {report.attached}</Tick>}
        {rolling ? fields.length > 0 && <Tick key={fields.length}>{fields.at(-1)}</Tick> : fields.length > 0 && <Tick faded>Profile fields filled</Tick>}
      </div>
    </div>
  )
}

// What the form asks that Pursuit had no answer for: a stack of sticky notes, one question on top.
function LeftForYou({ fields, onSaved }: { fields: Element[]; onSaved: (note: string) => void }) {
  const [index, setIndex] = useState(0)
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState<Set<number>>(new Set())
  const open = fields.map((_, i) => i).filter((i) => !done.has(i))
  const current = open.includes(index) ? index : open[0]
  const el = current === undefined ? undefined : fields[current]

  useEffect(() => {
    if (!el) return
    el.scrollIntoView({ block: "center", behavior: "smooth" })
    ;(el as HTMLElement).focus?.({ preventScroll: true })
  }, [el])

  if (!el || current === undefined) return null
  const position = open.indexOf(current)
  const next = () => setIndex(open[(position + 1) % open.length])

  async function save() {
    const hit = answerFor(el!)
    if (!hit) {
      onSaved("Type your answer into the outlined field first.")
      return
    }
    setBusy(true)
    try {
      await call("POST", "/v1/standing", { key: hit[0], answer: hit[1] })
      el!.classList.remove("pursuit-unfilled")
      setDone((d) => new Set(d).add(current!))
      onSaved(open.length > 1 ? "Saved. On to the next one." : "Saved. Every later form reuses your answers.")
      if (open.length > 1) next()
    } catch (err) {
      onSaved((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const layers = Math.min(open.length - 1, 2)
  return (
    <div className="relative mx-1 mt-2 mb-1.5">
      {layers >= 2 && <div className="absolute inset-0 translate-x-2 translate-y-2 rotate-3 bg-border" aria-hidden />}
      {layers >= 1 && <div className="absolute inset-0 translate-x-1 translate-y-1 rotate-[1.5deg] bg-border/80" aria-hidden />}
      <section aria-label="Left for you" className="relative flex -rotate-[1.5deg] flex-col gap-2 bg-muted px-4 pt-4 pb-3.5">
        <div className="absolute -top-2 left-1/2 h-4 w-16 -translate-x-1/2 rotate-3 bg-border/90" aria-hidden />
        <HoldIcon status="needs_input" className="absolute top-2.5 right-3 size-7" />
        <span className="text-xs font-semibold text-muted-foreground">
          Left for you · {position + 1} of {open.length}
        </span>
        <span className="pr-8 font-heading text-base font-semibold">{questionFor(el)}</span>
        <span className="text-xs text-muted-foreground">Type it on the form. We outlined it.</span>
        <div className="mt-1 flex items-center gap-3">
          <Button variant="outline" size="sm" disabled={busy} onClick={save}>
            {busy ? "Saving…" : "Save my answer"}
          </Button>
          {open.length > 1 && (
            <Button variant="link" size="sm" className="px-0" onClick={next}>
              Next question
            </Button>
          )}
        </div>
      </section>
    </div>
  )
}

type Bus = EventTarget & { ready: boolean; queued: string[] }

function Panel({ host, events }: { host: HTMLElement; events: Bus }) {
  const [open, setOpen] = useState(false)
  const [job, setJob] = useState(() => extractJob(document, location.href))
  const [form, setForm] = useState(hasForm)
  const [paired, setPaired] = useState<boolean | null>(null)
  const [server, setServer] = useState("")
  const [app, setApp] = useState<AppRow | undefined>()
  const [busy, setBusy] = useState("")
  const [error, setError] = useState("")
  const [note, setNote] = useState("")
  const [report, setReport] = useState<FillReport | null>(null)
  // Fields still to visit while a fill runs; null once it lands.
  const [left, setLeft] = useState<number | null>(null)
  // The posting this panel just saved, until something else happens: Undo deletes it.
  const [justSaved, setJustSaved] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [dark, setDark] = useState(() => matchMedia("(prefers-color-scheme: dark)").matches)
  const loaded = useRef(false)

  async function load() {
    setError("")
    const p = await new Promise<{ paired: boolean; server: string }>((resolve) =>
      chrome.runtime.sendMessage({ type: "paired" }, (r) => resolve(r || { paired: false, server: "" })),
    )
    setPaired(p.paired)
    setServer(p.server.replace(/\/$/, ""))
    if (!p.paired) return
    try {
      for (const url of candidates(extractJob(document, location.href).url)) {
        try {
          setApp(await call<AppRow>("GET", "/v1/applications/by-url?url=" + encodeURIComponent(url)))
          return
        } catch (err) {
          if ((err as ApiError).code !== "not_found") throw err
        }
      }
      setApp(undefined)
    } catch (err) {
      setError((err as Error).message)
    }
  }

  useEffect(() => {
    const toggle = () => setOpen((o) => !o)
    const show = () => setOpen(true)
    const refresh = () => {
      setJob(extractJob(document, location.href))
      setForm(hasForm())
      setReport(null)
      setJustSaved(false)
      setSubmitted(false)
      setApp(undefined)
      loaded.current = false
    }
    const submit = () => setSubmitted(true)
    const scheme = matchMedia("(prefers-color-scheme: dark)")
    const onScheme = () => setDark(scheme.matches)
    events.addEventListener("toggle", toggle)
    events.addEventListener("open", show)
    events.addEventListener("refresh", refresh)
    document.addEventListener("submit", submit, true)
    scheme.addEventListener("change", onScheme)
    events.ready = true
    events.queued.splice(0).forEach((type) => events.dispatchEvent(new Event(type)))
    return () => {
      events.ready = false
      events.removeEventListener("toggle", toggle)
      events.removeEventListener("open", show)
      events.removeEventListener("refresh", refresh)
      document.removeEventListener("submit", submit, true)
      scheme.removeEventListener("change", onScheme)
    }
  }, [events])

  useEffect(() => {
    if (open && !loaded.current) {
      loaded.current = true
      void load()
    }
  })

  async function act(name: string, fn: () => Promise<void>) {
    setBusy(name)
    setError("")
    setNote("")
    setJustSaved(false)
    try {
      await fn()
    } catch (err) {
      setError((err as Error).message)
      if ((err as ApiError).code === "unpaired") setPaired(false)
    } finally {
      setBusy("")
    }
  }

  const saveJob = () =>
    act("save", async () => {
      const saved = await call<{ application: AppRow; created: boolean }>("POST", "/v1/applications/capture", {
        company: job.company,
        title: job.title,
        location: job.location,
        salary: job.salary,
        posting_url: candidates(job.url).at(-1),
        source: source(job.url),
      })
      setApp(saved.application)
      setJustSaved(saved.created)
    })

  const undoSave = () =>
    act("undo", async () => {
      await call("DELETE", `/v1/applications/${app!.id}?version=${app!.version}`)
      setApp(undefined)
    })

  const markApplied = () =>
    act("applied", async () => {
      setApp(await call<AppRow>("PATCH", `/v1/applications/${app!.id}`, { version: app!.version, status: "applied" }))
      setNote("Marked applied.")
    })

  const autofill = () =>
    act("fill", async () => {
      const profile = await call<{ answers: Record<string, string> }>("GET", "/v1/profile")
      const answers = { ...(profile.answers || {}) }
      if (app) {
        const qs = await call<{ data: { prompt: string; answer: string }[] }>("GET", `/v1/applications/${app.id}/questions`)
        for (const q of qs.data || []) {
          if (!q.answer) continue
          answers[q.prompt] = q.answer
          answers[normalizeKey(q.prompt)] = q.answer
        }
      }
      host.dataset.pursuitState = "filling"
      setReport(null)
      setLeft(0)
      const r = await autofillDocument(document, answers, { resume: await resumeFile(app) }, async (live, rest) => {
        setReport({ ...live, filled: [...live.filled], missed: [...live.missed] })
        setLeft(rest)
        await new Promise((done) => setTimeout(done, PACE_MS))
      })
      setReport(r)
      setLeft(null)
      host.dataset.pursuitState = "filled"
      host.dataset.pursuitFilled = String(r.filled.length)
      host.dataset.pursuitMissed = String(r.missed.length)
      if (app) {
        // The agent running the CLI reads this from the application's events.
        await call("POST", `/v1/applications/${app.id}/events`, {
          kind: "note",
          title: `Autofilled ${r.filled.length} field${r.filled.length === 1 ? "" : "s"}${r.missed.length ? `, ${r.missed.length} left` : ""}`,
          body: [r.attached ? `Attached ${r.attached}.` : "", r.missed.length ? "Left for you: " + r.missed.join("; ") : "Nothing left to answer.", "Not submitted."]
            .filter(Boolean)
            .join("\n"),
        }).catch(() => undefined)
      }
    })

  const action = form ? "Autofill" : "Save job"
  if (!open) {
    // A single-page board navigated away from the posting: hide until the shortcut opens it.
    if (!form && !job.title) return null
    return (
      <div className={dark ? "dark" : ""}>
        <button
          type="button"
          aria-label={`Open Pursuit: ${action}`}
          onClick={() => setOpen(true)}
          className="fixed right-0 bottom-6 flex h-9 cursor-pointer items-center gap-2.5 rounded-l-full bg-[#141615] py-0 pr-3 pl-4 font-sans text-white outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <span className="block h-3.5 w-auto [&>svg]:h-full [&>svg]:w-auto" dangerouslySetInnerHTML={{ __html: wordmark }} />
          <kbd className="rounded border border-[#5d615e] px-1.5 py-0.5 font-sans text-[11px] font-medium text-[#c9cbc7]">Alt+Shift+P</kbd>
        </button>
      </div>
    )
  }

  const missedQuestions = report ? report.fields.filter((f) => (f.getAttribute("type") || "").toLowerCase() !== "file") : []
  const noResume = report?.missed.some((m) => m.includes("add a base résumé"))
  const shown = app ? app.outcome || app.status : ""
  const meta = [job.company, job.location, job.salary].filter(Boolean).join(" · ")

  return (
    <div className={dark ? "dark" : ""}>
      <div role="dialog" aria-label="Pursuit" className="fixed top-4 right-4 max-h-[calc(100vh-32px)] w-[344px] overflow-y-auto font-sans text-foreground">
        <div className="relative">
          <RoughFrame />
          <div className="relative flex flex-col gap-4 px-5 pt-4 pb-5">
            <header className="flex items-center gap-2.5">
              <ProgressMark stage={app ? app.stage ?? statusPieces(app.status) : 0} outcome={app?.outcome} size={26} />
              <span className="flex-1 font-heading text-base font-semibold">Pursuit</span>
              <Button variant="ghost" size="icon" aria-label="Close" onClick={() => setOpen(false)}>
                <X />
              </Button>
            </header>

            {paired === null ? (
              <p className="text-sm text-muted-foreground">Checking your tracker…</p>
            ) : !paired ? (
              <div className="flex flex-col gap-3">
                <p className="text-sm">Pair this browser with your Pursuit server to save jobs and fill forms.</p>
                <Button className="self-start" onClick={() => chrome.runtime.sendMessage({ type: "options" })}>
                  Pair
                </Button>
              </div>
            ) : (
              <>
                <section aria-label="Posting" className="flex flex-col gap-1">
                  {app && server ? (
                    <a href={`${server}/app/view?id=${encodeURIComponent(app.id)}`} target="_blank" rel="noopener" className="group inline-flex items-center gap-2 rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring">
                      <h2 className="font-heading text-xl leading-tight font-semibold group-hover:underline">{job.title || "Application form"}</h2>
                      <ExternalLink className="size-4 shrink-0" aria-hidden />
                      <span className="sr-only">Open in Pursuit</span>
                    </a>
                  ) : (
                    <h2 className="font-heading text-xl leading-tight font-semibold">{job.title || (form ? "Application form" : document.title)}</h2>
                  )}
                  {meta && <p className="text-sm text-muted-foreground">{meta}</p>}
                  <div className="mt-1.5 flex items-center gap-2">
                    {app ? (
                      <>
                        <StatusBadge status={shown} />
                        <span className="text-xs text-muted-foreground">Saved {ago(app.created_at)}</span>
                        {justSaved && (
                          <Button variant="link" size="sm" className="h-auto px-0 text-xs" disabled={busy === "undo"} onClick={undoSave}>
                            Undo
                          </Button>
                        )}
                      </>
                    ) : (
                      <Button variant="outline" size="sm" disabled={busy === "save" || !job.title} onClick={saveJob}>
                        {busy === "save" ? "Saving…" : "Save job"}
                      </Button>
                    )}
                  </div>
                </section>

                {form && (
                  <section aria-label="Application form" className="flex flex-col gap-4">
                    <Button size="lg" className="h-11 text-[15px]" variant={report ? "outline" : "default"} disabled={busy === "fill"} onClick={autofill}>
                      {busy === "fill" ? "Filling…" : report ? "Fill again" : "Autofill this form"}
                    </Button>
                    {(report || left !== null) && <Roll report={report || { filled: [], missed: [], fields: [] }} left={left} />}
                    {noResume && <p className="text-xs text-muted-foreground">No résumé to attach. Add a base résumé in Pursuit.</p>}
                    {report && left === null && missedQuestions.length > 0 && <LeftForYou key={report.missed.join("|")} fields={missedQuestions} onSaved={setNote} />}
                    {submitted && app && !["approved", "submitting", "submitted", "applied"].includes(app.status) && (
                      <div className="flex items-center gap-3 text-sm">
                        <span className="flex-1">Sent it?</span>
                        <Button size="sm" disabled={busy === "applied"} onClick={markApplied}>
                          Mark applied
                        </Button>
                      </div>
                    )}
                  </section>
                )}
              </>
            )}

            {(error || note) && (
              <p role="status" aria-live="polite" className={`text-sm ${error ? "text-destructive" : "text-muted-foreground"}`}>
                {error || note}
              </p>
            )}
            <p className="text-xs text-muted-foreground">Review before it’s sent. Pursuit never clicks Submit.</p>
          </div>
        </div>
      </div>
    </div>
  )
}

function fonts() {
  if (document.getElementById("pursuit-fonts")) return
  const face = (family: string, file: string) =>
    `@font-face{font-family:"${family}";src:url("${chrome.runtime.getURL("fonts/" + file)}") format("woff2");font-weight:100 900;font-display:swap}`
  const style = document.createElement("style")
  style.id = "pursuit-fonts"
  style.textContent = face("Pursuit Inter", "inter.woff2") + face("Pursuit Outfit", "outfit.woff2")
  document.head?.appendChild(style)
}

export function mount() {
  fonts()
  const host = document.createElement("div")
  host.setAttribute("data-pursuit-panel", "")
  host.dataset.pursuitState = "idle"
  const shadow = host.attachShadow({ mode: "open" })
  const style = document.createElement("style")
  style.textContent = __PANEL_CSS__
  const root = document.createElement("div")
  shadow.append(style, root)
  document.documentElement.appendChild(host)
  // Calls that arrive before React has rendered wait in a queue the panel drains on mount.
  const events = Object.assign(new EventTarget(), { ready: false, queued: [] as string[] })
  createRoot(root).render(<Panel host={host} events={events} />)
  const fire = (type: string) => (events.ready ? events.dispatchEvent(new Event(type)) : events.queued.push(type))
  return { toggle: () => fire("toggle"), open: () => fire("open"), refresh: () => fire("refresh") }
}
