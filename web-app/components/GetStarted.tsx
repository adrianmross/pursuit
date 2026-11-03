"use client"

import { useEffect, useState, useSyncExternalStore, type ReactNode } from "react"
import Link from "next/link"
import { ArrowRight, BookOpen, Bot, Download, Puzzle, RotateCcw, Sparkles, Terminal, X } from "lucide-react"
import { api, serverUrl } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { setPref, type Pref } from "@/lib/settings"
import { usePref } from "@/lib/use-pref"
import type { Document, Page, StandingAnswer } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { CommandBlock } from "@/components/CommandBlock"
import { Die, Sketch, Track } from "@/components/Sketch"

const noop = () => () => {}
const HINTS: Pref[] = ["hintProfile", "hintAgent", "hintTerminal", "hintExtension"]

type Tip = { pref: Pref; icon: ReactNode; title: string; description?: ReactNode; action: ReactNode; footer?: ReactNode; stays?: boolean }

// One card in the Get started deck, like Robinhood's news cards: icon, title,
// a line or two, and its action along the bottom.
function TipCard({ tip, depth, leaving, onDismiss }: { tip: Tip; depth: number; leaving: boolean; onDismiss: () => void }) {
  const d = Math.min(depth, 2)
  return (
    <div
      inert={depth > 0}
      aria-hidden={depth > 0}
      style={{
        gridArea: "1 / 1",
        zIndex: 10 - depth,
        // The cards behind sit lower and narrower, so their bottom edges peek out under the top one.
        transform: leaving ? "translateX(105%) rotate(6deg)" : `translateY(${d * 10}px) scale(${1 - d * 0.04})`,
        opacity: leaving || depth > 2 ? 0 : 1,
      }}
      className="relative flex h-32 origin-bottom flex-col gap-2.5 rounded-xl bg-card p-4 text-sm shadow-sm ring-1 ring-foreground/10 transition-[transform,opacity] duration-300 ease-out"
    >
      <Button size="icon-sm" variant="ghost" className="absolute top-2 right-2 text-muted-foreground" aria-label={`Dismiss ${tip.title}`} onClick={onDismiss}>
        <X />
      </Button>
      <div className="flex items-center gap-2.5 pr-8">
        <span className="flex size-7 shrink-0 items-center justify-center rounded-lg bg-muted [&_svg]:size-3.5">{tip.icon}</span>
        <p className="min-w-0 flex-1 truncate font-medium">{tip.title}</p>
        {tip.action}
      </div>
      {tip.footer ?? <p className="line-clamp-2 text-muted-foreground">{tip.description}</p>}
    </div>
  )
}

