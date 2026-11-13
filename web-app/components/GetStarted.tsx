"use client"

import { useEffect, useState, useSyncExternalStore, type ReactNode } from "react"
import Link from "next/link"
import { ArrowRight, BookOpen, Bot, Download, Megaphone, Puzzle, RotateCcw, Sparkles, Terminal, X } from "lucide-react"
import { api, serverUrl } from "@adrianmross/appkit-ui"
import { useAnnouncement } from "@adrianmross/appkit-ui"
import { latest } from "@/lib/announcements"
import { useAuth } from "@adrianmross/appkit-ui"
import { setPref, type Pref } from "@adrianmross/appkit-ui"
import { usePref } from "@adrianmross/appkit-ui"
import type { Document, Page, StandingAnswer } from "@/lib/types"
import { Button } from "@adrianmross/appkit-ui/ui/button"
import { CommandBlock } from "@/components/CommandBlock"
import { Die, Sketch, Track } from "@/components/Sketch"

const noop = () => () => {}
const HINTS: Pref[] = ["hintProfile", "hintAgent", "hintTerminal", "hintExtension"]

// "announcement" is the newest What's new item, dismissed on the account rather than per browser.
type Tip = { pref: Pref | "announcement"; icon: ReactNode; art: ReactNode; title: string; description?: ReactNode; action: ReactNode; footer?: ReactNode; stays?: boolean }

// One card in the start page feed, like Robinhood's news cards: icon, title,
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
      <div className="flex h-full gap-4">
        <div className="w-16 shrink-0 self-center">{tip.art}</div>
        <div className="flex min-w-0 flex-1 flex-col gap-2.5">
          <div className="flex items-center gap-2.5 pr-8">
            <p className="min-w-0 flex-1 truncate font-medium">{tip.title}</p>
            {tip.action}
          </div>
          {tip.footer ?? <p className="line-clamp-2 text-muted-foreground">{tip.description}</p>}
        </div>
      </div>
    </div>
  )
}

// Each card's drawing, in the error pages' hand-drawn style: plain strokes in
// currentColor with the brand gold and forest, roughened by <Sketch>.
const ART_STROKE = { stroke: "currentColor", strokeWidth: 2.5, strokeLinecap: "round", strokeLinejoin: "round" } as const

// A résumé with a head-and-shoulders photo and a gold sparkle.
function ProfileArt() {
  return (
    <Sketch viewBox="0 0 64 64" roughness={1.5} className="w-full">
      <g {...ART_STROKE} transform="rotate(-6 32 34)">
        <path d="M16 8 H40 L50 18 V58 H16 Z" className="fill-background" />
        <path d="M40 8 V18 H50" />
        <circle cx="27" cy="25" r="5" />
        <path d="M19 38 C19 31 35 31 35 38" />
        <path d="M21 46 H45 M21 52 H38" />
      </g>
      <path d="M52 4 V14 M47 9 H57" stroke="var(--brand-gold)" strokeWidth="3" strokeLinecap="round" />
    </Sketch>
  )
}

// A pawn that's turned robot: two eyes and a gold-tipped antenna, on a forest base.
function AgentArt() {
  return (
    <Sketch viewBox="0 0 64 64" roughness={1.5} className="w-full">
      <g {...ART_STROKE}>
        <path d="M32 14 V7" />
        <path d="M24 34 C22 42 18 48 16 54 H48 C46 48 42 42 40 34" className="fill-background" />
        <rect x="20" y="14" width="24" height="20" rx="7" className="fill-background" />
        <path d="M14 54 H50" stroke="var(--brand-forest)" strokeWidth="5" />
      </g>
      <circle cx="32" cy="6" r="3.5" fill="var(--brand-gold)" />
      <circle cx="27" cy="24" r="2.2" fill="currentColor" />
      <circle cx="37" cy="24" r="2.2" fill="currentColor" />
    </Sketch>
  )
}

// A terminal window with a prompt and a blinking gold cursor.
function TerminalArt() {
  return (
    <Sketch viewBox="0 0 64 64" roughness={1.5} className="w-full">
      <g {...ART_STROKE} transform="rotate(4 32 32)">
        <rect x="6" y="12" width="52" height="40" rx="6" className="fill-background" />
        <path d="M6 22 H58" />
        <path d="M15 31 L22 37 L15 43" />
      </g>
      <g transform="rotate(4 32 32)">
        <circle cx="12" cy="17" r="1.6" fill="currentColor" />
        <circle cx="17.5" cy="17" r="1.6" fill="currentColor" />
        <path d="M27 43 H38" stroke="var(--brand-gold)" strokeWidth="3.5" strokeLinecap="round" className="animate-pulse" />
      </g>
    </Sketch>
  )
}

