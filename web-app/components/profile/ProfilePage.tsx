"use client"

import { useEffect, useState, type ReactNode } from "react"
import { usePathname, useRouter } from "next/navigation"
import Link from "next/link"
import { Bot, Check, ChevronDown, ChevronRight, Copy, FilePlus, Sparkles, X } from "lucide-react"
import { api } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { cn } from "@/lib/utils"
import type { Document, Page, StandingAnswer } from "@/lib/types"
import { Alert, AlertAction, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { FileIcon } from "@/components/FilePreview"
import { docName } from "@/components/profile/DocumentsPage"
import { QuestionsForm } from "@/components/profile/QuestionsPage"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemMedia, ItemTitle } from "@/components/ui/item"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"

export function useList<T>(path: string) {
  const [rows, setRows] = useState<T[] | null>(null)
  const load = () => api<Page<T>>(path).then((p) => setRows(p.data || []))
  useEffect(() => {
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps -- path is a constant per caller
  }, [])
  return { rows, load }
}

// The main résumé and cover letter at a glance; the rest is under Documents.
function MainDocuments() {
  const { rows: docs } = useList<Document>("/v1/documents")
  if (!docs) return null
  const main = (["base_resume", "cover_letter"] as const).map((kind) => [kind, docs.find((d) => d.kind === kind && d.primary)] as const)
  const others = docs.filter((d) => !d.primary).length
  return (
    <ItemGroup className="gap-2">
      {main.map(([kind, d]) => (
        <Item key={kind} variant="outline" size="sm" asChild>
          <Link href={"/profile/documents#" + kind}>
            <ItemMedia variant="icon">{d ? <FileIcon type={d.content_type} name={d.filename} /> : <FilePlus />}</ItemMedia>
            <ItemContent>
              <ItemTitle>{d ? docName(d) : kind === "base_resume" ? "Add your résumé" : "Add a cover letter"}</ItemTitle>
              <ItemDescription>{kind === "base_resume" ? "Main résumé" : "Main cover letter"}</ItemDescription>
            </ItemContent>
            <ItemActions>
              <ChevronRight className="size-4 text-muted-foreground" />
            </ItemActions>
          </Link>
        </Item>
      ))}
      <Button asChild variant="link" size="sm" className="self-start px-0 text-muted-foreground">
        <Link href="/profile/documents">{others ? `All documents (${others} more)` : "All documents"}</Link>
      </Button>
    </ItemGroup>
  )
}

const AGENT_PROMPT = `Fill in my Pursuit profile from my résumé.

1. Run \`pursuit standing list\` to see what is already saved. Don't overwrite an answer without asking me.
2. Get my résumé: \`pursuit documents list\`, then \`pursuit documents get <id>\` for the base_resume marked primary (a binary body is base64).
3. Save what the résumé states with \`pursuit standing set --key <key> --value <answer>\` for these keys:
   first_name, last_name, email, phone, location, pronouns, linkedin, github, website (plus any other profile links, keyed by their name, e.g. portfolio)
4. Ask me, don't infer, for: work_authorization (Yes/No), visa_sponsorship (Yes/No), and anything about salary or start date.
5. Draft a short default cover letter (4-6 sentences, no buzzwords) from my most recent role, show it to me, and once I approve save it with \`pursuit documents add --kind cover_letter --file cover-letter.md --label Default\`.
6. Finish with \`pursuit list --status needs_input --fields id,company,title\` and ask me the unanswered questions on those applications, saving each answer with the question's wording as the key.`

// "open" (default, so a first visit sees the prompt), "closed", or "hidden".
const PROMPT_STATE = "pursuit.agent-prompt"

