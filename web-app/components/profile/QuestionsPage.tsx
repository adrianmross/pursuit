"use client"

import { useState, type FormEvent } from "react"
import Link from "next/link"
import { Link2, MessageSquarePlus, Plus, Search, SearchX, Trash2 } from "lucide-react"
import { api } from "@/lib/api-client"
import { cn } from "@/lib/utils"
import type { StandingAnswer } from "@/lib/types"
import { ProfileHeader, Section, useList } from "@/components/profile/ProfilePage"
import { Button } from "@/components/ui/button"
import { Spinner } from "@/components/ui/spinner"
import { ButtonGroup } from "@/components/ui/button-group"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput, InputGroupTextarea } from "@/components/ui/input-group"
import { ItemGroup } from "@/components/ui/item"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"

// Keys prepare fills: the exact normalized wording of a form question, or
// one of service.matchStanding's aliases (email, phone, linkedin, ...).
type Kind = "email" | "yesno" | "long"
const FIELDS: Record<"contact" | "eligibility" | "letter", [key: string, label: string, kind?: Kind][]> = {
  contact: [
    ["first_name", "First name"],
    ["last_name", "Last name"],
    ["email", "Email", "email"],
    ["phone", "Phone"],
    ["location", "Location"],
    ["pronouns", "Pronouns"],
  ],
  eligibility: [
    ["work_authorization", "Are you authorized to work in the country of the role?", "yesno"],
    ["visa_sponsorship", "Will you now or in the future require visa sponsorship?", "yesno"],
  ],
  letter: [["cover_letter", "Default cover letter", "long"]],
}

// Answers shown under Links: these keys, plus any answer that is a URL.
const LINK_KEYS = new Set(["linkedin", "github", "gitlab", "website", "portfolio", "x", "twitter", "dribbble", "behance", "medium", "stackoverflow"])
const LINK_SUGGESTIONS: [key: string, label: string][] = [
  ["linkedin", "LinkedIn"],
  ["github", "GitHub"],
  ["website", "Website"],
  ["portfolio", "Portfolio"],
  ["twitter", "Twitter"],
]

type Draft = Record<string, string>

const isLink = (key: string, value: string) => LINK_KEYS.has(key) || /^https?:\/\//i.test(value.trim())

const LINK_NAMES: Record<string, string> = { linkedin: "LinkedIn", github: "GitHub", gitlab: "GitLab", x: "X", twitter: "X / Twitter" }
const humanize = (key: string) => key.replaceAll("_", " ").replace(/^./, (c) => c.toUpperCase())
const linkName = (key: string) => LINK_NAMES[key] || humanize(key)

// Mirrors impex.NormalizeKey, which the server applies on every save:
// lowercase, letters and digits kept, every other run becomes one "_".
export const normalizeKey = (s: string) =>
  s
    .toLowerCase()
    .replace(/[^\p{L}\p{Nd}]+/gu, "_")
    .replace(/^_+|_+$/g, "")

type Sort = "recent" | "az"

