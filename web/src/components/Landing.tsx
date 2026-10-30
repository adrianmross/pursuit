import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { buttonVariants } from "@/components/ui/button"
import { cn } from "@/lib/utils"

// Scripted session, not real daemon output. It stops at `approved` because this
// build enqueues submit work but never clicks Submit.
const SCRIPT: { text: string; kind: "cmd" | "out" | "note" }[] = [
  { kind: "cmd", text: "pursuit list --status to_apply --fields id,company,title" },
  { kind: "out", text: "a_4f2  Halcyon      Staff Platform Engineer" },
  { kind: "out", text: "a_91c  Northwind    Backend Engineer" },
  { kind: "cmd", text: "pursuit apply a_4f2 --async" },
  { kind: "out", text: "preparing → awaiting_review   (form filled from your standing answers)" },
  { kind: "note", text: "agent stops here — you decide" },
  { kind: "cmd", text: "pursuit approve a_4f2 --note 'good fit'" },
  { kind: "out", text: "approved · submit work enqueued" },
]

// Pieces lit per pipeline step: to_apply 1 … offer 6 (the gold tail).
const STAGES = ["To apply", "Prepared", "Approved", "Submitted", "Interviewing", "Offer"]

function Pieces({ lit }: { lit: number }) {
  return (
    <span className="inline-flex gap-[3px]" aria-hidden>
      {STAGES.map((_, i) => (
        <i
          key={i}
          className={cn(
            "size-2 rounded-full transition-colors",
            i < lit ? (i === 5 ? "bg-brand-gold" : "bg-current") : "bg-current/20",
          )}
        />
      ))}
    </span>
  )
}

function Terminal() {
  const [lines, setLines] = useState<string[]>([])
  const [typing, setTyping] = useState("")
  const [lit, setLit] = useState(1)

  useEffect(() => {
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches
    let dead = false
    const sleep = (ms: number) => new Promise((r) => setTimeout(r, reduce ? 0 : ms))
    async function run() {
      while (!dead) {
        setLines([])
        setLit(1)
        for (const { kind, text } of SCRIPT) {
          if (dead) return
          if (kind === "cmd") {
            for (let c = 1; c <= text.length && !dead; c++) {
              setTyping(text.slice(0, c))
              await sleep(22)
            }
            await sleep(250)
            setTyping("")
            setLines((l) => [...l, "$ " + text])
          } else {
            await sleep(kind === "note" ? 700 : 350)
            setLines((l) => [...l, (kind === "note" ? "# " : "  ") + text])
          }
          if (text.includes("awaiting_review")) setLit(2)
          if (text.startsWith("approved")) setLit(3)
        }
        if (reduce) return
        await sleep(4000)
      }
    }
    void run()
    return () => {
      dead = true
    }
  }, [])

  return (
    <div
      role="img"
      aria-label="Demo: an agent prepares an application, then a human approves it"
      className="mx-auto w-full max-w-2xl overflow-hidden rounded-xl border border-white/10 bg-[#0e0f0f] text-left text-[#e6e7e4]"
    >
      <div className="flex items-center gap-1.5 border-b border-white/10 px-4 py-2.5 text-xs">
        <span className="size-2.5 rounded-full bg-white/15" />
        <span className="size-2.5 rounded-full bg-white/15" />
        <span className="size-2.5 rounded-full bg-white/15" />
        <b className="ml-2 font-medium opacity-60">pursuit</b>
        <span className="ml-auto">
          <Pieces lit={lit} />
        </span>
      </div>
      <pre aria-hidden className="min-h-60 whitespace-pre-wrap px-4 py-4 font-mono text-[13px] leading-7">
        {lines.map((l, i) => (
          <div key={i} className={l[0] === "#" ? "text-brand-gold" : l[0] === " " ? "text-[#a3a7a4]" : ""}>
            {l}
          </div>
        ))}
        {(typing !== "" || lines.length === 0) && (
          <div>
            $ {typing}
            <span className="ml-0.5 inline-block h-4 w-2 translate-y-0.5 animate-pulse bg-[#e6e7e4] motion-reduce:animate-none" />
          </div>
        )}
      </pre>
    </div>
  )
}

const FEATURES: [string, string][] = [
  ["CLI-first for agents", "Compact JSON on stdout, JSON errors on stderr, typed exit codes, --dry-run and idempotency keys. No MCP server to run."],
  ["Approval is a human act", "An approve-scoped token is the only way past review. Agents prepare; you decide."],
  ["Standing answers", "Answer a form question once. Unknown required questions stop at needs_input instead of guessing."],
  ["Link health", "A daily check marks closed postings stale so you stop preparing applications for roles that are gone."],
  ["Company caps", "Limit active applications per company over a window. Exceeding a cap is a conflict, not a surprise."],
  ["One server, three doors", "Dashboard, CLI and HTTP API share one daemon on SQLite, or Postgres for more replicas."],
]

const link = "text-sm font-medium text-foreground/80 hover:text-foreground"

export function Landing() {
  return (
    <div className="min-h-svh bg-background text-foreground">
      <header className="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
        <Link to="/" aria-label="Hire Pursuit">
          <img src="/brand/hire-pursuit-wordmark.svg" alt="Hire Pursuit" className="h-6 w-auto dark:hidden" />
          <img src="/brand/hire-pursuit-wordmark-reverse.svg" alt="" className="hidden h-6 w-auto dark:block" />
        </Link>
        <nav className="flex items-center gap-5">
          <a href="/docs/" className={link}>
            Docs
          </a>
          <a href="/reference" className={link}>
            Reference
          </a>
          <Link to="/login" className={buttonVariants({ size: "sm" })}>
            Log in
          </Link>
        </nav>
      </header>

      <main className="mx-auto max-w-6xl px-5 pb-20 pt-12">
        <section className="text-center">
          <img src="/brand/hire-pursuit-mark.svg" alt="" className="mx-auto mb-5 h-16 w-auto dark:hidden" />
          <img src="/brand/hire-pursuit-mark-reverse.svg" alt="" className="mx-auto mb-5 hidden h-16 w-auto dark:block" />
          <h1 className="font-heading text-5xl font-semibold leading-none tracking-tight sm:text-6xl">
            For your higher pursuits.
          </h1>
          <p className="mx-auto mt-5 max-w-xl text-lg text-muted-foreground">
            Hire Pursuit is a self-hosted job-application tracker for one person. Agents prepare applications through
            the <code className="font-mono text-base">pursuit</code> CLI. Nothing goes out until you approve it.
          </p>
          <div className="mb-12 mt-8 flex flex-wrap justify-center gap-3">
            <Link to="/login" className={buttonVariants({ size: "lg" })}>
              Log in
            </Link>
            <a href="/docs/getting-started" className={buttonVariants({ size: "lg", variant: "outline" })}>
              Get started
            </a>
          </div>
          <Terminal />
          <ol className="mt-8 flex flex-wrap justify-center gap-x-8 gap-y-5 text-sm text-muted-foreground">
            {STAGES.map((s, i) => (
              <li key={s} className="flex flex-col items-center gap-2">
                <Pieces lit={i + 1} />
                {s}
              </li>
            ))}
          </ol>
        </section>

        <section className="mt-20 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {FEATURES.map(([title, body]) => (
            <div key={title} className="rounded-[10px] border bg-card p-5">
              <h3 className="font-heading font-semibold">{title}</h3>
              <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">{body}</p>
            </div>
          ))}
        </section>
      </main>
    </div>
  )
}