function readPref(key: string) {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function writePref(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {}
}

function AgentPrompt() {
  const [state, setState] = useState(() => (typeof window === "undefined" ? "open" : readPref(PROMPT_STATE) || "open"))
  const [copied, setCopied] = useState(false)
  const update = (next: string) => {
    setState(next)
    writePref(PROMPT_STATE, next)
  }
  async function copy() {
    await navigator.clipboard.writeText(AGENT_PROMPT)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  if (state === "hidden")
    return (
      <Button variant="link" size="sm" className="self-start px-0 text-muted-foreground" onClick={() => update("open")}>
        <Bot /> Show the agent prompt
      </Button>
    )

  const open = state === "open"
  return (
    <Collapsible open={open} onOpenChange={(o) => update(o ? "open" : "closed")} asChild>
      <Item variant="muted" className="flex-wrap">
        <ItemMedia variant="icon">
          <Bot />
        </ItemMedia>
        <ItemContent>
          <ItemTitle>Fill this in with an agent</ItemTitle>
          <ItemDescription>
            Upload your résumé, then paste this prompt into an agent that has the <code>pursuit</code> CLI and a write token.
          </ItemDescription>
        </ItemContent>
        <ItemActions>
          <Button size="sm" variant="outline" onClick={() => void copy()}>
            {copied ? <Check /> : <Copy />} {copied ? "Copied" : "Copy prompt"}
          </Button>
          <CollapsibleTrigger asChild>
            <Button size="icon-sm" variant="ghost" aria-label={open ? "Collapse prompt" : "Expand prompt"}>
              <ChevronDown className={cn("transition-transform", open && "rotate-180")} />
            </Button>
          </CollapsibleTrigger>
          <Button size="icon-sm" variant="ghost" aria-label="Dismiss agent prompt" onClick={() => update("hidden")}>
            <X />
          </Button>
        </ItemActions>
        <CollapsibleContent className="basis-full">
          <pre className="mt-1 max-h-64 overflow-auto rounded-md bg-background p-3 font-mono text-xs whitespace-pre-wrap">{AGENT_PROMPT}</pre>
        </CollapsibleContent>
      </Item>
    </Collapsible>
  )
}

export function Section({ id, title, description, children }: { id: string; title: string; description?: ReactNode; children: ReactNode }) {
  return (
    <Card id={id} className="scroll-mt-20">
      <CardHeader>
        <CardTitle className="text-base">{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  )
}

const TABS = [
  ["/profile", "Profile"],
  ["/profile/questions", "Questions"],
  ["/profile/documents", "Documents"],
] as const

// Shared header for /profile, /profile/questions and /profile/documents. Each tab is its own
// route, so either can be linked to directly.
export function ProfileHeader({ description }: { description: ReactNode }) {
  const pathname = usePathname()
  const router = useRouter()
  const current = TABS.find(([href]) => pathname.replace(/\/$/, "") === href)?.[0] ?? "/profile"
  return (
    <div className="flex flex-col gap-3">
      <div>
        <h1 className="text-xl font-semibold">Profile</h1>
        <p className="text-sm text-muted-foreground">{description}</p>
      </div>
      <Tabs value={current} onValueChange={(href) => router.push(href)}>
        <TabsList>
          {TABS.map(([href, label]) => (
            <TabsTrigger key={href} value={href}>
              {label}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>
    </div>
  )
}

export function ProfilePage() {
  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-4">
      <ProfileHeader description="What forms ask about you, and the files prepare attaches. Your name, photo, and sign-in live under Settings → Account." />
      <AgentPrompt />
      <Section id="documents" title="Documents">
        <MainDocuments />
      </Section>
      <QuestionsForm mode="profile" />
    </div>
  )
}

const DISMISSED = "pursuit.profile-nudge-dismissed"

// First-run prompt on the home page until there is a résumé and an email
// on file. Dismissal is per browser.
export function ProfileNudge({ onOpen }: { onOpen: (href: string) => void }) {
  const { can } = useAuth()
  const home = usePathname() === "/start"
  const [missing, setMissing] = useState<string[]>([])
  useEffect(() => {
    let dismissed = false
    try {
      dismissed = localStorage.getItem(DISMISSED) === "1"
    } catch {}
    if (!home || dismissed || !can("write")) return
    void Promise.all([api<Page<Document>>("/v1/documents"), api<Page<StandingAnswer>>("/v1/standing")])
      .then(([docs, standing]) => {
        const out = []
        if (!docs.data?.some((d) => d.kind === "base_resume")) out.push("a résumé")
        if (!standing.data?.some((s) => s.key === "email")) out.push("your contact details")
        setMissing(out)
      })
      .catch(() => {})
  }, [home, can])
  if (!home || !missing.length) return null

  function dismiss() {
    try {
      localStorage.setItem(DISMISSED, "1")
    } catch {}
    setMissing([])
  }

  return (
    <Alert className="mx-auto mb-4 max-w-5xl">
      <Sparkles />
      <AlertTitle>Finish your profile</AlertTitle>
      <AlertDescription>Add {missing.join(" and ")} so prepared applications fill themselves in.</AlertDescription>
      <AlertAction className="flex items-center gap-1">
        <Button size="sm" onClick={() => onOpen(missing.includes("a résumé") ? "/profile" : "/profile/questions")}>
          Set up
        </Button>
        <Button size="icon-sm" variant="ghost" aria-label="Dismiss" onClick={dismiss}>
          <X />
        </Button>
      </AlertAction>
    </Alert>
  )
}
