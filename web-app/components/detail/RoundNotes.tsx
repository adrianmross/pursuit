"use client"

import { useEffect, useState } from "react"
import { Maximize2, Trash2 } from "lucide-react"
import { label, type Interview } from "@/lib/types"
import { InlineText } from "@/components/InlineEdit"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Item, ItemActions, ItemContent, ItemMedia } from "@/components/ui/item"
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Textarea } from "@/components/ui/textarea"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

const day = (value?: string) => (value ? value.slice(0, 10) : "")

// The round's notes outside its questions (Prep, Notes, any prose), each as a
// heading and its text, for the preview under the questions. Questions and
// the "Questions" heading that holds them are shown separately.
function proseSections(md: string) {
  const out: { title: string; body: string }[] = []
  let cur = { title: "", body: [] as string[] }
  let skip = false
  let fence = false
  const flush = () => {
    const body = cur.body.join("\n").trim()
    if (!skip && body) out.push({ title: cur.title, body })
  }
  for (const line of md.split("\n")) {
    if (/^\s*(```|~~~)/.test(line)) fence = !fence
    const h = !fence && line.match(/^\s*#{1,6}\s+(.*?)\s*#*\s*$/)
    if (h) {
      flush()
      cur = { title: h[1], body: [] }
      skip = h[1].endsWith("?") || /^questions$/i.test(h[1])
    } else {
      cur.body.push(line)
    }
  }
  flush()
  return out
}

export type RoundPatch =(item: Interview, body: Record<string, unknown>) => Promise<unknown>

// The round's whole markdown document, open beside the page like a transcript.
function NotesSheet({
  item,
  title,
  open,
  writable,
  onOpenChange,
  onSave,
}: {
  item: Interview
  title: string
  open: boolean
  writable: boolean
  onOpenChange: (open: boolean) => void
  onSave: (notes: string) => Promise<unknown>
}) {
  const [draft, setDraft] = useState(item.notes || "")
  const [saving, setSaving] = useState(false)
  useEffect(() => {
    // Picks up saves (and agent edits) while the sheet is closed.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (!open) setDraft(item.notes || "")
  }, [item.notes, open])
  const dirty = draft !== (item.notes || "")
  async function save() {
    setSaving(true)
    try {
      await onSave(draft)
      onOpenChange(false)
    } finally {
      setSaving(false)
    }
  }
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="flex flex-col gap-0 data-[side=right]:w-full data-[side=right]:sm:max-w-2xl">
        <SheetHeader>
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription>Markdown. A heading that ends in ? is a question, and the text under it is the answer.</SheetDescription>
        </SheetHeader>
        <div className="flex min-h-0 flex-1 px-4">
          <Textarea
            aria-label={`${title} notes`}
            className="min-h-0 flex-1 resize-none font-mono text-[13px] leading-relaxed"
            placeholder={"## Prep\nWhat to read, stories to tell.\n\n## Questions\n### Why this team?\nYour answer.\n\n## Notes\nHow it went."}
            value={draft}
            disabled={!writable}
            onChange={(e) => setDraft(e.target.value)}
          />
        </div>
        {writable && (
          <SheetFooter className="flex-row justify-end">
            <Button variant="ghost" disabled={!dirty || saving} onClick={() => setDraft(item.notes || "")}>
              Discard
            </Button>
            <Button disabled={!dirty || saving} onClick={save}>
              Save notes
            </Button>
          </SheetFooter>
        )}
      </SheetContent>
    </Sheet>
  )
}

// One round: done, name, date and who on one line; its questions and the
// start of its notes below; the full document in a sheet.
export function RoundItem({
  item,
  index,
  writable,
  onPatch,
  onDelete,
}: {
  item: Interview
  index: number
  writable: boolean
  onPatch: RoundPatch
  onDelete: (item: Interview) => void
}) {
  const [sheet, setSheet] = useState(false)
  const [question, setQuestion] = useState("")
  const name = item.kind || `Round ${index}`
  const questions = item.questions ?? []
  const prose = proseSections(item.notes || "")
  // Prep reads before the questions; everything else after them.
  const prep = prose.filter((s) => /^prep$/i.test(s.title))
  const sections = prose.filter((s) => !/^prep$/i.test(s.title))
  const section = (s: { title: string; body: string }, n: number) => (
    <button
      key={n}
      type="button"
      className="line-clamp-2 text-left text-sm whitespace-pre-line text-muted-foreground hover:text-foreground"
      onClick={() => setSheet(true)}
    >
      {s.title && <span className="font-medium text-foreground">{s.title} </span>}
      {s.body}
    </button>
  )

  return (
    <Item variant="outline" size="sm" role="listitem" className="flex-wrap items-start">
      <ItemMedia className="pt-1">
        <Checkbox
          aria-label={`Done: ${name}`}
          checked={!!item.done_at}
          disabled={!writable}
          onCheckedChange={(checked) => onPatch(item, { done: checked === true })}
        />
      </ItemMedia>
      <ItemContent className="min-w-0 gap-2">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <InlineText
            value={item.kind || ""}
            placeholder={`Round ${index}`}
            label="Round name"
            disabled={!writable}
            className="w-auto text-sm font-medium"
            onSave={(v) => onPatch(item, { kind: v })}
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
            label="Interviewer"
            disabled={!writable}
            className="w-auto text-sm text-muted-foreground"
            onSave={(v) => onPatch(item, { interviewer: v })}
          />
        </div>
        {prep.map(section)}
        {questions.length > 0 && (
          <ul className="flex flex-col gap-1">
            {questions.map((q) => (
              <li key={q.prompt} className="text-sm">
                <button type="button" className="w-full text-left hover:underline" onClick={() => setSheet(true)}>
                  <span className="font-medium">{q.prompt}</span>
                  {q.answer ? <span className="ml-2 text-muted-foreground">{q.answer.split("\n")[0]}</span> : <span className="ml-2 text-muted-foreground italic">No answer yet</span>}
                </button>
              </li>
            ))}
          </ul>
        )}
        {sections.map(section)}
        {writable && (
          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (!question.trim()) return
              void onPatch(item, { set_question: { prompt: question.trim(), answer: "" } }).then(() => setQuestion(""))
            }}
          >
            <Input
              className="h-7 text-sm"
              placeholder="Add a question they asked…"
              aria-label={`Add a question to ${name}`}
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
            />
          </form>
        )}
      </ItemContent>
      <ItemActions className="self-start">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon-xs" aria-label="Open notes" onClick={() => setSheet(true)}>
              <Maximize2 className="size-3.5" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>Open notes</TooltipContent>
        </Tooltip>
        {writable && (
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon-xs" aria-label="Delete round" onClick={() => onDelete(item)}>
                <Trash2 className="size-3.5" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Delete round</TooltipContent>
          </Tooltip>
        )}
      </ItemActions>
      <NotesSheet
        item={item}
        title={`${name} · ${label(item.stage)}`}
        open={sheet}
        writable={writable}
        onOpenChange={setSheet}
        onSave={(notes) => onPatch(item, { notes })}
      />
    </Item>
  )
}
