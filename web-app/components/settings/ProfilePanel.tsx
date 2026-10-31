"use client"

import { useEffect, useState, type FormEvent } from "react"
import { usePathname } from "next/navigation"
import { Bot, Check, ChevronDown, Copy, Eye, Plus, Sparkles, Trash2, X } from "lucide-react"
import { api } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { cn } from "@/lib/utils"
import type { Document, Page, StandingAnswer } from "@/lib/types"
import { Alert, AlertAction, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { FileDrop } from "@/components/FileDrop"
import { FileIcon, FilePreview, isTextType } from "@/components/FilePreview"
import { Field, FieldDescription, FieldGroup, FieldLabel, FieldLegend, FieldSeparator, FieldSet } from "@/components/ui/field"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput, InputGroupTextarea } from "@/components/ui/input-group"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemMedia, ItemTitle } from "@/components/ui/item"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"

// The keys prepare matches by alias (service.matchStanding). Any other key
// only fills a form question whose normalized wording equals it.
type Kind = "email" | "url" | "yesno" | "long"
const GROUPS: Record<string, [key: string, label: string, kind?: Kind][]> = {
  Contact: [
    ["email", "Email", "email"],
    ["phone", "Phone"],
    ["pronouns", "Pronouns"],
  ],
  Links: [
    ["linkedin", "LinkedIn", "url"],
    ["github", "GitHub", "url"],
    ["website", "Website", "url"],
  ],
  Eligibility: [
    ["work_authorization", "Are you authorized to work in the country of the role?", "yesno"],
    ["visa_sponsorship", "Will you now or in the future require visa sponsorship?", "yesno"],
  ],
  "Cover letter": [["cover_letter", "Default cover letter", "long"]],
}

const KINDS: Record<string, string> = { base_resume: "Base résumé", cover_letter: "Cover letter", other: "Other" }

function Status({ ok, text }: { ok: boolean; text: string }) {
  return text ? <p className={cn("text-sm", ok ? "text-emerald-600 dark:text-emerald-400" : "text-destructive")}>{text}</p> : null
}

function useList<T>(path: string) {
  const [rows, setRows] = useState<T[] | null>(null)
  const load = () => api<Page<T>>(path).then((p) => setRows(p.data || []))
  useEffect(() => {
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps -- path is a constant per caller
  }, [])
  return { rows, load }
}

function Documents() {
  const { rows: docs, load } = useList<Document>("/v1/documents")

  async function remove(id: string) {
    await api("/v1/documents/" + id, { method: "DELETE" })
    await load()
  }

  // Prepare attaches the newest base résumé; flag which one that is.
  const active = docs?.find((d) => d.kind === "base_resume")?.id

  return (
    <FieldSet>
      <FieldLegend>Documents</FieldLegend>
      <FieldDescription>
        The newest base résumé is attached to every application you prepare. Tailored versions for one application go
        on that application.
      </FieldDescription>
      <FileDrop
        path="/v1/documents"
        kinds={KINDS}
        description={'PDF, Word, Markdown, or text, up to 5MB. A LinkedIn "Save to PDF" export works too.'}
        onUploaded={() => void load()}
      />
      {!!docs?.length && (
        <ItemGroup className="gap-2">
          {docs.map((d) => (
            <Item key={d.id} variant="outline" size="sm">
              <ItemMedia variant="icon">
                <FileIcon type={d.content_type} name={d.filename} />
              </ItemMedia>
              <ItemContent>
                <ItemTitle>
                  {d.filename || d.id}
                  {d.id === active && <Badge variant="secondary">Used by prepare</Badge>}
                </ItemTitle>
                <ItemDescription>
                  {KINDS[d.kind] || d.kind}
                  {d.created_at && " · " + new Date(d.created_at).toLocaleDateString()}
                </ItemDescription>
              </ItemContent>
              <ItemActions>
                <FilePreview path={"/v1/documents/" + d.id} base64={(f) => !isTextType(f.content_type)} title={d.filename || "Document"} description={KINDS[d.kind]}>
                  <Button size="icon-sm" variant="ghost" aria-label={`Preview ${d.filename}`}>
                    <Eye />
                  </Button>
                </FilePreview>
                <Button size="icon-sm" variant="ghost" aria-label={`Delete ${d.filename}`} onClick={() => void remove(d.id)}>
                  <Trash2 />
                </Button>
              </ItemActions>
            </Item>
          ))}
        </ItemGroup>
      )}
    </FieldSet>
  )
}

