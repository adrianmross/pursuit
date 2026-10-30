"use client"

import { useEffect, useState, type FormEvent } from "react"
import { usePathname } from "next/navigation"
import { Download, Trash2, X } from "lucide-react"
import { api, apiHeaders, apiUrl } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import type { Document, Page, StandingAnswer } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { CommandBlock } from "@/components/CommandBlock"
import { Field, FieldDescription, FieldGroup, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Textarea } from "@/components/ui/textarea"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

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

function useStanding() {
  const [rows, setRows] = useState<StandingAnswer[] | null>(null)
  const load = () => api<Page<StandingAnswer>>("/v1/standing").then((p) => setRows(p.data || []))
  useEffect(() => {
    void load()
  }, [])
  return { rows, load }
}

function useDocuments() {
  const [docs, setDocs] = useState<Document[] | null>(null)
  const load = () => api<Page<Document>>("/v1/documents").then((p) => setDocs(p.data || []))
  useEffect(() => {
    void load()
  }, [])
  return { docs, load }
}

function Documents() {
  const { docs, load } = useDocuments()
  const [kind, setKind] = useState("base_resume")
  const [msg, setMsg] = useState("")

  async function upload(file?: File) {
    if (!file) return
    const form = new FormData()
    form.set("kind", kind)
    form.set("file", file)
    const res = await fetch(apiUrl("/v1/documents"), { method: "POST", headers: apiHeaders(), body: form })
    if (!res.ok) {
      const body = await res.json().catch(() => null)
      return setMsg(body?.error?.message || "Upload failed")
    }
    setMsg("")
    await load()
  }

  async function open(id: string) {
    const d = await api<Document & { body: string }>("/v1/documents/" + id)
    const text = d.content_type?.startsWith("text/") || d.content_type?.includes("json")
    const bytes = text ? new TextEncoder().encode(d.body) : Uint8Array.from(atob(d.body), (c) => c.charCodeAt(0))
    const url = URL.createObjectURL(new Blob([bytes], { type: d.content_type }))
    const a = Object.assign(document.createElement("a"), { href: url, download: d.filename || "document" })
    a.click()
    URL.revokeObjectURL(url)
  }

  async function remove(id: string) {
    await api("/v1/documents/" + id, { method: "DELETE" })
    await load()
  }

  return (
    <section className="flex flex-col gap-3">
      <div>
        <h3 className="text-sm font-medium">Documents</h3>
        <p className="text-sm text-muted-foreground">
          The newest base résumé is attached to every application you prepare. Agents can add tailored versions to a
          single application, which show up on that application.
        </p>
      </div>
      <div className="flex items-center gap-2">
        <Select value={kind} onValueChange={setKind}>
          <SelectTrigger className="w-40" aria-label="Document kind">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(KINDS).map(([k, label]) => (
              <SelectItem key={k} value={k}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button variant="outline" asChild>
          <label>
            Upload file
            <input type="file" accept=".pdf,.doc,.docx,.txt,.md" className="sr-only" onChange={(e) => void upload(e.target.files?.[0])} />
          </label>
        </Button>
      </div>
      {msg && <p className="text-sm text-destructive">{msg}</p>}
      {docs?.length ? (
        <ul className="flex flex-col divide-y rounded-md border">
          {docs.map((d) => (
            <li key={d.id} className="flex items-center gap-2 px-3 py-2 text-sm">
              <span className="min-w-0 flex-1 truncate">{d.filename || d.id}</span>
              <span className="text-xs text-muted-foreground">{KINDS[d.kind] || d.kind}</span>
              <Button size="icon-sm" variant="ghost" aria-label={`Download ${d.filename}`} onClick={() => void open(d.id)}>
                <Download />
              </Button>
              <Button size="icon-sm" variant="ghost" aria-label={`Delete ${d.filename}`} onClick={() => void remove(d.id)}>
                <Trash2 />
              </Button>
            </li>
          ))}
        </ul>
      ) : (
        docs && <p className="text-sm text-muted-foreground">No documents yet. Start with your résumé.</p>
      )}
    </section>
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

type Draft = Record<string, string>

function DeleteAnswer({ k, onDelete }: { k: string; onDelete: (k: string) => void }) {
  return (
    <Button type="button" size="icon-sm" variant="ghost" aria-label={`Delete saved answer for ${k}`} onClick={() => onDelete(k)}>
      <Trash2 />
    </Button>
  )
}

function Answers() {
  const { rows, load } = useStanding()
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

  const text = (k: string, label: string, kind?: Kind) => (
    <Field key={k}>
      <FieldLabel htmlFor={"standing-" + k}>{label}</FieldLabel>
      {kind === "yesno" ? (
        <div className="flex items-center gap-2">
          <RadioGroup value={value(k)} onValueChange={set(k)} className="flex w-auto gap-4" aria-label={label}>
            {["Yes", "No"].map((opt) => (
              <Field key={opt} orientation="horizontal" className="w-auto">
                <RadioGroupItem id={`standing-${k}-${opt}`} value={opt} />
                <FieldLabel htmlFor={`standing-${k}-${opt}`} className="font-normal">
                  {opt}
                </FieldLabel>
              </Field>
            ))}
          </RadioGroup>
          {k in saved && <DeleteAnswer k={k} onDelete={remove} />}
        </div>
      ) : (
        <div className="flex items-start gap-2">
          {kind === "long" ? (
            <Textarea id={"standing-" + k} rows={6} value={value(k)} onChange={(e) => set(k)(e.target.value)} />
          ) : (
            <Input id={"standing-" + k} type={kind === "url" ? "url" : kind === "email" ? "email" : "text"} value={value(k)} onChange={(e) => set(k)(e.target.value)} />
          )}
          {k in saved && <DeleteAnswer k={k} onDelete={remove} />}
        </div>
      )}
    </Field>
  )

  return (
    <form onSubmit={save} className="flex flex-col gap-6">
      <div>
        <h3 className="text-sm font-medium">Standing answers</h3>
        <p className="text-sm text-muted-foreground">
          Prepare fills application forms from these. Answers you give to unanswered questions on an application show
          up under Other answers.
        </p>
      </div>
      {Object.entries(GROUPS).map(([legend, fields]) => (
        <FieldSet key={legend}>
          <FieldLegend>{legend}</FieldLegend>
          <FieldGroup className={fields.length > 2 ? "grid gap-4 sm:grid-cols-2" : undefined}>{fields.map(([k, label, kind]) => text(k, label, kind))}</FieldGroup>
        </FieldSet>
      ))}
      <FieldSet>
        <FieldLegend>Other answers</FieldLegend>
        <FieldDescription>Each one fills a form question with the same wording, such as &quot;Why do you want to work here?&quot;</FieldDescription>
        <FieldGroup>
          {others.map((r) => text(r.key, r.key.replaceAll("_", " "), r.answer.length > 120 ? "long" : undefined))}
          <Field orientation="horizontal">
            <Input aria-label="New question" placeholder="Question" value={custom.key} onChange={(e) => setCustom({ ...custom, key: e.target.value })} />
            <Input aria-label="Answer" placeholder="Answer" value={custom.answer} onChange={(e) => setCustom({ ...custom, answer: e.target.value })} />
          </Field>
        </FieldGroup>
      </FieldSet>
      {msg.text && <p className={"text-sm " + (msg.ok ? "text-emerald-600 dark:text-emerald-400" : "text-destructive")}>{msg.text}</p>}
      <Button type="submit" className="self-start">
        Save answers
      </Button>
    </form>
  )
}

function AgentPrompt() {
  return (
    <section className="flex flex-col gap-2">
      <h3 className="text-sm font-medium">Fill this in with an agent</h3>
      <p className="text-sm text-muted-foreground">
        Upload your résumé above, then paste this into an agent that has the <code>pursuit</code> CLI and a write token.
      </p>
      <CommandBlock command={AGENT_PROMPT} wrap />
    </section>
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
    <div className="mx-auto mb-4 flex max-w-5xl items-center gap-3 rounded-lg border bg-muted/40 px-4 py-3 text-sm">
      <p className="flex-1">Add {missing.join(" and ")} so prepared applications fill themselves in.</p>
      <Button size="sm" onClick={onOpen}>
        Set up profile
      </Button>
      <Button size="icon-sm" variant="ghost" aria-label="Dismiss" onClick={dismiss}>
        <X />
      </Button>
    </div>
  )
}