// What's left to set up: a résumé and email on file, an agent, the CLI and the
// extension. Each row is dismissed per browser.
export function GetStarted() {
  const { can } = useAuth()
  const server = useSyncExternalStore(noop, serverUrl, () => "")
  const profileOpen = usePref("hintProfile") === "on"
  const [missing, setMissing] = useState<string[]>([])
  const [leaving, setLeaving] = useState<Pref | null>(null)
  useEffect(() => {
    if (!profileOpen || !can("write")) return
    void Promise.all([api<Page<Document>>("/v1/documents"), api<Page<StandingAnswer>>("/v1/standing")])
      .then(([docs, standing]) => {
        const out = []
        if (!docs.data?.some((d) => d.kind === "base_resume")) out.push("a résumé")
        if (!standing.data?.some((s) => s.key === "email")) out.push("your contact details")
        setMissing(out)
      })
      .catch(() => {})
  }, [profileOpen, can])

  const prefs: Record<string, boolean> = {
    hintProfile: profileOpen,
    hintAgent: usePref("hintAgent") === "on",
    hintTerminal: usePref("hintTerminal") === "on",
    hintExtension: usePref("hintExtension") === "on",
  }
  const tips: Tip[] = [
    ...(missing.length
      ? [
          {
            pref: "hintProfile" as Pref,
            icon: <Sparkles />,
            title: "Finish your profile",
            description: `Add ${missing.join(" and ")} so prepared applications fill themselves in.`,
            action: (
              <Button size="sm" variant="outline" asChild>
                <Link href={missing.includes("a résumé") ? "/profile" : "/profile/questions"}>
                  Set up <ArrowRight />
                </Link>
              </Button>
            ),
          },
        ]
      : []),
    {
      pref: "hintAgent",
      icon: <Bot />,
      title: "Connect an agent",
      description: "Copy one prompt into your coding agent. It installs the CLI and signs in with a code you type here.",
      action: (
        <Button size="sm" variant="outline" asChild>
          <Link href="/agents">
            Set up <ArrowRight />
          </Link>
        </Button>
      ),
    },
    {
      pref: "hintTerminal",
      stays: true,
      icon: <Terminal />,
      title: "Use the terminal",
      // The command is the description: install it, then sign in to this server.
      footer: <CommandBlock command={`brew install adrianmross/tap/pursuit\npursuit auth login${server ? ` --server ${server}` : ""}`} />,
      action: (
        <Button variant="outline" size="sm" asChild>
          <a href="https://hire-pursuit.app/docs" target="_blank" rel="noopener noreferrer">
            <BookOpen /> Docs
          </a>
        </Button>
      ),
    },
    {
      pref: "hintExtension",
      stays: true,
      icon: <Puzzle />,
      title: "Browser extension",
      description: "Save the posting you have open and autofill Greenhouse, Ashby, and Lever forms from this server.",
      action: (
        <Button variant="outline" size="sm" asChild>
          <a href="https://github.com/adrianmross/pursuit#browser-extension" target="_blank" rel="noopener noreferrer">
            <Download /> Get it
          </a>
        </Button>
      ),
    },
  ]
  const open = tips.filter((t) => prefs[t.pref])
  // A dismissed card that stays becomes a chip; clicking it brings the card back.
  const kept = tips.filter((t) => !prefs[t.pref] && t.stays)
  const gone = open.findIndex((t) => t.pref === leaving)
  // The top card flies off to the right first, and the rest move up under it.
  function dismiss(pref: Pref) {
    setLeaving(pref)
    setTimeout(() => {
      setPref(pref, "off")
      setLeaving(null)
    }, 300)
  }

  return (
    <div className="flex flex-col gap-3">
      <h2 className="flex text-sm font-medium text-muted-foreground">
        Get started
        {open.length > 1 && <span className="ml-auto font-normal tabular-nums">{open.length - (leaving ? 1 : 0)} left</span>}
      </h2>
      {open.length > 0 ? (
        <div className="grid overflow-x-clip pb-5">
          {open.map((t, i) => (
            <TipCard key={t.pref} tip={t} depth={gone >= 0 && i > gone ? i - 1 : i} leaving={t.pref === leaving} onDismiss={() => dismiss(t.pref)} />
          ))}
        </div>
      ) : (
        <AllClear />
      )}
      {open.length < tips.length && (
        // Quiet rows like the calendar's day list, muted until hovered. The last brings every card back.
        <div className="flex flex-col gap-0.5">
          {[...kept, { pref: "all", icon: <RotateCcw />, title: "See them all again" } as const].map((t) => (
            <button
              key={t.pref}
              type="button"
              onClick={() => (t.pref === "all" ? HINTS : [t.pref]).forEach((p) => setPref(p, "on"))}
              className="flex items-center gap-2.5 rounded-md px-1.5 py-1 text-left text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground [&_svg]:size-3.5"
            >
              {t.icon}
              <span className="flex-1">{t.title}</span>
              <span className="text-xs text-muted-foreground/70">Show</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

// The deck is empty: the cards squared up in a pile, a gold tick drawn on the
// top one, and a die rolling in to settle on six. Drawn like the error pages.
function AllClear() {
  return (
    <div className="flex flex-col items-center gap-1 py-4 text-center">
      <Sketch viewBox="0 0 200 104" roughness={2} className="w-48 text-foreground">
        <Track y={88} count={10} />
        <g strokeWidth="3" stroke="currentColor">
          <rect x="62" y="22" width="60" height="44" rx="6" transform="rotate(-8 92 44)" className="fill-background" />
          <rect x="66" y="18" width="60" height="44" rx="6" transform="rotate(4 96 40)" className="fill-background" />
          <rect x="64" y="16" width="60" height="44" rx="6" className="fill-background" />
        </g>
        <path d="M80 38 L91 48 L110 28" pathLength={1} stroke="var(--brand-gold)" strokeWidth="6" className="animate-[draw-tick_500ms_ease-out_250ms_both] [stroke-dasharray:1]" />
        <g className="animate-[roll-in_900ms_cubic-bezier(.3,1.4,.5,1)_500ms_both] [transform-box:fill-box] [transform-origin:center]">
          <Die x={160} y={70} size={22} tilt={12} pips={6} />
        </g>
      </Sketch>
      <p className="font-medium">Nothing more to see</p>
      <p className="text-sm text-muted-foreground">You&apos;re all set up.</p>    </div>
  )
}