// A gold puzzle piece clicking into a browser window.
function ExtensionArt() {
  return (
    <Sketch viewBox="0 0 64 64" roughness={1.5} className="w-full">
      <g {...ART_STROKE}>
        <rect x="4" y="10" width="44" height="36" rx="5" className="fill-background" />
        <path d="M4 19 H48" />
        <path
          d="M28 30 H35 A4.5 4.5 0 1 1 44 30 H51 V37 A4.5 4.5 0 1 1 51 46 V54 H28 V46 A4.5 4.5 0 1 0 28 37 Z"
          fill="var(--brand-gold)"
          transform="rotate(10 40 42)"
        />
      </g>
    </Sketch>
  )
}

// A megaphone with gold sound waves.
function AnnouncementArt() {
  return (
    <Sketch viewBox="0 0 64 64" roughness={1.5} className="w-full">
      <g {...ART_STROKE} transform="rotate(-8 32 34)">
        <path d="M10 26 H20 L40 14 V50 L20 38 H10 Z" className="fill-background" />
        <path d="M20 38 L24 54 H31 L28 41" className="fill-background" />
      </g>
      <path d="M47 24 C51 29 51 35 47 40" stroke="var(--brand-gold)" strokeWidth="3.5" strokeLinecap="round" fill="none" />
      <path d="M53 17 C60 26 60 38 53 47" stroke="var(--brand-gold)" strokeWidth="3.5" strokeLinecap="round" fill="none" />
    </Sketch>
  )
}

// What's left to set up: a résumé and email on file, an agent, the CLI and the
// extension. Each row is dismissed per browser.
export function GetStarted() {
  const { can } = useAuth()
  const server = useSyncExternalStore(noop, serverUrl, () => "")
  const profileOpen = usePref("hintProfile") === "on"
  const [missing, setMissing] = useState<string[]>([])
  const announcement = useAnnouncement(latest)
  const [leaving, setLeaving] = useState<string | null>(null)
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
    announcement: !!announcement.item,
    hintProfile: profileOpen,
    hintAgent: usePref("hintAgent") === "on",
    hintTerminal: usePref("hintTerminal") === "on",
    hintExtension: usePref("hintExtension") === "on",
  }
  const tips: Tip[] = [
    ...(announcement.item
      ? [
          {
            pref: "announcement" as const,
            icon: <Megaphone />,
            art: <AnnouncementArt />,
            title: announcement.item.title,
            description: announcement.item.body,
            action: (
              <Button size="sm" variant="outline" asChild>
                <a href={announcement.item.href}>
                  What&apos;s new <ArrowRight />
                </a>
              </Button>
            ),
          },
        ]
      : []),
    ...(missing.length
      ? [
          {
            pref: "hintProfile" as Pref,
            icon: <Sparkles />,
            art: <ProfileArt />,
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
      art: <AgentArt />,
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
      art: <TerminalArt />,
      title: "Use the terminal",
      // The command is the description: install it, then sign in to this server.
      footer: <CommandBlock command={`brew install adrianmross/tap/pursuit\npursuit auth login${server ? ` --server ${server}` : ""}`} />,
      action: (
        <Button variant="outline" size="sm" asChild>
          <a href="https://hirepursuit.app/docs" target="_blank" rel="noopener noreferrer">
            <BookOpen /> Docs
          </a>
        </Button>
      ),
    },
    {
      pref: "hintExtension",
      stays: true,
      icon: <Puzzle />,
      art: <ExtensionArt />,
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
  function dismiss(pref: Tip["pref"]) {
    setLeaving(pref)
    setTimeout(() => {
      if (pref === "announcement") announcement.dismiss()
      else setPref(pref, "off")
      setLeaving(null)
    }, 300)
  }

  return (
    <div className="flex flex-col gap-3">
      <h2 className="flex text-sm font-medium text-muted-foreground">
        Feed
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
      {(kept.length > 0 || !open.length) && (
        // Small muted buttons: each kept card brings itself back, and once the deck is empty, See them again brings them all.
        <div className={`-mt-1 flex flex-wrap gap-1 ${open.length ? "" : "justify-center"}`}>
          {kept.map((t) => (
            <Button key={t.pref} variant="ghost" size="sm" className="text-muted-foreground" onClick={() => setPref(t.pref as Pref, "on")}>
              {t.icon} {t.title}
            </Button>
          ))}
          {!open.length && (
            <Button variant="ghost" size="sm" className="text-muted-foreground" onClick={() => HINTS.forEach((p) => setPref(p, "on"))}>
              <RotateCcw /> See them again
            </Button>
          )}
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
      <p className="text-sm text-muted-foreground">You&apos;re all set up.</p>
    </div>
  )
}
