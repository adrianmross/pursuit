"use client"

import { useEffect, useState } from "react"

// Scripted session. "cmd" lines are typed; the rest print at once. The run ends at
// `approved` because this build enqueues submit work but never clicks Submit.
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

// Pieces lit per step of the pipeline: to_apply 1 … offer 6 (the gold tail).
const STAGES = ["To apply", "Prepared", "Approved", "Submitted", "Interviewing", "Offer"]

function Pieces({ lit }: { lit: number }) {
  return (
    <span className="wl-pieces" aria-hidden>
      {STAGES.map((_, i) => (
        <i key={i} data-on={i < lit} data-tail={i === 5} />
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
        for (let i = 0; i < SCRIPT.length && !dead; i++) {
          const { kind, text } = SCRIPT[i]
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
    run()
    return () => {
      dead = true
    }
  }, [])

  return (
    <div className="wl-term" role="img" aria-label="Demo: an agent prepares an application, then a human approves it">
      <div className="wl-term-bar">
        <span /> <span /> <span />
        <b>pursuit</b>
        <Pieces lit={lit} />
      </div>
      <pre aria-hidden>
        {lines.map((l, i) => (
          <div key={i} data-k={l[0]}>
            {l}
          </div>
        ))}
        {typing !== "" || lines.length === 0 ? (
          <div data-k="$">
            $ {typing}
            <span className="wl-caret" />
          </div>
        ) : null}
      </pre>
    </div>
  )
}

const FEATURES = [
  ["CLI-first for agents", "Compact JSON on stdout, JSON errors on stderr, typed exit codes, --dry-run and idempotency keys. No MCP server to run."],
  ["Approval is a human act", "An approve-scoped token is the only way past review. Agents prepare; you decide. Never put approve on an agent token."],
  ["Standing answers", "Answer a form question once. Unknown required questions stop at needs_input instead of guessing."],
  ["Link health", "A daily check marks closed postings stale so you stop preparing applications for roles that are gone."],
  ["Company caps", "Limit active applications per company over a window. Exceeding a cap is a conflict, not a surprise."],
  ["One server, three doors", "Dashboard, CLI and HTTP API share one daemon on SQLite, or Postgres for more replicas."],
]

export function Welcome() {
  return (
    <div className="wl">
      <section className="wl-hero">
        <img className="wl-mark" src="/docs/brand/hire-pursuit-app-icon.svg" alt="" />
        <h1>For your higher pursuits.</h1>
        <p>
          Hire Pursuit is a self-hosted job-application tracker for one person. Agents prepare applications through the{" "}
          <code>pursuit</code> CLI. Nothing goes out until you approve it.
        </p>
        <div className="wl-cta">
          <a className="wl-btn wl-btn-primary" href="/docs/getting-started">
            Get started
          </a>
          <a className="wl-btn" href="/docs/cli">
            CLI reference
          </a>
          <a className="wl-btn" href="/docs/api-reference">
            API reference
          </a>
        </div>
        <Terminal />
        <ol className="wl-stages">
          {STAGES.map((s, i) => (
            <li key={s}>
              <Pieces lit={i + 1} />
              {s}
            </li>
          ))}
        </ol>
      </section>

      <section className="wl-grid">
        {FEATURES.map(([t, d]) => (
          <div key={t}>
            <h3>{t}</h3>
            <p>{d}</p>
          </div>
        ))}
      </section>

      <section className="wl-steps">
        <h2>Prepare, approve, submit</h2>
        <div className="wl-grid">
          <div><h3>1 · Prepare</h3><p>An agent with a <code>write</code> token takes a role from <code>to_apply</code> to <code>awaiting_review</code>.</p></div>
          <div><h3>2 · Approve</h3><p>You review in the dashboard or run <code>pursuit approve</code>. It only enqueues the submit.</p></div>
          <div><h3>3 · Submit</h3><p>A worker claims the queued work and reports <code>submitted</code>, <code>needs_input</code> or <code>failed</code>.</p></div>
        </div>
      </section>
    </div>
  )
}
