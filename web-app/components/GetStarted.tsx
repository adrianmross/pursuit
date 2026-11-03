"use client"

import { useEffect, useState, useSyncExternalStore, type ReactNode } from "react"
import Link from "next/link"
import { ArrowRight, BookOpen, Bot, Download, Puzzle, Sparkles, Terminal, X } from "lucide-react"
import { api, serverUrl } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { setPref, type Pref } from "@/lib/settings"
import { usePref } from "@/lib/use-pref"
import type { Document, Page, StandingAnswer } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { CommandBlock } from "@/components/CommandBlock"

const noop = () => () => {}

type Tip = { pref: Pref; icon: ReactNode; title: string; description: ReactNode; action: ReactNode; footer?: ReactNode; stays?: boolean }

// One card in the Get started row, like Robinhood's news cards: icon, title,
// a line or two, and its action along the bottom.
function TipCard({ tip }: { tip: Tip }) {
  return (
    <div className={`${tip.footer ? "w-[26rem]" : "w-72"} relative flex shrink-0 flex-col gap-2 rounded-xl bg-card p-4 text-sm ring-1 ring-foreground/10`}>
      <Button size="icon-sm" variant="ghost" className="absolute top-2 right-2 text-muted-foreground" aria-label={`Dismiss ${tip.title}`} onClick={() => setPref(tip.pref, "off")}>
        <X />
      </Button>
      <span className="flex size-8 items-center justify-center rounded-lg bg-muted [&_svg]:size-4">{tip.icon}</span>
      <p className="pr-6 font-medium">{tip.title}</p>
      <p className="text-muted-foreground">{tip.description}</p>
      {tip.footer}
      <div className="mt-auto flex pt-1">{tip.action}</div>
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
              <Button size="sm" asChild>
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
      description: "Everything here is also a command. Install it and sign in to this server.",
      footer: <CommandBlock command={`brew install adrianmross/tap/pursuit\npursuit auth login${server ? ` \\\n  --server ${server}` : ""}`} />,
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
  if (!open.length && !kept.length) return null

  return (
    <div className="flex flex-col gap-3">
      <h2 className="text-sm font-medium text-muted-foreground">Get started</h2>
      {open.length > 0 && (
        // No scroll snap: it re-snaps to the old card when the profile card loads in front of it.
        <div className="flex gap-3 overflow-x-auto p-px [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
          {open.map((t) => (
            <TipCard key={t.pref} tip={t} />
          ))}
        </div>
      )}
      {kept.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {kept.map((t) => (
            <Button key={t.pref} variant="outline" size="sm" className="rounded-full" onClick={() => setPref(t.pref, "on")}>
              {t.icon} {t.title}
            </Button>
          ))}
        </div>
      )}
    </div>
  )
}