const AGENT_PROMPT = `Fill in my Pursuit profile from my résumé.

1. Run \`pursuit standing list\` to see what is already saved. Don't overwrite an answer without asking me.
2. Get my résumé: \`pursuit documents list\`, then \`pursuit documents get <id>\` for the newest base_resume (the body is base64).
3. Save what the résumé states with \`pursuit standing set --key <key> --value <answer>\` for these keys:
   email, phone, linkedin, github, website, pronouns
4. Ask me, don't infer, for: work_authorization (Yes/No), visa_sponsorship (Yes/No), and anything about salary or start date.
5. Draft a short default cover letter (4-6 sentences, no buzzwords) from my most recent role, show it to me, and save it as cover_letter once I approve.
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

type Draft = Record<string, string>

function Answers() {
  const { rows, load } = useList<StandingAnswer>("/v1/standing")
  const [draft, setDraft] = useState<Draft>({})
  const [custom, setCustom] = useState({ key: "", answer: "" })
  const [msg, setMsg] = useState({ ok: true, text: "" })
  if (!rows) return null

  const saved: Draft = Object.fromEntries(rows.map((r) => [r.key, r.answer]))
  const value = (k: string) => draft[k] ?? saved[k] ?? ""
  const set = (k: string) => (v: string) => setDraft({ ...draft, [k]: v })
  const known = new Set(Object.values(GROUPS).flat().map(([k]) => k))
  const others = rows.filter((r) => !known.has(r.key))

  async function remove(key: string) {
    try {
      await api("/v1/standing/" + encodeURIComponent(key), { method: "DELETE" })
      setDraft(Object.fromEntries(Object.entries(draft).filter(([k]) => k !== key)))
      await load()
      setMsg({ ok: true, text: "Deleted." })
    } catch (err) {
      setMsg({ ok: false, text: err instanceof Error ? err.message : "Could not delete" })
    }
  }

  async function save(event: FormEvent) {
    event.preventDefault()
    const changed = Object.entries(draft).filter(([k, v]) => v.trim() && v !== saved[k])
    if (custom.key.trim() && custom.answer.trim()) changed.push([custom.key, custom.answer])
    try {
      for (const [key, answer] of changed) await api("/v1/standing", { method: "POST", body: { key, answer } })
      setDraft({})
      setCustom({ key: "", answer: "" })
      await load()
      setMsg({ ok: true, text: changed.length ? "Saved." : "Nothing changed." })
    } catch (err) {
      setMsg({ ok: false, text: err instanceof Error ? err.message : "Could not save" })
    }
  }

  const clear = (k: string, align: "inline-end" | "block-end" = "inline-end") =>
    k in saved && (
      <InputGroupAddon align={align}>
        <InputGroupButton size="icon-xs" aria-label={`Delete saved answer for ${k}`} onClick={() => void remove(k)}>
          <Trash2 />
        </InputGroupButton>
      </InputGroupAddon>
    )

  const field = (k: string, label: string, kind?: Kind) => {
    const id = "standing-" + k
    if (kind === "yesno")
      return (
        <Field key={k}>
          <FieldLabel htmlFor={id + "-Yes"}>{label}</FieldLabel>
          <div className="flex items-center gap-3">
            <RadioGroup value={value(k)} onValueChange={set(k)} className="flex w-auto gap-4" aria-label={label}>
              {["Yes", "No"].map((opt) => (
                <Field key={opt} orientation="horizontal" className="w-auto">
                  <RadioGroupItem id={`${id}-${opt}`} value={opt} />
                  <FieldLabel htmlFor={`${id}-${opt}`} className="font-normal">
                    {opt}
                  </FieldLabel>
                </Field>
              ))}
            </RadioGroup>
            {k in saved && (
              <Button type="button" size="icon-xs" variant="ghost" aria-label={`Delete saved answer for ${k}`} onClick={() => void remove(k)}>
                <Trash2 />
              </Button>
            )}
          </div>
        </Field>
      )
    return (
      <Field key={k}>
        <FieldLabel htmlFor={id}>{label}</FieldLabel>
        <InputGroup>
          {kind === "long" ? (
            <InputGroupTextarea id={id} rows={6} value={value(k)} onChange={(e) => set(k)(e.target.value)} />
          ) : (
            <InputGroupInput id={id} type={kind === "url" ? "url" : kind === "email" ? "email" : "text"} value={value(k)} onChange={(e) => set(k)(e.target.value)} />
          )}
          {clear(k, kind === "long" ? "block-end" : "inline-end")}
        </InputGroup>
      </Field>
    )
  }

  return (
    <form onSubmit={save}>
      <FieldGroup>
        <FieldSet>
          <FieldLegend>Standing answers</FieldLegend>
          <FieldDescription>Prepare fills application forms from these.</FieldDescription>
        </FieldSet>
        {Object.entries(GROUPS).map(([legend, fields]) => (
          <FieldSet key={legend}>
            <FieldLegend variant="label">{legend}</FieldLegend>
            <FieldGroup className={fields.length > 2 ? "grid gap-4 sm:grid-cols-2" : undefined}>
              {fields.map(([k, label, kind]) => field(k, label, kind))}
            </FieldGroup>
          </FieldSet>
        ))}
        <FieldSeparator />
        <FieldSet>
          <FieldLegend variant="label">Other answers</FieldLegend>
          <FieldDescription>
            Each fills a form question with the same wording. Answers you give on an application land here.
          </FieldDescription>
          <FieldGroup>
            {others.map((r) => field(r.key, r.key.replaceAll("_", " "), r.answer.length > 120 ? "long" : undefined))}
            <ButtonGroup className="w-full">
              <InputGroup className="flex-1">
                <InputGroupInput aria-label="New question" placeholder="Question" value={custom.key} onChange={(e) => setCustom({ ...custom, key: e.target.value })} />
              </InputGroup>
              <InputGroup className="flex-[2]">
                <InputGroupInput aria-label="Answer" placeholder="Answer" value={custom.answer} onChange={(e) => setCustom({ ...custom, answer: e.target.value })} />
              </InputGroup>
              <Button type="submit" variant="outline" aria-label="Add answer" disabled={!custom.key.trim() || !custom.answer.trim()}>
                <Plus />
              </Button>
            </ButtonGroup>
          </FieldGroup>
        </FieldSet>
        <Field orientation="horizontal">
          <Button type="submit">Save answers</Button>
          <Status {...msg} />
        </Field>
      </FieldGroup>
    </form>
  )
}

export function ProfilePanel() {
  return (
    <div className="flex flex-col gap-8">
      <Documents />
      <AgentPrompt />
      <Answers />
    </div>
  )
}

const DISMISSED = "pursuit.profile-nudge-dismissed"

// First-run prompt on the home page until there is a résumé and an email
// on file. Dismissal is per browser.
export function ProfileNudge({ onOpen }: { onOpen: () => void }) {
  const { can } = useAuth()
  const home = usePathname() === "/"
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
        <Button size="sm" onClick={onOpen}>
          Set up
        </Button>
        <Button size="icon-sm" variant="ghost" aria-label="Dismiss" onClick={dismiss}>
          <X />
        </Button>
      </AlertAction>
    </Alert>
  )
}