function AddQuestion({ initial, existing, onClose, onSaved }: { initial: string; existing: Set<string>; onClose: () => void; onSaved: () => Promise<void> }) {
  const [question, setQuestion] = useState(initial)
  const [answer, setAnswer] = useState("")
  const [error, setError] = useState("")
  const [saving, setSaving] = useState(false)
  const key = normalizeKey(question)

  async function submit(event: FormEvent) {
    event.preventDefault()
    setSaving(true)
    setError("")
    try {
      await api("/v1/standing", { method: "POST", body: { key: question, answer } })
      await onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save")
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <form onSubmit={submit} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Add a question</DialogTitle>
            <DialogDescription>Write the question the way forms ask it. Prepare fills any form question with the same wording.</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="add-question">Question</FieldLabel>
              <InputGroup>
                <InputGroupInput id="add-question" autoFocus placeholder="e.g. What are your salary expectations?" value={question} onChange={(e) => setQuestion(e.target.value)} />
              </InputGroup>
              <FieldDescription>
                {key ? (
                  <>
                    Saved as <code className="rounded bg-muted px-1 font-mono text-xs">{key}</code>
                    {existing.has(key) && <span className="text-amber-700 dark:text-amber-400"> · replaces your current answer</span>}
                  </>
                ) : (
                  "Lowercased, with spaces and punctuation turned into _, so small wording differences still match."
                )}
              </FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor="add-answer">Answer</FieldLabel>
              <InputGroup>
                <InputGroupTextarea id="add-answer" rows={4} value={answer} onChange={(e) => setAnswer(e.target.value)} />
              </InputGroup>
            </Field>
          </FieldGroup>
          {error && <p className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={saving || !key || !answer.trim()}>
              {saving && <Spinner />} Save
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function QuestionsForm() {
  const { rows, load } = useList<StandingAnswer>("/v1/standing")
  const [draft, setDraft] = useState<Draft>({})
  const [newLink, setNewLink] = useState({ label: "", url: "" })
  const [msg, setMsg] = useState({ ok: true, text: "" })
  const [saving, setSaving] = useState(false)
  const [query, setQuery] = useState("")
  const [sort, setSort] = useState<Sort>("recent")
  // null: dialog closed; otherwise the question it opens with.
  const [adding, setAdding] = useState<string | null>(null)
  if (!rows) return null

  const saved: Draft = Object.fromEntries(rows.map((r) => [r.key, r.answer]))
  const value = (k: string) => draft[k] ?? saved[k] ?? ""
  const set = (k: string) => (v: string) => setDraft({ ...draft, [k]: v })
  const fixed = new Set(Object.values(FIELDS).flat().map(([k]) => k))
  const links = rows.filter((r) => !fixed.has(r.key) && isLink(r.key, r.answer))
  const others = rows
    .filter((r) => !fixed.has(r.key) && !isLink(r.key, r.answer))
    .sort((a, b) => (sort === "az" ? a.key.localeCompare(b.key) : (b.updated_at || "").localeCompare(a.updated_at || "")))
  const suggestions = LINK_SUGGESTIONS.filter(([k]) => !(k in saved))

  // Filtering only hides rows; unsaved edits in hidden rows still count.
  const q = query.trim().toLowerCase()
  const qKey = normalizeKey(q)
  const hit = (k: string, label: string) => !q || (!!qKey && k.includes(qKey)) || label.toLowerCase().includes(q) || value(k).toLowerCase().includes(q)
  const contact = FIELDS.contact.filter(([k, label]) => hit(k, label))
  const eligibility = FIELDS.eligibility.filter(([k, label]) => hit(k, label))
  const letter = FIELDS.letter.filter(([k, label]) => hit(k, label))
  const shownLinks = links.filter((r) => hit(r.key, linkName(r.key)))
  const shownOthers = others.filter((r) => hit(r.key, humanize(r.key)))
  const matches = contact.length + eligibility.length + letter.length + shownLinks.length + shownOthers.length

  const pending = Object.entries(draft).filter(([k, v]) => v.trim() && v !== saved[k])
  if (newLink.label.trim() && newLink.url.trim()) pending.push([newLink.label, newLink.url])

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

  async function save(event?: FormEvent) {
    event?.preventDefault()
    setSaving(true)
    try {
      for (const [key, answer] of pending) await api("/v1/standing", { method: "POST", body: { key, answer } })
      setDraft({})
      setNewLink({ label: "", url: "" })
      await load()
      setMsg({ ok: true, text: "Saved." })
    } catch (err) {
      setMsg({ ok: false, text: err instanceof Error ? err.message : "Could not save" })
    } finally {
      setSaving(false)
    }
  }

  const trash = (k: string, label: string) =>
    k in saved && (
      <InputGroupAddon align="inline-end">
        <InputGroupButton size="icon-xs" aria-label={`Delete ${label}`} onClick={() => void remove(k)}>
          <Trash2 />
        </InputGroupButton>
      </InputGroupAddon>
    )

  const field = (k: string, label: string, kind?: Kind, hint?: string) => {
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
              <Button type="button" size="icon-xs" variant="ghost" aria-label={`Delete ${label}`} onClick={() => void remove(k)}>
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
            <InputGroupTextarea id={id} rows={k === "cover_letter" ? 8 : 4} value={value(k)} onChange={(e) => set(k)(e.target.value)} />
          ) : (
            <InputGroupInput id={id} type={kind === "email" ? "email" : "text"} value={value(k)} onChange={(e) => set(k)(e.target.value)} />
          )}
          {trash(k, label)}
        </InputGroup>
        {hint && <FieldDescription className="font-mono text-xs">{hint}</FieldDescription>}
      </Field>
    )
  }

  const updated = (r: StandingAnswer) => r.key + (r.updated_at ? " · updated " + new Date(r.updated_at).toLocaleDateString() : "")

  return (
    <>
      <form onSubmit={save} className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <InputGroup className="min-w-0 flex-1 basis-60">
            <InputGroupAddon>
              <Search />
            </InputGroupAddon>
            <InputGroupInput type="search" aria-label="Search questions and answers" placeholder="Search questions and answers" value={query} onChange={(e) => setQuery(e.target.value)} />
            {q && <InputGroupAddon align="inline-end">{matches === 1 ? "1 match" : `${matches} matches`}</InputGroupAddon>}
          </InputGroup>
          <Button type="button" onClick={() => setAdding(q && !matches ? query.trim() : "")}>
            <Plus /> Add question
          </Button>
        </div>

        {q && !matches && (
          <Empty className="border">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <SearchX />
              </EmptyMedia>
              <EmptyTitle>No answers match “{query.trim()}”</EmptyTitle>
              <EmptyDescription>Search covers question wording, saved keys, and answers.</EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button type="button" variant="outline" onClick={() => setAdding(query.trim())}>
                <Plus /> Add “{query.trim()}” as a question
              </Button>
            </EmptyContent>
          </Empty>
        )}

        {(!q || shownOthers.length > 0) && (
          <Section
            id="answers"
            title="Questions"
            description="Each answer fills a form question with the same wording. Answers you give on an application land here."
          >
            {others.length === 0 ? (
              <Empty className="p-6 md:p-6">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <MessageSquarePlus />
                  </EmptyMedia>
                  <EmptyTitle>No questions yet</EmptyTitle>
                  <EmptyDescription>
                    When a form asks something you have not answered, the application waits in Needs input. Answer it there and it is saved
                    here for the next form. Or add the usual ones now: salary expectations, notice period, why this company.
                  </EmptyDescription>
                </EmptyHeader>
                <EmptyContent>
                  <Button type="button" variant="outline" onClick={() => setAdding("")}>
                    <Plus /> Add question
                  </Button>
                  {rows.length === 0 && (
                    <Button asChild variant="link" size="sm">
                      <Link href="/profile">Or let an agent fill them in from your résumé</Link>
                    </Button>
                  )}
                </EmptyContent>
              </Empty>
            ) : (
              <FieldGroup>
                {others.length > 1 && !q && (
                  <ToggleGroup type="single" variant="outline" size="sm" value={sort} onValueChange={(v) => v && setSort(v as Sort)} aria-label="Sort questions">
                    <ToggleGroupItem value="recent">Recently updated</ToggleGroupItem>
                    <ToggleGroupItem value="az">A–Z</ToggleGroupItem>
                  </ToggleGroup>
                )}
                {shownOthers.map((r) => field(r.key, humanize(r.key), value(r.key).length > 120 ? "long" : undefined, updated(r)))}
              </FieldGroup>
            )}
          </Section>
        )}

        {contact.length > 0 && (
          <Section id="contact" title="Personal details" description="Name and contact details forms ask for first.">
            <FieldGroup>{contact.map(([k, label, kind]) => field(k, label, kind))}</FieldGroup>
          </Section>
        )}

        {(!q || shownLinks.length > 0) && (
          <Section id="links" title="Links" description="Any profile or site. Each fills a form question with the same name, so a link labelled Portfolio answers “Portfolio”.">
            <FieldGroup>
              {shownLinks.length > 0 && (
                <ItemGroup className="gap-2">
                  {shownLinks.map((r) => (
                    <Field key={r.key}>
                      <FieldLabel htmlFor={"standing-" + r.key}>{linkName(r.key)}</FieldLabel>
                      <InputGroup>
                        <InputGroupAddon>
                          <Link2 />
                        </InputGroupAddon>
                        <InputGroupInput id={"standing-" + r.key} type="url" value={value(r.key)} onChange={(e) => set(r.key)(e.target.value)} />
                        {trash(r.key, linkName(r.key))}
                      </InputGroup>
                    </Field>
                  ))}
                </ItemGroup>
              )}
              {!q && (
                <Field>
                  <FieldLabel htmlFor="new-link-label">Add a link</FieldLabel>
                  <ButtonGroup className="w-full">
                    <InputGroup className="w-40 shrink-0">
                      <InputGroupInput id="new-link-label" placeholder="Label" value={newLink.label} onChange={(e) => setNewLink({ ...newLink, label: e.target.value })} />
                    </InputGroup>
                    <InputGroup className="flex-1">
                      <InputGroupInput aria-label="URL" type="url" placeholder="https://" value={newLink.url} onChange={(e) => setNewLink({ ...newLink, url: e.target.value })} />
                    </InputGroup>
                    <Button type="submit" variant="outline" aria-label="Add link" disabled={!newLink.label.trim() || !newLink.url.trim()}>
                      <Plus />
                    </Button>
                  </ButtonGroup>
                  {suggestions.length > 0 && (
                    <div className="flex flex-wrap gap-1.5">
                      {suggestions.map(([k, label]) => (
                        <Button key={k} type="button" size="xs" variant="outline" onClick={() => setNewLink({ label, url: newLink.url })}>
                          <Plus /> {label}
                        </Button>
                      ))}
                    </div>
                  )}
                </Field>
              )}
            </FieldGroup>
          </Section>
        )}

        {eligibility.length > 0 && (
          <Section id="eligibility" title="Work eligibility">
            <FieldGroup>{eligibility.map(([k, label, kind]) => field(k, label, kind))}</FieldGroup>
          </Section>
        )}

        {letter.length > 0 && (
          <Section id="cover-letter" title="Cover letter" description="Attached when a form asks for one and no tailored letter is on the application.">
            <FieldGroup>{letter.map(([k, label, kind]) => field(k, label, kind))}</FieldGroup>
          </Section>
        )}

        {(pending.length > 0 || msg.text) && (
          <div className="sticky bottom-4 z-10 flex items-center gap-3 rounded-lg border bg-background/95 px-4 py-2 shadow-sm backdrop-blur">
            <p className={cn("flex-1 text-sm", !msg.ok && "text-destructive")}>
              {pending.length ? `${pending.length} unsaved ${pending.length === 1 ? "change" : "changes"}` : msg.text}
            </p>
            {pending.length > 0 && (
              <>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    setDraft({})
                    setNewLink({ label: "", url: "" })
                  }}
                >
                  Discard
                </Button>
                <Button type="submit" size="sm" disabled={saving}>
                  {saving && <Spinner />} Save
                </Button>
              </>
            )}
          </div>
        )}
      </form>

      {/* Outside the form: React bubbles the dialog's submit through the portal. */}
      {adding !== null && (
        <AddQuestion
          initial={adding}
          existing={new Set(Object.keys(saved))}
          onClose={() => setAdding(null)}
          onSaved={async () => {
            await load()
            setMsg({ ok: true, text: "Saved." })
          }}
        />
      )}
    </>
  )
}

export function QuestionsPage() {
  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-4">
      <ProfileHeader description="Standing answers prepare uses to fill in application forms. The same list as pursuit standing on the CLI." />
      <QuestionsForm />
    </div>
  )
}
