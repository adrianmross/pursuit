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
import { Item, ItemActions, ItemContent, ItemDescription, ItemFooter, ItemGroup, ItemMedia, ItemTitle } from "@/components/ui/item"
import { CommandBlock } from "@/components/CommandBlock"

const noop = () => () => {}

// One row of Get started. Dismissing a row with `stays` shrinks it to its
// title and action; any other row goes away.
function Row({ pref, icon, title, description, action, footer, stays }: { pref: Pref; icon: ReactNode; title: string; description: ReactNode; action: ReactNode; footer?: ReactNode; stays?: boolean }) {
  const open = usePref(pref) === "on"
  if (!open && !stays) return null
  return (
    <Item variant="outline" size={open ? "default" : "xs"}>
      <ItemMedia variant="icon">{icon}</ItemMedia>
      <ItemContent>
        <ItemTitle>{title}</ItemTitle>
        {open && <ItemDescription>{description}</ItemDescription>}
      </ItemContent>
      <ItemActions>
        {action}
        {open && (
          <Button size="icon-sm" variant="ghost" aria-label={stays ? `Shrink ${title}` : `Dismiss ${title}`} onClick={() => setPref(pref, "off")}>
            <X />
          </Button>
        )}
      </ItemActions>
      {open && footer && <ItemFooter className="block">{footer}</ItemFooter>}
    </Item>
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

  return (
    <div className="flex flex-col gap-3">
      <h2 className="text-sm font-medium text-muted-foreground">Get started</h2>
      <ItemGroup className="gap-2">
        {missing.length > 0 && (
          <Row
            pref="hintProfile"
            icon={<Sparkles />}
            title="Finish your profile"
            description={`Add ${missing.join(" and ")} so prepared applications fill themselves in.`}
            action={
              <Button size="sm" asChild>
                <Link href={missing.includes("a résumé") ? "/profile" : "/profile/questions"}>
                  Set up <ArrowRight />
                </Link>
              </Button>
            }
          />
        )}
        <Row
          pref="hintAgent"
          icon={<Bot />}
          title="Connect an agent"
          description="Copy one prompt into your coding agent. It installs the CLI and signs in with a code you type here."
          action={
            <Button size="sm" variant="outline" asChild>
              <Link href="/agents">
                Set up <ArrowRight />
              </Link>
            </Button>
          }
        />
        <Row
          pref="hintTerminal"
          stays
          icon={<Terminal />}
          title="Use the terminal"
          description={
            <>
              Everything here is also a <code className="font-mono text-foreground">pursuit</code> command. Install it and sign in to this server.
            </>
          }
          action={
            <Button variant="outline" size="sm" asChild>
              <a href="https://hire-pursuit.app/docs" target="_blank" rel="noopener noreferrer">
                <BookOpen /> Docs
              </a>
            </Button>
          }
          footer={<CommandBlock command={`brew install adrianmross/tap/pursuit\npursuit auth login${server ? ` --server ${server}` : ""}`} />}
        />
        <Row
          pref="hintExtension"
          stays
          icon={<Puzzle />}
          title="Browser extension"
          description="Save the posting you have open and autofill Greenhouse, Ashby, and Lever application forms from this server."
          action={
            <Button variant="outline" size="sm" asChild>
              <a href="https://github.com/adrianmross/pursuit#browser-extension" target="_blank" rel="noopener noreferrer">
                <Download /> Get it
              </a>
            </Button>
          }
        />
      </ItemGroup>
    </div>
  )
}
