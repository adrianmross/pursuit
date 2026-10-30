"use client"

import { useEffect, useState, type FormEvent } from "react"
import { usePathname } from "next/navigation"
import { Download, Trash2, X } from "lucide-react"
import { api, apiHeaders, apiUrl } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import type { Document, Page, StandingAnswer } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

// The keys prepare matches by alias (service.matchStanding). Any other key
// only fills a form question whose normalized wording equals it.
const COMMON: [key: string, label: string, long?: boolean][] = [
  ["email", "Email"],
  ["phone", "Phone"],
  ["linkedin", "LinkedIn URL"],
  ["github", "GitHub URL"],
  ["website", "Website"],
  ["pronouns", "Pronouns"],
  ["work_authorization", "Are you authorized to work in the country of the role?"],
  ["visa_sponsorship", "Will you now or in the future require sponsorship?"],
  ["cover_letter", "Default cover letter", true],
]

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

function Answers() {
  const { rows, load } = useStanding()
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [custom, setCustom] = useState({ key: "", answer: "" })
  const [msg, setMsg] = useState({ ok: true, text: "" })
  if (!rows) return null

  const saved = Object.fromEntries(rows.map((r) => [r.key, r.answer]))
  const value = (k: string) => draft[k] ?? saved[k] ?? ""
  const others = rows.filter((r) => !COMMON.some(([k]) => k === r.key))

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

  const field = (key: string, label: string, long?: boolean) => (
    <div key={key} className="flex flex-col gap-1.5">
      <Label htmlFor={"standing-" + key}>{label}</Label>
      {long ? (
        <Textarea id={"standing-" + key} rows={5} value={value(key)} onChange={(e) => setDraft({ ...draft, [key]: e.target.value })} />
      ) : (
        <Input id={"standing-" + key} value={value(key)} onChange={(e) => setDraft({ ...draft, [key]: e.target.value })} />
      )}
    </div>
  )

  return (
    <form onSubmit={save} className="flex flex-col gap-3">
      <div>
        <h3 className="text-sm font-medium">Standing answers</h3>
        <p className="text-sm text-muted-foreground">
          Prepare fills application forms from these. Questions it can&apos;t answer land here once you answer them on an
          application.
        </p>
      </div>
      {COMMON.map(([k, label, long]) => field(k, label, long))}
      {others.map((r) => field(r.key, r.key.replaceAll("_", " ")))}
      <div className="grid grid-cols-[1fr_2fr] gap-2">
        <Input aria-label="New question" placeholder="Another question" value={custom.key} onChange={(e) => setCustom({ ...custom, key: e.target.value })} />
        <Input aria-label="Answer" placeholder="Answer" value={custom.answer} onChange={(e) => setCustom({ ...custom, answer: e.target.value })} />
      </div>
      {msg.text && <p className={"text-sm " + (msg.ok ? "text-emerald-600 dark:text-emerald-400" : "text-destructive")}>{msg.text}</p>}
      <Button type="submit" className="self-start">
        Save answers
      </Button>
    </form>
  )
}

export function ProfilePanel() {
  return (
    <div className="flex flex-col gap-8">
      <Documents />
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
