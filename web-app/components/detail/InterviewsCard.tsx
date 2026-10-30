"use client"

import { useEffect, useState } from "react"
import { ChevronDown, Plus, Trash2 } from "lucide-react"
import { api, ApiError } from "@/lib/api-client"
import { label, type Interview } from "@/lib/types"
import { InlineSelect, InlineText } from "@/components/InlineEdit"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"

const STAGES: [string, string][] = [
  ["awaiting_review", "Prepared"],
  ["applied", "Applied"],
  ["screening", "Screening"],
  ["interviewing", "Interviewing"],
  ["final", "Final"],
  ["offer", "Offer"],
]

const day = (value?: string) => (value ? value.slice(0, 10) : "")

function TextBox({
  value,
  placeholder,
  disabled,
  onSave,
}: {
  value: string
  placeholder: string
  disabled?: boolean
  onSave: (next: string) => void
}) {
  const [draft, setDraft] = useState(value)
  useEffect(() => {
    // Syncs from the prop after a save, not a render loop.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDraft(value)
  }, [value])
  return (
    <div className="flex flex-col gap-1.5">
      <Textarea
        className="min-h-24 text-sm"
        placeholder={placeholder}
        value={draft}
        disabled={disabled}
        onChange={(e) => setDraft(e.target.value)}
      />
      {draft !== value && !disabled && (
        <div className="flex gap-2">
          <Button size="sm" onClick={() => onSave(draft)}>
            Save
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setDraft(value)}>
            Discard
          </Button>
        </div>
      )}
    </div>
  )
}

function Row({
  item,
  writable,
  onPatch,
  onDelete,
}: {
  item: Interview
  writable: boolean
  onPatch: (item: Interview, body: Record<string, unknown>) => void
  onDelete: (item: Interview) => void
}) {
  return (
    <li className="flex flex-col gap-2 rounded-lg border p-3">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
        <Checkbox
          aria-label="Done"
          checked={!!item.done_at}
          disabled={!writable}
          onCheckedChange={(checked) => onPatch(item, { done: checked === true })}
        />
        <InlineText
          value={item.kind || ""}
          placeholder="Round"
          disabled={!writable}
          className="text-sm font-medium"
          inputClassName="h-7 text-sm"
          onSave={(v) => onPatch(item, { kind: v })}
        />
        <InlineSelect
          value={item.stage}
          display={label(item.stage)}
          options={STAGES}
          disabled={!writable}
          className="text-xs text-muted-foreground"
          onSave={(v) => onPatch(item, { stage: v })}
        />
        <Input
          type="date"
          aria-label="Date"
          className="h-7 w-36 text-sm"
          value={day(item.scheduled_at)}
          disabled={!writable}
          onChange={(e) => onPatch(item, { scheduled_at: e.target.value })}
        />
        <InlineText
          value={item.interviewer || ""}
          placeholder="Who"
          disabled={!writable}
          className="text-sm text-muted-foreground"
          inputClassName="h-7 text-sm"
          onSave={(v) => onPatch(item, { interviewer: v })}
        />
        {writable && (
          <Button variant="ghost" size="icon" className="ml-auto size-6" aria-label="Delete interview" onClick={() => onDelete(item)}>
            <Trash2 className="size-3.5" />
          </Button>
        )}
      </div>
      <Collapsible defaultOpen={!!item.prep && !item.done_at}>
        <CollapsibleTrigger className="group flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground">
          <ChevronDown className="size-3.5 transition-transform group-data-[state=closed]:-rotate-90" />
          Prep and notes
        </CollapsibleTrigger>
        <CollapsibleContent className="mt-2 grid gap-3 sm:grid-cols-2">
          <TextBox
            value={item.prep || ""}
            placeholder="What to prepare: topics, stories, questions to ask…"
            disabled={!writable}
            onSave={(v) => onPatch(item, { prep: v })}
          />
          <TextBox
            value={item.notes || ""}
            placeholder="How it went"
            disabled={!writable}
            onSave={(v) => onPatch(item, { notes: v })}
          />
        </CollapsibleContent>
      </Collapsible>
    </li>
  )
}

// InterviewsCard lists an application's rounds. Any stage can have them, and
// each round is a sub-arc of its stage's piece on the progress mark; marking
// one done fills it.
export function InterviewsCard({
  appId,
  current,
  writable,
  onChange,
}: {
  appId: string
  current: number
  writable: boolean
  onChange: () => void
}) {
  const [items, setItems] = useState<Interview[]>([])
  const [error, setError] = useState<string | null>(null)
  const [kind, setKind] = useState("")
  // New rounds join the stage the application is on.
  const [stage, setStage] = useState(STAGES[Math.max(current, 1) - 1][0])
  const base = `/v1/applications/${encodeURIComponent(appId)}/interviews`

  async function load() {
    try {
      const page = await api<{ data: Interview[] }>(base)
      setItems(page.data || [])
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load interviews")
    }
  }

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [appId])

  async function run(request: Promise<unknown>) {
    setError(null)
    try {
      await request
      await load()
      onChange()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save")
    }
  }

  const add = () => run(api(base, { method: "POST", body: { stage, kind } }).then(() => setKind("")))
  const patch = (item: Interview, body: Record<string, unknown>) =>
    run(api(`/v1/interviews/${encodeURIComponent(item.id)}`, { method: "PATCH", body: { version: item.version, ...body } }))
  const remove = (item: Interview) =>
    run(api(`/v1/interviews/${encodeURIComponent(item.id)}?version=${item.version}`, { method: "DELETE" }))

  const ordered = STAGES.flatMap(([value]) => items.filter((item) => item.stage === value))

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Interviews</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {ordered.length > 0 && (
          <ul className="flex flex-col gap-2">
            {ordered.map((item) => (
              <Row key={item.id} item={item} writable={writable} onPatch={patch} onDelete={remove} />
            ))}
          </ul>
        )}
        {ordered.length === 0 && <p className="text-sm text-muted-foreground">No rounds yet. Each one fills part of the mark as you finish it.</p>}
        {writable && (
          <form
            className="flex flex-wrap items-center gap-2"
            onSubmit={(e) => {
              e.preventDefault()
              void add()
            }}
          >
            <Select value={stage} onValueChange={setStage}>
              <SelectTrigger className="h-8 w-36" aria-label="Stage">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {STAGES.map(([value, text]) => (
                  <SelectItem key={value} value={value}>
                    {text}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              className="h-8 min-w-40 flex-1 text-sm"
              placeholder="Round, e.g. System design"
              value={kind}
              onChange={(e) => setKind(e.target.value)}
            />
            <Button type="submit" size="sm" variant="outline">
              <Plus className="size-3.5" />
              Add
            </Button>
          </form>
        )}
        {error && <p className="text-sm text-destructive">{error}</p>}
      </CardContent>
    </Card>
  )
}
