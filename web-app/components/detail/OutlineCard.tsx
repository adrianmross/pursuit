"use client"

import { useEffect, useRef, useState, type ComponentProps, type ReactNode } from "react"
import { Check, ChevronLeft, ChevronRight, Copy, FilePlus, FileText, Inbox, ListPlus, Plus, SlidersHorizontal, Trash2, X } from "lucide-react"
import { api } from "@/lib/api-client"
import { cn } from "@/lib/utils"
import { label, STAGE_STOPS, type Application, type Interview, type Note, type Page, type Question, type Task } from "@/lib/types"
import { AddRoundIcon, RoundPie, useRounds, useTasks } from "@/components/Rounds"
import { Unanswered } from "@/components/PreparedAnswers"
import { InlineText } from "@/components/InlineEdit"
import { SectionCard } from "@/components/detail/SectionCard"
import { Markdown } from "@/components/detail/notes/Markdown"
import { compose, NotesEditor, type NoteSection } from "@/components/detail/notes/NotesEditor"
import { RoundCounter } from "@/components/detail/notes/RoundCounter"
import { SegmentedTabs } from "@/components/detail/notes/SegmentedTabs"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { Input } from "@/components/ui/input"
import { Popover as PopoverPrimitive } from "radix-ui"
import { Popover, PopoverAnchor, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"

const ANYTIME = "anytime"
const day = (value?: string) => (value ? value.slice(0, 10) : "")
const short = (v?: string) => (v ? new Date(`${day(v)}T12:00:00`).toLocaleDateString(undefined, { month: "short", day: "numeric" }) : "")
const roundName = (r: Interview, n: number) => r.kind || `Round ${n + 1}`
const baseStageName = (key: string) => (key === ANYTIME ? "Inbox" : (STAGE_STOPS.find(([k]) => k === key)?.[1] ?? label(key)))
const STAGE_KEYS = [ANYTIME, ...STAGE_STOPS.map(([k]) => k)]

// What the card shows. Layout (outline now; board and timeline to come) is a
// separate choice in the view options, so the tabs stay about content.
type View = "all" | "rounds" | "tasks" | "questions" | "notes"
type Prefs = { view: View; showDone: boolean; hideLeftOut: boolean; sort: "stage" | "date"; layout: "outline" }
const DEFAULT_PREFS: Prefs = { view: "all", showDone: false, hideLeftOut: false, sort: "stage", layout: "outline" }
const PREFS_KEY = "pursuit.outline"

type Q = { prompt: string; answer: string }

// Headings that end in ? and the text under them, as the server reads them.
function parseQuestions(md: string): Q[] {
  const out: Q[] = []
  let cur: { prompt: string; body: string[] } | null = null
  let fence = false
  const flush = () => cur && out.push({ prompt: cur.prompt, answer: cur.body.join("\n").trim() })
  for (const line of md.split("\n")) {
    if (/^\s*(```|~~~)/.test(line)) fence = !fence
    const h = !fence && line.match(/^\s*#{1,6}\s+(.*?)\s*#*\s*$/)
    if (h) {
      flush()
      cur = h[1].endsWith("?") ? { prompt: h[1], body: [] } : null
    } else if (cur) cur.body.push(line)
  }
  flush()
  return out
}

// A question added to the application's own notes, where loose questions live.
function withQuestion(md: string, prompt: string, answer: string) {
  const p = prompt.trim().endsWith("?") ? prompt.trim() : `${prompt.trim()}?`
  return `${md.trimEnd()}${md.trim() ? "\n\n" : ""}### ${p}\n${answer.trim()}\n`
}

// One question changed in place: its heading renamed and the text under it,
// up to the next heading, replaced. A question not there yet is added.
function editQuestion(md: string, from: string, prompt: string, answer: string) {
  const p = prompt.trim().endsWith("?") ? prompt.trim() : `${prompt.trim()}?`
  const lines = md.split("\n")
  const heading = (l: string) => l.match(/^\s*(#{1,6})\s+(.*?)\s*#*\s*$/)
  const i = lines.findIndex((l) => heading(l)?.[2] === from)
  if (i < 0) return withQuestion(md, p, answer)
  let j = i + 1
  while (j < lines.length && !heading(lines[j])) j++
  const body = answer.trim() ? [answer.trim(), ""] : [""]
  return [...lines.slice(0, i), `${heading(lines[i])![1]} ${p}`, ...body, ...lines.slice(j)].join("\n").replace(/\n{3,}/g, "\n\n")
}

// A question taken out: its heading and the text under it, up to the next
// heading.
function dropQuestion(md: string, prompt: string) {
  const lines = md.split("\n")
  const heading = (l: string) => /^\s*#{1,6}\s/.test(l)
  const i = lines.findIndex((l) => heading(l) && l.replace(/^\s*#+\s+/, "").replace(/\s*#*\s*$/, "") === prompt)
  if (i < 0) return md
  let j = i + 1
  while (j < lines.length && !heading(lines[j])) j++
  return [...lines.slice(0, i), ...lines.slice(j)].join("\n").replace(/\n{3,}/g, "\n\n").trim()
}

// A question as a row: click shows the whole answer, double-click edits the
// question and its answer right there.
function QuestionRow({ q, index, writable, onSave, onDelete, onOpen }: { q: Q; index: number; writable: boolean; onSave: (prompt: string, answer: string) => Promise<unknown>; onDelete: () => void; onOpen?: () => void }) {
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(false)
  const [prompt, setPrompt] = useState(q.prompt)
  const [answer, setAnswer] = useState(q.answer)
  const anim = stagger(index)
  const start = () => {
    if (!writable) return
    setPrompt(q.prompt)
    setAnswer(q.answer)
    cancelled.current = false
    setEditing(true)
  }
  // Esc unmounts the fields, which can blur them: that must not save.
  const cancelled = useRef(false)
  const save = async () => {
    if (cancelled.current) return
    setEditing(false)
    if (prompt.trim() && (prompt.trim() !== q.prompt || answer.trim() !== q.answer.trim())) await onSave(prompt, answer)
  }
  if (editing)
    return (
      <div
        className="-mx-2 flex flex-col gap-1 rounded-md bg-muted px-2 py-1.5"
        onBlur={(e) => !e.currentTarget.contains(e.relatedTarget) && void save()}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            cancelled.current = true
            setEditing(false)
          }
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) void save()
        }}
      >
        <input autoFocus aria-label="Question" className="bg-transparent text-sm font-medium outline-none" value={prompt} onChange={(e) => setPrompt(e.target.value)} onKeyDown={(e) => e.key === "Enter" && !e.metaKey && (e.preventDefault(), (e.currentTarget.nextElementSibling as HTMLElement | null)?.focus())} />
        <textarea aria-label="Answer" placeholder="Your answer" className="field-sizing-content min-h-6 resize-none bg-transparent text-sm text-muted-foreground outline-none placeholder:italic" value={answer} onChange={(e) => setAnswer(e.target.value)} />
        <span className="text-[11px] text-muted-foreground/70">⌘↵ or click out saves · Esc cancels</span>
      </div>
    )
  return (
    <div style={anim.style} className={cn("group relative -mx-2 rounded-md px-2 py-1.5 hover:bg-muted", anim.className)} onDoubleClick={start}>
      {writable && (
        <Button variant="ghost" size="icon-xs" className="absolute top-1 right-1 opacity-0 group-hover:opacity-100 focus-visible:opacity-100" aria-label={`Delete ${q.prompt}`} onClick={onDelete} onDoubleClick={(e) => e.stopPropagation()}>
          <X />
        </Button>
      )}
      <button type="button" className="flex w-full flex-col pr-6 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring" onClick={() => setOpen((o) => !o)} aria-expanded={open} title={writable ? "Double-click to edit" : undefined}>
        <span className="truncate text-sm font-medium">{q.prompt}</span>
        {!open && <span className={cn("line-clamp-2 text-sm text-muted-foreground", !q.answer && "italic")}>{q.answer || "No answer yet"}</span>}
      </button>
      {open && (
        <div className="flex flex-col gap-1 pt-1">
          {q.answer ? <Markdown md={q.answer} className="text-muted-foreground" /> : <p className="text-sm text-muted-foreground italic">No answer yet</p>}
          <span className="flex gap-3 text-xs text-muted-foreground">
            {writable && (
              <button type="button" className="hover:text-foreground" onClick={start}>
                Edit
              </button>
            )}
            {onOpen && (
              <button type="button" className="hover:text-foreground" onClick={onOpen}>
                Open in notes
              </button>
            )}
          </span>
        </div>
      )}
    </div>
  )
}

// Rows arrive one after another, like the rows in the landing demo.
// What New file starts from.
const TEMPLATES = [
  { name: "Blank", body: "" },
  { name: "Interview prep", body: "## Prep\n\n## Questions\n\n### A question they might ask?\nYour answer.\n\n## Notes\n" },
  { name: "Questions", body: "### A question?\nIts answer.\n" },
]

const stagger = (i = 0) => ({ className: "animate-in fade-in slide-in-from-bottom-1 fill-mode-both duration-300", style: { animationDelay: `${Math.min(i, 12) * 60}ms` } })

// A row that lights up on hover, like the questions list: a title and a line under it.
function Row({ title, sub, onClick, lead, muted, index }: { title: ReactNode; sub?: ReactNode; onClick?: () => void; lead?: ReactNode; muted?: boolean; index?: number }) {
  const anim = stagger(index)
  const body = (
    <>
      {lead}
      <span className="flex min-w-0 flex-1 flex-col">
        <span className={cn("truncate text-sm font-medium", muted && "font-normal text-muted-foreground")}>{title}</span>
        {sub && <span className="line-clamp-2 text-sm text-muted-foreground">{sub}</span>}
      </span>
    </>
  )
  const cls = cn("-mx-2 flex items-start gap-2.5 rounded-md px-2 py-1.5 text-left hover:bg-muted", anim.className)
  return onClick ? (
    <button type="button" className={cn(cls, "w-[calc(100%+1rem)] outline-none focus-visible:ring-2 focus-visible:ring-ring")} style={anim.style} onClick={onClick}>
      {body}
    </button>
  ) : (
    <div className={cls} style={anim.style}>
      {body}
    </div>
  )
}

// Where a new task or question goes, shown as a small chip at the row's end.
function WherePicker({ value, options, onChange, label: name }: { value: string; options: { id: string; name: string; indent?: boolean }[]; onChange: (v: string) => void; label: string }) {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger size="sm" className="h-6 max-w-40 shrink-0 gap-1 border-0 bg-transparent px-1.5 text-xs text-muted-foreground shadow-none hover:bg-background/60 dark:bg-transparent" aria-label={name}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {options.map((o) => (
          <SelectItem key={o.id} value={o.id} className={cn(o.indent && "pl-6")}>
            {o.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

const bare = "h-auto min-w-0 flex-1 rounded-none border-0 bg-transparent p-0 shadow-none focus-visible:ring-0 dark:bg-transparent"

// Adding a task looks like the task it becomes: the same row, a dashed box
// where the checkbox will be. Enter adds and stays open; Esc closes.
function AddTaskRow({ where: initial, places, onAdd, onAddQuestion, startOpen = false, compact = false, closeOnAdd = false, onClose }: { where: string; places: { id: string; name: string; indent?: boolean }[]; onAdd: (title: string, where: string) => Promise<unknown>; onAddQuestion?: (prompt: string, where: string) => Promise<unknown>; startOpen?: boolean; compact?: boolean; closeOnAdd?: boolean; onClose?: () => void }) {
  const [asQuestion, setAsQuestion] = useState(false)
  const [open, setOpenState] = useState(startOpen)
  const [title, setTitle] = useState("")
  const [where, setWhere] = useState(initial)
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setWhere(initial), [initial])
  const setOpen = (v: boolean) => {
    setOpenState(v)
    if (!v) onClose?.()
  }
  const size = compact ? "min-h-7 text-[13px]" : "min-h-9 text-sm"
  if (!open)
    return (
      <button type="button" className={cn("-mx-2 flex w-[calc(100%+1rem)] items-center gap-2.5 rounded-md px-2 text-left text-muted-foreground outline-none hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring", size)} onClick={() => setOpen(true)}>
        <Plus className="size-4 shrink-0" /> Add task
      </button>
    )
  return (
    <form
      className={cn("-mx-2 flex items-center gap-2.5 rounded-md bg-muted px-2", size)}
      onSubmit={(e) => {
        e.preventDefault()
        if (title.trim())
          void (asQuestion && onAddQuestion ? onAddQuestion(title.trim(), where) : onAdd(title.trim(), where)).then(() => {
            setTitle("")
            if (closeOnAdd) setOpen(false)
          })
      }}
    >
      {onAddQuestion ? (
        <button
          type="button"
          aria-label={asQuestion ? "Adding a question; switch to a task" : "Adding a task; switch to a question"}
          title={asQuestion ? "Question. Click for a task." : "Task. Click for a question."}
          onClick={() => setAsQuestion((v) => !v)}
          className={cn("flex size-4 shrink-0 items-center justify-center border border-dashed border-muted-foreground text-[10px] font-semibold text-muted-foreground outline-none hover:border-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring", asQuestion ? "rounded-full" : "rounded-[4px]")}
        >
          {asQuestion ? "?" : ""}
        </button>
      ) : (
        <span aria-hidden className="size-4 shrink-0 rounded-[4px] border border-dashed border-muted-foreground" />
      )}
      <Input autoFocus className={cn(bare, compact ? "text-[13px]" : "text-sm")} placeholder={asQuestion ? "The question they asked?" : "What needs doing?"} aria-label={asQuestion ? "New question" : "New task"} value={title} onChange={(e) => setTitle(e.target.value)} onKeyDown={(e) => e.key === "Escape" && setOpen(false)} onBlur={(e) => !title.trim() && !e.currentTarget.form?.contains(e.relatedTarget) && setOpen(false)} />
      {!compact && <WherePicker value={where} options={places} onChange={setWhere} label="Where the task goes" />}
    </form>
  )
}

// Adding a question looks like the question it becomes: a title line and the
// answer under it. Enter in the question moves to the answer; Enter there adds.
function AddQuestionRow({ places, onAdd }: { places: { id: string; name: string }[]; onAdd: (where: string, prompt: string, answer: string) => Promise<unknown> }) {
  const [open, setOpen] = useState(false)
  const [prompt, setPrompt] = useState("")
  const [answer, setAnswer] = useState("")
  const [where, setWhere] = useState(places[0]?.id ?? "about")
  const submit = () => {
    if (!prompt.trim()) return
    void onAdd(where, prompt, answer).then(() => {
      setPrompt("")
      setAnswer("")
      document.getElementById("add-question-prompt")?.focus()
    })
  }
  if (!open)
    return (
      <button type="button" className="-mx-2 flex min-h-9 w-[calc(100%+1rem)] items-center gap-2.5 rounded-md px-2 text-left text-sm text-muted-foreground outline-none hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring" onClick={() => setOpen(true)}>
        <Plus className="size-4 shrink-0" /> Add question
      </button>
    )
  return (
    <div
      className="-mx-2 flex items-start gap-2.5 rounded-md bg-muted px-2 py-1.5"
      onKeyDown={(e) => e.key === "Escape" && setOpen(false)}
      onBlur={(e) => !e.currentTarget.contains(e.relatedTarget) && !prompt.trim() && setOpen(false)}
    >
      <span className="flex min-w-0 flex-1 flex-col">
        <Input id="add-question-prompt" autoFocus className={cn(bare, "text-sm font-medium")} placeholder="The question, e.g. Why this team?" aria-label="Question" value={prompt} onChange={(e) => setPrompt(e.target.value)} onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), document.getElementById("add-question-answer")?.focus())} />
        <Textarea
          id="add-question-answer"
          className={cn(bare, "min-h-0 resize-none text-sm text-muted-foreground")}
          rows={Math.max(1, answer.split("\n").length)}
          placeholder="Your answer, or leave it for later"
          aria-label="Answer"
          value={answer}
          onChange={(e) => setAnswer(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && (e.preventDefault(), submit())}
        />
      </span>
      <WherePicker value={where} options={places} onChange={setWhere} label="Where it was asked" />
    </div>
  )
}


// A note read as formatted text; double-click edits its markdown right
// there, like a question. Click out or ⌘↵ saves, Esc cancels.
function NoteBody({ md, writable, label: name, placeholder, onSave, reveal, autoEdit = false }: { md: string; writable: boolean; label: string; placeholder: string; onSave: (next: string) => Promise<unknown>; reveal: boolean; autoEdit?: boolean }) {
  const [editing, setEditing] = useState(autoEdit && writable)
  const [draft, setDraft] = useState(md)
  const cancelled = useRef(false)
  const start = () => {
    if (!writable) return
    setDraft(md)
    cancelled.current = false
    setEditing(true)
  }
  const save = () => {
    if (cancelled.current) return
    setEditing(false)
    if (draft.trim() !== md.trim()) void onSave(draft.trim())
  }
  if (editing)
    return (
      <div className="-mx-2 flex flex-col gap-1 rounded-md bg-muted px-2 py-1.5">
        <textarea
          autoFocus
          aria-label={name}
          placeholder={placeholder}
          className="field-sizing-content min-h-6 resize-none bg-transparent text-sm leading-relaxed outline-none placeholder:text-muted-foreground/60"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onBlur={save}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              cancelled.current = true
              setEditing(false)
            }
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) save()
          }}
        />
        <span className="text-[11px] text-muted-foreground/70">Markdown · a heading that ends in ? is a question · ⌘↵ or click out saves · Esc cancels</span>
      </div>
    )
  return (
    <div className={cn("-mx-2 min-h-6 rounded-md px-2 py-1", writable && "cursor-text hover:bg-muted/60")} onDoubleClick={start} title={writable ? "Double-click to edit" : undefined}>
      {md.trim() ? <Markdown md={md} active={reveal} /> : <p className="text-sm text-muted-foreground italic">{writable ? "Double-click to write" : "Nothing written yet."}</p>}
    </div>
  )
}

// A round with no notes offers to start them (Show in notes); once it has
// some, whether they go into overall.md (Show in overall). The same control
// in All, the round's details and the Notes tab.
function NotesToggle({ r, writable, onStart, onPatch }: { r: Interview; writable: boolean; onStart: () => void; onPatch: (body: Record<string, unknown>) => Promise<unknown> }) {
  if (!r.notes?.trim())
    return writable ? (
      <Button variant="ghost" size="xs" className="shrink-0 text-muted-foreground" onClick={onStart}>
        <FilePlus /> Show in notes
      </Button>
    ) : null
  return (
    <label className="flex shrink-0 items-center gap-1.5 text-xs text-muted-foreground">
      <Checkbox checked={r.in_notes !== false} disabled={!writable} onCheckedChange={(v) => void onPatch({ in_notes: v === true })} />
      Show in overall
    </label>
  )
}

// One line of the notes: ▸ Title ─── meta, opening to the note, with an
// action such as NotesToggle beside it.
function Fold({ id, title, meta, open, onToggle, dim, action, children }: { id: string; title: string; meta?: string; open: boolean; onToggle: () => void; dim?: boolean; action?: ReactNode; children: ReactNode }) {
  return (
    <div id={`note-${id}`} className="scroll-mt-24">
      <div className="flex items-center gap-2">
        <button type="button" className="flex min-w-0 flex-1 items-center gap-2.5 rounded-sm py-1.5 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring" onClick={onToggle} aria-expanded={open}>
          <span aria-hidden className="w-3 shrink-0 font-mono text-xs text-muted-foreground">
            {open ? "▾" : "▸"}
          </span>
          <span className={cn("truncate text-sm font-medium", dim && "font-normal text-muted-foreground")}>{title}</span>
          <span aria-hidden className="h-px min-w-4 flex-1 bg-border" />
          <span className="shrink-0 text-xs text-muted-foreground">{meta}</span>
        </button>
        {action}
      </div>
      {open && <div className="pt-1 pb-3 pl-5">{children}</div>}
    </div>
  )
}

// A round's details, one click away instead of on the page.
function RoundDetails({ r, name, writable, onPatch, onDelete, onOpenNotes, open, onOpenChange, children }: { r: Interview; name: string; writable: boolean; onPatch: (body: Record<string, unknown>) => Promise<unknown>; onDelete: () => void; onOpenNotes: () => void; open?: boolean; onOpenChange?: (open: boolean) => void; children: ReactNode }) {
  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      {open === undefined ? <PopoverTrigger asChild>{children}</PopoverTrigger> : <PopoverAnchor asChild>{children}</PopoverAnchor>}
      <PopoverContent align="start" side="right" className="flex w-72 flex-col gap-3 text-sm">
        <InlineText value={r.kind || ""} placeholder={name} label="Round name" disabled={!writable} className="font-medium" onSave={(v) => onPatch({ kind: v })} />
        <div className="grid grid-cols-[64px_1fr] items-center gap-x-3 gap-y-2 text-muted-foreground">
          <span>Date</span>
          <Input type="date" aria-label="Date" className="h-7 text-sm" value={day(r.scheduled_at)} disabled={!writable} onChange={(e) => void onPatch({ scheduled_at: e.target.value })} />
          <span>With</span>
          <InlineText value={r.interviewer || ""} placeholder="Who" label="Interviewer" disabled={!writable} className="text-foreground" onSave={(v) => onPatch({ interviewer: v })} />
        </div>
        <label className="flex items-center gap-2">
          <Checkbox checked={!!r.done_at} disabled={!writable} onCheckedChange={(v) => void onPatch({ done: v === true })} />
          Done
        </label>
        <div className="flex items-center justify-between">
          <NotesToggle r={r} writable={writable} onStart={onOpenNotes} onPatch={onPatch} />
        </div>
        <div className="flex items-center justify-between">
          <Button variant="outline" size="sm" onClick={onOpenNotes}>
            <FileText /> Open notes
          </Button>
          {writable && (
            <Button variant="ghost" size="sm" className="text-muted-foreground" onClick={onDelete}>
              <Trash2 /> Delete
            </Button>
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}

// A stage's name in this application. Double-click to rename it; a renamed
// stage keeps the status it stands for, shown after an arrow.
function StageLabel({ name, base, current, reached, writable, onRename }: { name: string; base: string; current: boolean; reached: boolean; writable: boolean; onRename: (name: string) => Promise<unknown> }) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(name)
  if (editing)
    return (
      <form
        className="min-w-0 flex-1"
        onSubmit={(e) => {
          e.preventDefault()
          setEditing(false)
          if (draft.trim() !== name) void onRename(draft)
        }}
      >
        <input autoFocus aria-label={`Name for ${base}`} placeholder={base} className="w-full bg-transparent text-[11px] tracking-wide text-foreground uppercase outline-none" value={draft} onChange={(e) => setDraft(e.target.value)} onBlur={(e) => e.currentTarget.form?.requestSubmit()} onKeyDown={(e) => e.key === "Escape" && setEditing(false)} />
      </form>
    )
  return (
    <span
      onDoubleClick={() => {
        if (!writable) return
        setDraft(name)
        setEditing(true)
      }}
      title={writable ? "Double-click to rename" : undefined}
      className={cn("min-w-0 truncate text-[11px] tracking-wide uppercase", current ? "font-semibold text-foreground" : reached ? "text-muted-foreground" : "text-muted-foreground/60", writable && "cursor-text")}
    >
      {name}
      {name !== base && <span className="ml-1 font-normal text-muted-foreground/60 normal-case">→ {base}</span>}
    </span>
  )
}

// A round as one compact line: a dot (filled once done), its name, then date,
// who and questions. Click for its details; double-click to rename it.
function RoundChip({ r, name, prefix, index, writable, tasks = 0, onTasks, onQuestions, onRename, details }: { r: Interview; name: string; prefix?: string; index: number; writable: boolean; tasks?: number; onTasks?: () => void; onQuestions?: () => void; onRename: (kind: string) => Promise<unknown>; details: (trigger: ReactNode) => ReactNode }) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(name)
  const anim = stagger(index)
  if (editing)
    return (
      <form
        className="flex h-7 items-center gap-2 rounded-md bg-muted px-2"
        onSubmit={(e) => {
          e.preventDefault()
          const v = draft.trim()
          setEditing(false)
          if (v && v !== name) void onRename(v)
        }}
      >
        <span aria-hidden className="size-2 shrink-0 rounded-full border border-dashed border-foreground" />
        <input autoFocus aria-label="Round name" className="min-w-0 flex-1 bg-transparent text-[13px] font-medium outline-none" value={draft} onChange={(e) => setDraft(e.target.value)} onBlur={(e) => e.currentTarget.form?.requestSubmit()} onKeyDown={(e) => e.key === "Escape" && setEditing(false)} />
      </form>
    )
  return (
    <>
      {details(
        <button
          type="button"
          style={anim.style}
          onDoubleClick={() => {
            if (!writable) return
            setDraft(name)
            setEditing(true)
          }}
          className={cn("flex h-7 w-full min-w-0 items-center gap-2 rounded-md px-2 text-left text-[13px] outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring", r.in_notes === false && "text-muted-foreground", anim.className)}
        >
          <span aria-hidden className={cn("size-2 shrink-0 rounded-full border border-foreground", r.done_at && "bg-foreground")} />
          {prefix && <span className="text-muted-foreground">{prefix} ·</span>}
          <span className="truncate font-medium">{name}</span>
          <span className="ml-auto flex shrink-0 items-center gap-1.5 text-xs text-muted-foreground">
            {[short(r.scheduled_at), r.interviewer].filter(Boolean).join(" · ")}
            {tasks > 0 && (
              <span
                role="link"
                tabIndex={-1}
                className="rounded px-1 hover:bg-background hover:text-foreground"
                onClick={(e) => {
                  e.stopPropagation()
                  onTasks?.()
                }}
              >
                {tasks} {tasks === 1 ? "task" : "tasks"}
              </span>
            )}
            {!!r.questions?.length && (
              <span
                role="link"
                tabIndex={-1}
                className="rounded px-1 hover:bg-background hover:text-foreground"
                onClick={(e) => {
                  e.stopPropagation()
                  onQuestions?.()
                }}
              >
                {r.questions.length}?
              </span>
            )}
          </span>
        </button>,
      )}
    </>
  )
}

// Filters, sort and layout, behind one icon so the tabs stay few.
function ViewOptions({ prefs, onChange }: { prefs: Prefs; onChange: (p: Partial<Prefs>) => void }) {
  const row = "flex items-center justify-between gap-4 text-sm"
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label="View options">
          <SlidersHorizontal />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="flex w-64 flex-col gap-3">
        <label className={row}>
          Show done tasks
          <Switch checked={prefs.showDone} onCheckedChange={(v) => onChange({ showDone: v })} />
        </label>
        <label className={row}>
          Hide hidden rounds
          <Switch checked={prefs.hideLeftOut} onCheckedChange={(v) => onChange({ hideLeftOut: v })} />
        </label>
        <div className={row}>
          Sort
          <ToggleGroup type="single" variant="outline" size="sm" value={prefs.sort} onValueChange={(v) => v && onChange({ sort: v as Prefs["sort"] })}>
            <ToggleGroupItem value="stage" className="px-2.5 text-xs">
              Stage
            </ToggleGroupItem>
            <ToggleGroupItem value="date" className="px-2.5 text-xs">
              Date
            </ToggleGroupItem>
          </ToggleGroup>
        </div>
        <div className="flex flex-col gap-1.5 border-t pt-3">
          <span className="text-xs text-muted-foreground">Layout</span>
          <ToggleGroup type="single" variant="outline" size="sm" value={prefs.layout}>
            <ToggleGroupItem value="outline" className="px-2.5 text-xs">
              Outline
            </ToggleGroupItem>
            <ToggleGroupItem value="board" disabled className="px-2.5 text-xs">
              Board
            </ToggleGroupItem>
            <ToggleGroupItem value="timeline" disabled className="px-2.5 text-xs">
              Timeline
            </ToggleGroupItem>
          </ToggleGroup>
          <span className="text-xs text-muted-foreground">Board and Timeline are on the way.</span>
        </div>
      </PopoverContent>
    </Popover>
  )
}

// The compiled notes, exactly as an agent reads them from GET /notes.
function CompiledNotes({ appId, open, onOpenChange }: { appId: string; open: boolean; onOpenChange: (v: boolean) => void }) {
  const [md, setMd] = useState<string | null>(null)
  useEffect(() => {
    if (!open) return
    api<{ markdown: string }>(`/v1/applications/${encodeURIComponent(appId)}/notes`)
      .then((d) => setMd(d.markdown))
      .catch(() => setMd("Couldn't load the notes."))
  }, [appId, open])
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="flex flex-col gap-0 data-[side=right]:w-full data-[side=right]:sm:max-w-2xl">
        <SheetHeader>
          <SheetTitle>Markdown</SheetTitle>
          <SheetDescription>The whole notes document as one file, the same text as pursuit apps notes.</SheetDescription>
        </SheetHeader>
        <pre className="min-h-0 flex-1 overflow-auto px-4 pb-6 font-mono text-[13px] leading-relaxed whitespace-pre-wrap">{md ?? "Loading…"}</pre>
      </SheetContent>
    </Sheet>
  )
}

// NEXT brings you to its task: scrolled into view and focused.
function useFocus(id: string | undefined) {
  useEffect(() => {
    if (!id) return
    const frame = requestAnimationFrame(() => {
      const row = document.getElementById(`task-${id}`)
      row?.scrollIntoView({ block: "center", behavior: "smooth" })
      row?.querySelector<HTMLElement>("button")?.focus({ preventScroll: true })
    })
    return () => cancelAnimationFrame(frame)
  }, [id])
}

// Rounds, tasks, questions and notes for one application. Tabs pick what to
// show; All puts the next step, the outline and the notes together.
export function OutlineCard({ application, writable, onChange, onSaveNotes, onPatch }: { application: Application; writable: boolean; onChange: () => Promise<void>; onSaveNotes: (notes: string) => Promise<unknown>; onPatch: (patch: Record<string, unknown>) => Promise<unknown> }) {
  // This application's own stage names and skipped stages. The status each
  // stands for never changes; only what it is called and whether it shows.
  const plan = application.stage_plan
  const stageName = (key: string) => plan?.labels?.[key] || baseStageName(key)
  const isOff = (key: string) => !!plan?.off?.includes(key)
  const savePlan = (labels: Record<string, string>, off: string[]) => onPatch({ stage_plan: { labels, off } })
  const rename = (key: string, name: string) => {
    const labels = { ...(plan?.labels ?? {}) }
    if (name.trim() && name.trim() !== baseStageName(key)) labels[key] = name.trim()
    else delete labels[key]
    return savePlan(labels, plan?.off ?? [])
  }
  const setOn = (key: string, on: boolean) => savePlan(plan?.labels ?? {}, on ? (plan?.off ?? []).filter((k) => k !== key) : [...(plan?.off ?? []), key])
  const appId = application.id
  const current = application.stage ?? 0
  const currentKey = STAGE_STOPS[current][0]
  const rounds = useRounds(appId, current, onChange)
  const tasks = useTasks(appId, current, onChange)
  const [form, setForm] = useState<Question[]>([])
  const [shut, setShut] = useState<Record<string, boolean>>({})
  const [open, setOpen] = useState<Record<string, boolean>>({})
  const [justOpened, setJustOpened] = useState<string | null>(null)
  const [marked, setMarked] = useState<string | null>(null)
  const [editTask, setEditTask] = useState<string | null>(null)
  // The round whose details popover the outline opened by double-click.
  const [detailsFor, setDetailsFor] = useState<string | null>(null)
  const [addingAt, setAddingAt] = useState<string | null>(null)
  // Switching to a shorter view would pull the page up under you when you are
  // scrolled near the bottom. Hold the old height, then let it go only as far
  // as it can without moving anything, as you scroll.
  const bodyRef = useRef<HTMLDivElement>(null)
  const innerRef = useRef<HTMLDivElement>(null)
  const [hold, setHold] = useState(0)
  useEffect(() => {
    if (!hold) return
    const release = () => {
      const natural = innerRef.current?.offsetHeight ?? 0
      const slack = document.documentElement.scrollHeight - (window.scrollY + window.innerHeight)
      setHold((h) => {
        const next = Math.max(natural, h - Math.max(0, slack))
        return next <= natural ? 0 : next
      })
    }
    release()
    window.addEventListener("scroll", release, { passive: true })
    window.addEventListener("resize", release)
    return () => {
      window.removeEventListener("scroll", release)
      window.removeEventListener("resize", release)
    }
  }, [hold])
  // New tasks from the Tasks view land on the stage the application is on.
  const taskStage = currentKey
  const [compiled, setCompiled] = useState(false)
  // Notes files of their own, and which file the Notes tab has open.
  const [noteFiles, setNoteFiles] = useState<Note[]>([])
  // null shows the files as a grid; an id opens that file.
  const [noteFile, setNoteFile] = useState<string | null>(null)
  // A file just made with New file, whose name field starts selected.
  const [fresh, setFresh] = useState<string | null>(null)
  const [copied, setCopied] = useState<string | null>(null)
  const filesPath = `/v1/applications/${encodeURIComponent(appId)}/notes/files`
  const loadFiles = () => api<Page<Note>>(filesPath).then((p) => setNoteFiles(p.data || []))
  const createFile = async (name: string, body: string) => {
    const f = await api<Note>(filesPath, { method: "POST", body: { name, body } })
    await loadFiles()
    setNoteFile(f.id)
    return f
  }
  const saveFile = async (f: Note, body: Record<string, string>) => {
    await api(`/v1/notes/${encodeURIComponent(f.id)}`, { method: "PATCH", body: { version: f.version, ...body } })
    await loadFiles()
  }
  const deleteFile = async (f: Note) => {
    await api(`/v1/notes/${encodeURIComponent(f.id)}?version=${f.version}`, { method: "DELETE" })
    await loadFiles()
  }
  const [prefs, setPrefsState] = useState<Prefs>(DEFAULT_PREFS)

  useEffect(() => {
    rounds.fetchRounds().catch(() => {})
    tasks.fetchTasks().catch(() => {})
    loadFiles().catch(() => {})
    api<Page<Question>>(`/v1/applications/${encodeURIComponent(appId)}/questions`)
      .then((page) => setForm(page.data || []))
      .catch(() => {})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [appId])
  useEffect(() => {
    // Read after hydration so the server render matches the first client one.
    try {
      const saved = JSON.parse(localStorage.getItem(PREFS_KEY) || "null")
      // eslint-disable-next-line react-hooks/set-state-in-effect
      if (saved) setPrefsState({ ...DEFAULT_PREFS, ...saved })
    } catch {
      // Unreadable or blocked storage: keep the defaults.
    }
  }, [])
  const setPrefs = (next: Partial<Prefs>) => {
    if (next.view && next.view !== prefs.view) setHold(bodyRef.current?.offsetHeight ?? 0)
    const merged = { ...prefs, ...next }
    setPrefsState(merged)
    try {
      localStorage.setItem(PREFS_KEY, JSON.stringify(merged))
    } catch {
      // Private windows: the choice lasts until reload.
    }
  }

  const all = rounds.list ?? []
  const ordered = STAGE_STOPS.flatMap(([key]) => rounds.storedFor(all, key).map((r, n) => ({ r, n })))
  const byDate = (list: { r: Interview; n: number }[]) => [...list].sort((a, b) => (a.r.scheduled_at || "9999").localeCompare(b.r.scheduled_at || "9999"))
  const sorted = (list: { r: Interview; n: number }[]) => (prefs.sort === "date" ? byDate(list) : list)
  const notesRounds = sorted(prefs.hideLeftOut ? ordered.filter(({ r }) => r.in_notes !== false) : ordered)

  // Everything waiting on you: open tasks, form questions prepare could not
  // answer, and questions in your notes still without an answer.
  const allTasks = tasks.list ?? []
  const openTasks = allTasks.filter((t) => !t.done_at)
  const toAnswer = form.filter((q) => q.required && !q.answer)
  const looseQuestions = parseQuestions(application.notes || "")
  const unanswered = [
    ...looseQuestions.filter((q) => !q.answer).map((q) => ({ q, stage: ANYTIME, note: "about" })),
    ...ordered.flatMap(({ r }) => (r.questions ?? []).filter((q) => !q.answer).map((q) => ({ q, stage: r.stage, note: r.id }))),
  ]
  const nextTask = openTasks.find((t) => t.stage === currentKey) ?? openTasks.find((t) => t.stage === ANYTIME) ?? openTasks[0]
  const next = nextTask
    ? { title: nextTask.title, where: stageName(nextTask.stage) }
    : toAnswer[0]
      ? { title: `Answer: ${toAnswer[0].prompt}`, where: "Apply" }
      : unanswered[0]
        ? { title: `Answer: ${unanswered[0].q.prompt}`, where: stageName(unanswered[0].stage) }
        : null
  const waiting = openTasks.length + toAnswer.length + unanswered.length
  const questionCount = looseQuestions.length + ordered.reduce((n, { r }) => n + (r.questions?.length ?? 0), 0) + form.length
  useFocus(prefs.view === "tasks" ? nextTask?.id : undefined)
  // NEXT in All jumps to the task in the outline, under its stage or round,
  // and marks it until the pointer finds it.
  // Drag a task onto a stage or a round in the outline to move it there.
  const [dropAt, setDropAt] = useState<string | null>(null)
  const [armedDel, setArmedDel] = useState<string | null>(null)
  const moveTask = async (id: string, where: string) => {
    const t = allTasks.find((x) => x.id === id)
    if (!t) return
    const body = where.startsWith("round:") ? { round_id: where.slice(6) } : { stage: where.slice(6), round_id: "" }
    const already = where.startsWith("round:") ? t.round_id === where.slice(6) : !t.round_id && t.stage === where.slice(6)
    if (already) return
    await api(`/v1/tasks/${encodeURIComponent(id)}`, { method: "PATCH", body: { version: t.version, ...body } })
    await tasks.fetchTasks()
    await onChange()
  }
  const dropTarget = (where: string) =>
    writable
      ? {
          onDragOver: (e: React.DragEvent) => {
            if (!e.dataTransfer.types.includes("text/x-pursuit-task")) return
            e.preventDefault()
            if (dropAt !== where) setDropAt(where)
          },
          onDragLeave: () => dropAt === where && setDropAt(null),
          onDrop: (e: React.DragEvent) => {
            const id = e.dataTransfer.getData("text/x-pursuit-task")
            setDropAt(null)
            if (id) void moveTask(id, where)
          },
        }
      : {}

  // From a round's counts to its questions or tasks: switch view, scroll, glow.
  const jumpTo = (view: View, target: string) => {
    if (!target) return
    setPrefs({ view })
    setMarked(target)
    requestAnimationFrame(() => requestAnimationFrame(() => (document.getElementById(`mark-${target}`) ?? document.getElementById(`task-${target}`))?.scrollIntoView({ block: "center", behavior: "smooth" })))
  }
  const jumpToNext = () => {
    if (!nextTask) return setPrefs({ view: "tasks" })
    setShut((x) => ({ ...x, [nextTask.stage]: false }))
    setMarked(nextTask.id)
    requestAnimationFrame(() => {
      const row = document.getElementById(`task-${nextTask.id}`)
      row?.scrollIntoView({ block: "center", behavior: "smooth" })
      row?.querySelector<HTMLElement>("button")?.focus({ preventScroll: true })
    })
  }

  const patch = (r: Interview, body: Record<string, unknown>) =>
    rounds.run(() => api(`/v1/rounds/${encodeURIComponent(r.id)}`, { method: "PATCH", body: { version: r.version, ...body } }))
  const remove = (r: Interview) => rounds.run(() => api(`/v1/rounds/${encodeURIComponent(r.id)}?version=${r.version}`, { method: "DELETE" }))
  const isOpen = (id: string, fallback = false) => open[id] ?? fallback
  const toggle = (id: string, fallback = false) => {
    const now = isOpen(id, fallback)
    if (!now) setJustOpened(id)
    setOpen((o) => ({ ...o, [id]: !now }))
  }
  const reveal = (id: string) => {
    setJustOpened(id)
    setOpen((o) => ({ ...o, [id]: true }))
    requestAnimationFrame(() => document.getElementById(`note-${id}`)?.scrollIntoView({ block: "nearest", behavior: "smooth" }))
  }
  const go = (view: View, noteId?: string) => {
    setPrefs({ view })
    if (view === "notes" && noteId) setNoteFile(noteId)
    else if (noteId) reveal(noteId)
  }
  // Show in notes on a round with none: open it in All, already editing.
  const [started, setStarted] = useState<string | null>(null)
  const startNotes = (id: string) => {
    setStarted(id)
    reveal(id)
  }
  const addQuestion = (where: string, prompt: string, answer: string): Promise<unknown> => {
    if (where === "about") return onSaveNotes(withQuestion(application.notes || "", prompt, answer))
    const r = all.find((x) => x.id === where)
    return r ? patch(r, { set_question: { prompt, answer } }) : Promise.resolve()
  }
  const saveQuestion = (where: string, from: string, prompt: string, answer: string): Promise<unknown> => {
    if (where === "about") return onSaveNotes(editQuestion(application.notes || "", from, prompt, answer))
    const r = all.find((x) => x.id === where)
    if (!r) return Promise.resolve()
    return prompt.trim().replace(/\?*$/, "?") === from ? patch(r, { set_question: { prompt: from, answer } }) : patch(r, { notes: editQuestion(r.notes || "", from, prompt, answer) })
  }
  const deleteQuestion = (where: string, prompt: string): Promise<unknown> => {
    if (where === "about") return onSaveNotes(dropQuestion(application.notes || "", prompt))
    const r = all.find((x) => x.id === where)
    return r ? patch(r, { notes: dropQuestion(r.notes || "", prompt) }) : Promise.resolve()
  }
  const places = [{ id: "about", name: "Not tied to a round" }, ...ordered.map(({ r, n }) => ({ id: r.id, name: `${roundName(r, n)} · ${stageName(r.stage)}` }))]
  const meta = (r: Interview) => [stageName(r.stage), short(r.scheduled_at), r.questions?.length ? `${r.questions.length}?` : ""].filter(Boolean).join(" · ")

  const roundOf = (t: Task) => {
    const at = ordered.find(({ r }) => r.id === t.round_id)
    return at ? roundName(at.r, at.n) : ""
  }
  const taskRow = (t: Task, i = 0, compact = false) => {
    const anim = stagger(i)
    const isMarked = marked === t.id
    return (
      <div
        key={t.id}
        id={`task-${t.id}`}
        draggable={writable && editTask !== t.id}
        onDragStart={(e) => {
          e.dataTransfer.setData("text/x-pursuit-task", t.id)
          e.dataTransfer.effectAllowed = "move"
        }}
        onMouseEnter={() => isMarked && setMarked(null)}
        style={anim.style}
        className={cn("group relative -mx-2 flex scroll-mt-24 items-center gap-2.5 rounded-md px-2 hover:bg-muted", compact ? "min-h-7 text-[13px]" : "min-h-9 text-sm", anim.className, isMarked && "marked-glow", editTask === t.id && "bg-muted")}
      >
        <Checkbox aria-label={`Done: ${t.title}`} checked={!!t.done_at} disabled={!writable} onCheckedChange={() => void tasks.toggle(t)} />
        {editTask === t.id ? (
          <input
            autoFocus
            aria-label="Task"
            defaultValue={t.title}
            className="min-w-0 flex-1 bg-transparent outline-none"
            onBlur={(e) => {
              const v = e.currentTarget.value.trim()
              setEditTask(null)
              if (v && v !== t.title) void tasks.rename(t, v)
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter") e.currentTarget.blur()
              if (e.key === "Escape") {
                e.currentTarget.value = t.title
                e.currentTarget.blur()
              }
            }}
          />
        ) : (
          <span onDoubleClick={() => writable && setEditTask(t.id)} title={writable ? "Double-click to edit" : undefined} className={cn("min-w-0 flex-1 truncate", writable && "cursor-text", t.done_at && "text-muted-foreground line-through")}>
            {t.title}
          </span>
        )}
        {!compact && t.round_id && <span className="shrink-0 text-xs text-muted-foreground">{roundOf(t)}</span>}
        {!compact && t.id === nextTask?.id && <span className="text-[10px] font-semibold tracking-wider text-muted-foreground">NEXT</span>}
        {writable && (
          <Button variant="ghost" size="icon-xs" className="opacity-0 group-hover:opacity-100 focus-visible:opacity-100" aria-label={`Delete ${t.title}`} onClick={() => void tasks.remove(t)}>
            <X />
          </Button>
        )}
      </div>
    )
  }
  const taskPlaces = [
    { id: `stage:${ANYTIME}`, name: "Inbox" },
    ...STAGE_STOPS.filter(([key]) => !isOff(key)).flatMap(([key]) => [{ id: `stage:${key}`, name: stageName(key) }, ...rounds.storedFor(all, key).map((r, n) => ({ id: `round:${r.id}`, name: roundName(r, n), indent: true }))]),
  ]
  // Where a question added from a task row goes: the round itself, else the
  // application's notes (Inbox and stages without a round).
  const questionPlace = (where: string) => (where.startsWith("round:") ? where.slice(6) : "about")
  const addTask = (title: string, where: string) => (where.startsWith("round:") ? tasks.add(title, undefined, where.slice(6)) : tasks.add(title, where.slice(6)))

  // Tasks, grouped Inbox first, with answers waiting where they belong.
  const groups = STAGE_KEYS.map((key) => ({
    key,
    items: allTasks.filter((t) => t.stage === key && (prefs.showDone || !t.done_at)),
    form: key === "to_apply" ? toAnswer : [],
    notes: unanswered.filter((u) => u.stage === key),
  })).filter((g) => g.items.length || g.form.length || g.notes.length)
  const doneCount = allTasks.filter((t) => t.done_at).length
  const tasksView = (
    <div className="flex flex-col gap-4">
      {groups.map((g) => (
        <section key={g.key} className="flex flex-col">
          <h3 className="flex items-center gap-1.5 pb-1 text-xs text-muted-foreground">
            {g.key === ANYTIME && <Inbox className="size-3.5" />}
            {stageName(g.key)}
          </h3>
          {g.items.map((t, i) => taskRow(t, i))}
          {g.form.map((q) => (
            <Collapsible key={q.prompt}>
              <CollapsibleTrigger className="-mx-2 flex min-h-9 w-[calc(100%+1rem)] items-center gap-2.5 rounded-md px-2 text-left text-sm hover:bg-muted">
                <span aria-hidden className="size-4 shrink-0 rounded-[5px] border border-dashed border-muted-foreground" />
                <span className="flex-1">Answer: {q.prompt}</span>
                <span className="text-xs text-muted-foreground">form</span>
              </CollapsibleTrigger>
              <CollapsibleContent className="py-2 pl-6">{q.prompt && <Unanswered prompt={q.prompt} />}</CollapsibleContent>
            </Collapsible>
          ))}
          {g.notes.map((u) => (
            <Row key={`${u.note}:${u.q.prompt}`} lead={<span aria-hidden className="mt-0.5 size-4 shrink-0 rounded-[5px] border border-dashed border-muted-foreground" />} title={`Answer: ${u.q.prompt}`} sub="A question in your notes" onClick={() => go("notes", u.note)} />
          ))}
        </section>
      ))}
      {!groups.length && <p className="text-sm text-muted-foreground">Nothing waiting on you.</p>}
      {/* Sits right under the last task, not a group's gap away. */}
      <div className={cn("flex flex-col", groups.length > 0 && "-mt-4")}>
        {writable && <AddTaskRow where={`stage:${taskStage}`} places={taskPlaces} onAdd={addTask} />}
        {doneCount > 0 && (
          <button type="button" className="self-start pt-1 text-xs text-muted-foreground hover:text-foreground" onClick={() => setPrefs({ showDone: !prefs.showDone })}>
            {prefs.showDone ? "Hide done" : `${doneCount} done`}
          </button>
        )}
      </div>
    </div>
  )

  const questionGroups = [
    { id: "about", title: "Not tied to a round", items: looseQuestions },
    ...ordered.filter(({ r }) => r.questions?.length).map(({ r, n }) => ({ id: r.id, title: `${roundName(r, n)} · ${stageName(r.stage)}`, items: r.questions ?? [] })),
  ].filter((g) => g.items.length)
  const questionsView = (
    <div className="flex flex-col gap-5">
      {questionGroups.map((g) => (
        <section key={g.id} id={`mark-${g.id}`} onMouseEnter={() => marked === g.id && setMarked(null)} className={cn("-mx-2 flex scroll-mt-24 flex-col rounded-lg px-2 py-1", marked === g.id && "marked-glow")}>
          <h3 className="pb-1 text-xs text-muted-foreground">{g.title}</h3>
          {g.items.map((q, i) => (
            <QuestionRow key={q.prompt} q={q} index={i} writable={writable} onSave={(prompt, answer) => saveQuestion(g.id, q.prompt, prompt, answer)} onDelete={() => void deleteQuestion(g.id, q.prompt)} onOpen={() => go("notes", g.id)} />
          ))}
        </section>
      ))}
      {form.length > 0 && (
        <section className="flex flex-col">
          <h3 className="pb-1 text-xs text-muted-foreground">Application form</h3>
          {form.map((q, i) => (
            <Row key={q.prompt} index={i} title={q.prompt} sub={q.answer || (q.prompt && writable ? <Unanswered prompt={q.prompt} /> : "No answer yet")} />
          ))}
        </section>
      )}
      {!questionGroups.length && !form.length && <p className="text-sm text-muted-foreground">No questions yet. In any note, a heading that ends in ? is a question.</p>}
      {/* Right under the last question: less the gap, and the group's padding. */}
      {writable && (
        <div className={cn(form.length ? "-mt-5" : questionGroups.length > 0 && "-mt-6")}>
          <AddQuestionRow places={places} onAdd={addQuestion} />
        </div>
      )}
    </div>
  )

  // Rounds as one rail of named chips. The stage each sits in is only a small
  // grey label between them: it still decides where the round counts.
  const chip = (r: Interview, n: number, k: number, prefix?: string) => (
    <RoundChip
      key={r.id}
      r={r}
      name={roundName(r, n)}
      prefix={prefix}
      index={k}
      writable={writable}
      tasks={allTasks.filter((t) => t.round_id === r.id && !t.done_at).length}
      onTasks={() => jumpTo("tasks", allTasks.find((t) => t.round_id === r.id && !t.done_at)?.id ?? "")}
      onQuestions={() => jumpTo("questions", r.id)}
      onRename={(kind) => patch(r, { kind })}
      details={(trigger) => (
        <RoundDetails r={r} name={roundName(r, n)} writable={writable} onPatch={(body) => patch(r, body)} onDelete={() => void remove(r)} onOpenNotes={() => go("notes", r.id)}>
          {trigger}
        </RoundDetails>
      )}
    />
  )
  // Rounds to a number: add or drop one at a time, from the end.
  const setCount = async (key: string, i: number, target: number) => {
    const shown = Math.max(1, rounds.storedFor(all, key).length)
    if (target > shown) for (let n = shown; n < target; n++) await rounds.add(key, i)
    else for (let n = shown; n > target; n--) await rounds.remove(key)
  }
  let k = 0
  // A grey stage label on the left, its rounds as lines on the right.
  const roundsView = (
    <div className="flex flex-col">
      {prefs.sort === "date"
        ? byDate(ordered).map(({ r, n }) => chip(r, n, k++, stageName(r.stage)))
        : STAGE_STOPS.slice(1, -1).map(([key], j) => {
            const i = j + 1
            const stored = rounds.storedFor(all, key)
            const off = isOff(key)
            const name = stageName(key)
            return (
              <div key={key} className={cn("grid grid-cols-[minmax(0,210px)_minmax(0,1fr)] items-start gap-x-3 border-t py-1 first:border-t-0", off && "opacity-50")}>
                <div className="flex h-7 items-center gap-2">
                  {writable && <Switch className="scale-75" checked={!off} aria-label={`${baseStageName(key)} in this process`} onCheckedChange={(v) => void setOn(key, v)} />}
                  <StageLabel name={name} base={baseStageName(key)} current={i === current} reached={i < current} writable={writable && !off} onRename={(v) => rename(key, v)} />
                  <span className="flex-1" />
                  {!off && (
                    <RoundCounter value={Math.max(1, stored.length)} label={`${name} rounds`} disabled={!writable || rounds.busy} onChange={(n) => void setCount(key, i, n)} />
                  )}
                </div>
                <div className="flex min-w-0 flex-col">
                  {off ? (
                    <span className="flex h-7 items-center px-2 text-xs text-muted-foreground">Not part of this process</span>
                  ) : stored.length ? (
                    stored.map((r, n) => chip(r, n, k++))
                  ) : (
                    <span className="flex h-7 items-center px-2 text-xs text-muted-foreground/60">{i <= current ? "One round, done" : "One round"}</span>
                  )}
                </div>
              </div>
            )
          })}
      {rounds.armed && <p className="pt-1 text-xs text-destructive">That round has notes. Turn down once more to delete it.</p>}
      <p className="pt-2 text-xs text-muted-foreground">Scroll a counter to add or remove rounds. Double-click a stage or round to rename it; the switch leaves a stage out of this process.</p>
    </div>
  )

  const inboxTasks = allTasks.filter((t) => t.stage === ANYTIME && (prefs.showDone || !t.done_at))
  const inboxCount = openTasks.filter((t) => t.stage === ANYTIME).length + looseQuestions.length
  const outline = (
    <nav aria-label="Stages and rounds">
      <ul className="flex flex-col">
        <li>
          <button type="button" className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-[13px] outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring" onClick={() => reveal("inbox")}>
            <Inbox className="size-4 shrink-0 text-muted-foreground" />
            <span className="flex-1">Inbox</span>
            {inboxCount > 0 && <span className="text-xs text-muted-foreground">{inboxCount}</span>}
          </button>
        </li>
        {STAGE_STOPS.map(([key], i) => {
          if (isOff(key)) return null
          const name = stageName(key)
          const stored = rounds.storedFor(all, key)
          const offer = i === STAGE_STOPS.length - 1
          const own = allTasks.filter((t) => t.stage === key && !t.round_id && (prefs.showDone || !t.done_at))
          const hasKids = stored.length > 0 || own.length > 0
          return (
            <li key={key} className="flex flex-col">
              <div {...dropTarget(`stage:${key}`)} className={cn("group flex h-8 items-center gap-1 rounded-md px-2 hover:bg-muted", dropAt === `stage:${key}` && "bg-muted ring-1 ring-foreground/30")}>
                <button type="button" className="flex min-w-0 flex-1 items-center gap-2 rounded-sm text-left text-[13px] outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-expanded={hasKids ? !shut[key] : undefined} onClick={() => {
                    if (!hasKids) return
                    // Folding a stage folds its rounds on the right too.
                    if (!shut[key]) setOpen((o) => ({ ...o, ...Object.fromEntries(stored.map((r) => [r.id, false])) }))
                    setShut((x) => ({ ...x, [key]: !x[key] }))
                  }}>
                  <RoundPie className="size-4 shrink-0" total={offer ? 1 : Math.max(1, stored.length)} done={stored.length ? stored.filter((r) => r.done_at).length : i <= current ? 1 : 0} />
                  <span className={cn("truncate", i === current ? "font-semibold" : i > current && "text-muted-foreground")}>{name}</span>
                </button>
                {shut[key] && hasKids && (
                  // Folded: what is under it, until hover brings the actions back.
                  (() => {
                    const q = stored.reduce((m, r) => m + (r.questions?.length ?? 0), 0)
                    const t = allTasks.filter((x) => x.stage === key && !x.done_at).length
                    const full = [stored.length && `${stored.length} ${stored.length === 1 ? "round" : "rounds"}`, q && `${q} ${q === 1 ? "question" : "questions"}`, t && `${t} open ${t === 1 ? "task" : "tasks"}`].filter(Boolean).join(", ")
                    return (
                      <span title={full} aria-label={full} className={cn("flex shrink-0 items-center gap-1.5 text-[11px] text-muted-foreground tabular-nums", writable && "group-focus-within:hidden group-hover:hidden")}>
                        {stored.length > 0 && (
                          <span className="flex items-center gap-0.5">
                            <span aria-hidden className="size-1.5 rounded-full border border-current" />
                            {stored.length}
                          </span>
                        )}
                        {q > 0 && (
                          <span className="flex items-center gap-0.5">
                            <span aria-hidden className="font-semibold">?</span>
                            {q}
                          </span>
                        )}
                        {t > 0 && (
                          <span className="flex items-center gap-0.5">
                            <span aria-hidden className="size-2 rounded-[2px] border border-current" />
                            {t}
                          </span>
                        )}
                      </span>
                    )
                  })()
                )}
                {writable && (
                  <span className={cn("shrink-0 gap-0.5 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100", shut[key] && hasKids ? "hidden group-focus-within:flex group-hover:flex" : "flex")}>
                    {i > 0 && !offer && (
                      <Button size="icon-xs" variant="ghost" aria-label={`Add a ${name} round`} title="Add a round" disabled={rounds.busy} onClick={() => { setShut((x) => ({ ...x, [key]: false })); void rounds.add(key, i) }}>
                        <AddRoundIcon className="size-3.5" />
                      </Button>
                    )}
                    <Button size="icon-xs" variant="ghost" aria-label={`Add a ${name} task`} title="Add a task" onClick={() => { setShut((x) => ({ ...x, [key]: false })); setAddingAt(`stage:${key}`) }}>
                      <ListPlus className="size-3.5" />
                    </Button>
                  </span>
                )}
              </div>
              {!shut[key] && (
                <div className="ml-[15px] flex flex-col border-l pl-3">
                  {stored.map((r, n) => {
                    const rt = allTasks.filter((t) => t.round_id === r.id && (prefs.showDone || !t.done_at))
                    return (
                      <div key={r.id} className="flex flex-col">
                        <div {...dropTarget(`round:${r.id}`)} className={cn("group flex h-7 items-center gap-1 rounded-md pr-2 hover:bg-muted", dropAt === `round:${r.id}` && "bg-muted ring-1 ring-foreground/30")}>
                          <RoundDetails r={r} name={roundName(r, n)} writable={writable} onPatch={(body) => patch(r, body)} onDelete={() => void remove(r)} onOpenNotes={() => reveal(r.id)} open={detailsFor === r.id} onOpenChange={(o) => setDetailsFor(o ? r.id : null)}>
                            <button type="button" title="Click to open on the right · double-click for details" onClick={() => reveal(r.id)} onDoubleClick={() => setDetailsFor(r.id)} className="flex min-w-0 flex-1 items-center gap-2 rounded-md pl-1 text-left text-[13px] outline-none focus-visible:ring-2 focus-visible:ring-ring">
                              <span aria-hidden className={cn("size-1.5 shrink-0 rounded-full border border-foreground", r.done_at && "bg-foreground")} />
                              <span className={cn("min-w-0 flex-1 truncate", r.in_notes === false && "text-muted-foreground")}>{roundName(r, n)}</span>
                            </button>
                          </RoundDetails>
                          {writable && (
                            <span className="flex shrink-0 gap-0.5 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100">
                              <Button size="icon-xs" variant="ghost" aria-label={`Add to ${roundName(r, n)}`} title="Add a task or question" onClick={() => setAddingAt(`round:${r.id}`)}>
                                <ListPlus className="size-3.5" />
                              </Button>
                              <Button
                                size="icon-xs"
                                variant="ghost"
                                className={cn(armedDel === r.id && "text-destructive hover:text-destructive")}
                                aria-label={armedDel === r.id ? `Confirm deleting ${roundName(r, n)}` : `Delete ${roundName(r, n)}`}
                                title={armedDel === r.id ? "It has notes. Click again to delete." : "Delete round"}
                                onClick={() => {
                                  if (r.notes?.trim() && armedDel !== r.id) return setArmedDel(r.id)
                                  setArmedDel(null)
                                  void remove(r)
                                }}
                                onMouseLeave={() => armedDel === r.id && setArmedDel(null)}
                              >
                                <X />
                              </Button>
                            </span>
                          )}
                        </div>
                        {(rt.length > 0 || !!r.questions?.length) && (
                          <div className="flex flex-col pl-6">
                            {rt.map((t, k) => taskRow(t, k, true))}
                            {(r.questions ?? []).map((q, k) => (
                              <div key={q.prompt} style={stagger(rt.length + k).style} className={cn("group -mx-2 flex min-h-7 items-center gap-1 rounded-md px-2 hover:bg-muted", stagger(rt.length + k).className)}>
                                <button type="button" className="flex min-w-0 flex-1 items-center gap-2.5 rounded-sm text-left text-[13px] outline-none focus-visible:ring-2 focus-visible:ring-ring" onClick={() => reveal(r.id)}>
                                  <span aria-hidden className={cn("flex size-4 shrink-0 items-center justify-center rounded-full border text-[10px] font-semibold", q.answer ? "border-foreground/40 text-muted-foreground" : "border-dashed border-foreground")}>?</span>
                                  <span className={cn("min-w-0 flex-1 truncate", q.answer && "text-muted-foreground")}>{q.prompt}</span>
                                </button>
                                {writable && (
                                  <Button variant="ghost" size="icon-xs" className="opacity-0 group-hover:opacity-100 focus-visible:opacity-100" aria-label={`Delete ${q.prompt}`} onClick={() => void deleteQuestion(r.id, q.prompt)}>
                                    <X />
                                  </Button>
                                )}
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    )
                  })}
                  {own.length > 0 && (
                    <div className="flex flex-col pl-2">
                      {own.map((t, k) => taskRow(t, k, true))}
                    </div>
                  )}
                </div>
              )}
            </li>
          )
        })}
      </ul>
    </nav>
  )

  // All: the inbox and each round's notes as folds beside the outline.
  const allNotes = (
    <article aria-label="Notes" className="min-w-0">
      <Fold id="inbox" title="Inbox" meta={[inboxTasks.length ? `${inboxTasks.length} tasks` : "", looseQuestions.length ? `${looseQuestions.length}?` : ""].filter(Boolean).join(" · ") || "empty"} open={isOpen("inbox")} onToggle={() => toggle("inbox")}>
        <div className="flex flex-col">
          {inboxTasks.map((t, i) => taskRow(t, i))}
          {looseQuestions.map((q, i) => (
            <QuestionRow key={q.prompt} q={q} index={inboxTasks.length + i} writable={writable} onSave={(prompt, answer) => saveQuestion("about", q.prompt, prompt, answer)} onDelete={() => void deleteQuestion("about", q.prompt)} onOpen={() => reveal("about")} />
          ))}
          {!inboxTasks.length && !looseQuestions.length && <p className="text-sm text-muted-foreground">Tasks with no stage and questions not tied to a round land here.</p>}
          {writable && (
            <div className="flex flex-col pt-1">
              <AddTaskRow where={`stage:${ANYTIME}`} places={taskPlaces} onAdd={addTask} />
              <AddQuestionRow places={places} onAdd={addQuestion} />
            </div>
          )}
        </div>
      </Fold>
      <Fold id="about" title="About this application" meta={application.notes ? "" : "empty"} open={isOpen("about")} onToggle={() => toggle("about")}>
        <NoteBody md={application.notes || ""} writable={writable} label="Application notes" placeholder={"Anything worth remembering.\n\n### A question not tied to a round?\nIts answer."} onSave={onSaveNotes} reveal={justOpened === "about"} />
      </Fold>
      {notesRounds.map(({ r, n }) => (
        <Fold
          key={r.id}
          id={r.id}
          title={roundName(r, n)}
          meta={!r.notes?.trim() ? "" : r.in_notes === false ? "not in overall" : meta(r)}
          open={isOpen(r.id)}
          onToggle={() => toggle(r.id)}
          dim={!r.notes?.trim() || r.in_notes === false}
          action={(isOpen(r.id) || !r.notes?.trim()) && started !== r.id && <NotesToggle r={r} writable={writable} onStart={() => startNotes(r.id)} onPatch={(body) => patch(r, body)} />}
        >
          {allTasks.some((t) => t.round_id === r.id && (prefs.showDone || !t.done_at)) && (
            <div className="flex flex-col pb-1">{allTasks.filter((t) => t.round_id === r.id && (prefs.showDone || !t.done_at)).map((t, i) => taskRow(t, i))}</div>
          )}
          <NoteBody md={r.notes || ""} writable={writable} label={`${roundName(r, n)} notes`} placeholder={"## Prep\n\n### A question they asked?\nYour answer.\n\n## Notes"} onSave={(notes) => patch(r, { notes })} reveal={justOpened === r.id} autoEdit={started === r.id} />
        </Fold>
      ))}
    </article>
  )

  // Notes: separate files you pick from a list. overall.md is all of them as
  // one file, shown only when picked.
  const slug = (name: string) => `${name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "") || "untitled"}.md`
  const roundSection = ({ r, n }: { r: Interview; n: number }) => ({
    id: r.id,
    title: roundName(r, n),
    md: r.notes || "",
    onSave: (notes: string) => patch(r, { notes }),
    onRename: (kind: string) => patch(r, { kind }),
    onDelete: () => remove(r),
  })
  const fileSection = (f: Note) => ({
    id: f.id,
    title: f.name,
    md: f.body,
    onSave: (body: string) => saveFile(f, { body }),
    onRename: (name: string) => saveFile(f, { name }),
    onDelete: () => deleteFile(f),
  })
  const aboutSection = { id: "about", title: "About this application", md: application.notes || "", onSave: onSaveNotes }
  // about.md and a round's file show once they have notes, or while open;
  // until then New file offers them.
  const written = (id: string, md?: string) => !!md?.trim() || noteFile === id
  const aboutWritten = written("about", application.notes)
  const writtenRounds = notesRounds.filter(({ r }) => written(r.id, r.notes))
  const unwritten = ordered.filter(({ r }) => !r.notes?.trim() && noteFile !== r.id)
  const fileList: { id: string; name: string; md: string; hint?: string; dim?: boolean; file?: Note; round?: Interview }[] = [
    { id: "overall", name: "overall.md", md: "", hint: "everything" },
    ...(aboutWritten ? [{ id: "about", name: "about.md", md: application.notes || "" }] : []),
    ...noteFiles.map((f) => ({ id: f.id, name: slug(f.name), md: f.body, file: f })),
    ...writtenRounds.map(({ r, n }) => ({ id: r.id, name: slug(roundName(r, n)), md: r.notes || "", dim: r.in_notes === false, round: r })),
  ]
  const newFile = (body: string) => {
    const taken = new Set(noteFiles.map((n) => n.name.toLowerCase()))
    let name = "Untitled"
    for (let i = 2; taken.has(name.toLowerCase()); i++) name = `Untitled ${i}`
    void createFile(name, body).then((n) => setFresh(n.id))
  }
  // A new # heading in overall.md named like a round with no notes yet is
  // that round's notes, not a new file.
  const createPart = (name: string, body: string) => {
    const hit = unwritten.find(({ r, n }) => roundName(r, n).toLowerCase() === name.trim().toLowerCase())
    if (hit) return patch(hit.r, { notes: body })
    if (!application.notes?.trim() && name.trim().toLowerCase() === aboutSection.title.toLowerCase()) return onSaveNotes(body)
    return createFile(name, body)
  }
  // X on a tile: a file goes, a round or about.md keeps its place with no
  // notes, and overall.md does all of that.
  const clearFile = async (f: (typeof fileList)[number]): Promise<unknown> => {
    if (noteFile === f.id) setNoteFile(null)
    if (f.file) return deleteFile(f.file)
    if (f.round) return f.md.trim() && patch(f.round, { notes: "" })
    if (f.id === "about") return f.md.trim() && onSaveNotes("")
    return Promise.all(fileList.filter((g) => g.id !== "overall" && (g.file || g.md.trim())).map(clearFile))
  }
  // A deleted file falls back to the grid.
  const picked = fileList.find((f) => f.id === noteFile)
  const newTile = (props: ComponentProps<"button">) => (
    <button type="button" {...props} style={stagger(fileList.length).style} className={cn(tile, "text-muted-foreground hover:text-foreground", stagger(fileList.length).className)}>
      <span className="flex size-10 items-center justify-center rounded-md border border-dashed">
        <Plus className="size-4" />
      </span>
      <span className="text-xs">New file</span>
      <span className="h-3.5" />
    </button>
  )
  // What the open file holds: overall.md is every part, the rest one each.
  const single = picked?.id !== "overall"
  const openSections: NoteSection[] = !picked
    ? []
    : !single
      ? [...(application.notes?.trim() ? [aboutSection] : []), ...noteFiles.map(fileSection), ...notesRounds.filter(({ r }) => r.in_notes !== false && r.notes?.trim()).map(roundSection)]
      : picked.file
        ? [fileSection(picked.file)]
        : picked.round
          ? [roundSection({ r: picked.round, n: ordered.find((o) => o.r.id === picked.round!.id)?.n ?? 0 })]
          : [aboutSection]
  const tile = "flex w-full flex-col items-center gap-1.5 rounded-lg px-2 pt-3 pb-2 text-center outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
  const notesView = !picked ? (
    // Like Finder's icon view: a file per tile, click one to open it.
    <nav aria-label="Notes files" className="grid grid-cols-[repeat(auto-fill,minmax(7rem,1fr))] gap-2">
      {fileList.map((f, i) => (
        <div key={f.id} style={stagger(i).style} className={cn("group relative", stagger(i).className)}>
          <button type="button" onClick={() => setNoteFile(f.id)} className={cn(tile, f.dim && "opacity-50")}>
            <FileText strokeWidth={1.25} className={cn("size-10", f.id === "overall" ? "text-foreground" : "text-muted-foreground")} />
            <span className={cn("line-clamp-2 w-full font-mono text-xs break-all", f.id === "overall" && "italic")} title={f.name}>
              {f.name}
            </span>
            <span className="h-3.5 w-full truncate text-[11px] leading-none text-muted-foreground">{f.hint ?? (f.round ? meta(f.round) : f.dim ? "hidden" : "")}</span>
          </button>
          {writable && (
            <Button
              size="icon-xs"
              variant="ghost"
              className={cn("absolute top-1 right-1 opacity-0 group-hover:opacity-100 focus-visible:opacity-100", armedDel === f.id && "text-destructive opacity-100 hover:text-destructive")}
              aria-label={armedDel === f.id ? `Confirm clearing ${f.name}` : `Delete ${f.name}`}
              title={armedDel === f.id ? "Click again: deletes every file and clears every note" : "Delete"}
              onMouseLeave={() => armedDel === f.id && setArmedDel(null)}
              onClick={() => {
                // overall.md is everything at once, so it asks first.
                if (f.id === "overall" && armedDel !== f.id) return setArmedDel(f.id)
                setArmedDel(null)
                void clearFile(f)
              }}
            >
              <X />
            </Button>
          )}
        </div>
      ))}
      {writable && (
        // Like a word processor's new document: a template, or notes for a
        // round that has none yet.
        <Popover>
          <PopoverTrigger asChild>{newTile({})}</PopoverTrigger>
          <PopoverContent align="start" side="bottom" className="flex w-80 flex-col gap-3">
            <div className="grid grid-cols-3 gap-2">
              {TEMPLATES.map((t) => (
                <PopoverPrimitive.Close asChild key={t.name}>
                  <button type="button" onClick={() => newFile(t.body)} className="flex flex-col items-center gap-1.5 rounded-md p-2 text-xs outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring">
                    <span className="flex h-14 w-11 flex-col gap-1 overflow-hidden rounded-sm border bg-background p-1.5 text-left font-mono text-[5px] leading-tight text-muted-foreground">
                      {t.body ? t.body.split("\n").filter(Boolean).slice(0, 5).map((l, k) => <span key={k} className="truncate">{l}</span>) : <Plus className="m-auto size-3" />}
                    </span>
                    {t.name}
                  </button>
                </PopoverPrimitive.Close>
              ))}
            </div>
            {(!aboutWritten || unwritten.length > 0) && (
              <div className="flex flex-col">
                <span className="px-2 pb-1 text-[11px] text-muted-foreground">Notes for</span>
                {[...(!aboutWritten ? [{ id: "about", name: "about.md" }] : []), ...unwritten.map(({ r, n }) => ({ id: r.id, name: slug(roundName(r, n)) }))].map((o) => (
                  <PopoverPrimitive.Close asChild key={o.id}>
                    <button type="button" onClick={() => setNoteFile(o.id)} className="flex h-7 items-center gap-2 rounded-md px-2 text-left font-mono text-xs outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring">
                      <FileText className="size-3.5 text-muted-foreground" /> {o.name}
                    </button>
                  </PopoverPrimitive.Close>
                ))}
              </div>
            )}
          </PopoverContent>
        </Popover>
      )}
    </nav>
  ) : (
    <div key={picked.id} className={cn("flex min-w-0 flex-col gap-2", stagger().className)}>
      <div className="flex items-center gap-2 font-mono text-xs text-muted-foreground">
        <Button size="xs" variant="ghost" className="-ml-2 font-sans" onClick={() => setNoteFile(null)}>
          <ChevronLeft /> Files
        </Button>
        {picked.file && writable ? (
          // A file is named here; Enter or clicking out renames it.
          <span className="flex items-center text-foreground">
            <input
              key={picked.file.id + picked.file.version}
              aria-label="File name"
              autoFocus={fresh === picked.file.id}
              onFocus={(e) => {
                if (fresh !== picked.file!.id) return
                e.currentTarget.select()
                setFresh(null)
              }}
              defaultValue={picked.file.name}
              size={Math.max(picked.file.name.length, 4)}
              className="rounded-sm bg-transparent outline-none hover:bg-muted focus:bg-muted"
              onKeyDown={(e) => {
                if (e.key === "Enter") e.currentTarget.blur()
                if (e.key === "Escape") {
                  e.currentTarget.value = picked.file!.name
                  e.currentTarget.blur()
                }
              }}
              onBlur={(e) => {
                const name = e.currentTarget.value.trim()
                if (name && name !== picked.file!.name) void saveFile(picked.file!, { name })
                else e.currentTarget.value = picked.file!.name
              }}
            />
            <span className="text-muted-foreground">.md</span>
          </span>
        ) : (
          <span className="text-foreground">{picked.name}</span>
        )}
        {picked.round && <span className="font-sans">· {meta(picked.round)}</span>}
        <span className="flex-1" />
        {picked.round?.notes?.trim() && (
          <span className="font-sans">
            <NotesToggle r={picked.round} writable={writable} onStart={() => {}} onPatch={(body) => patch(picked.round!, body)} />
          </span>
        )}
        {picked.id === "overall" && (
          <button type="button" className="flex items-center gap-1 font-sans hover:text-foreground" onClick={() => setCompiled(true)}>
            <FileText className="size-3.5" /> Raw
          </button>
        )}
        <button
          type="button"
          className="flex items-center gap-1 font-sans hover:text-foreground"
          onClick={() => {
            void navigator.clipboard.writeText(single ? openSections[0].md.trim() : compose(openSections))
            setCopied(picked.id)
            setTimeout(() => setCopied(null), 1500)
          }}
        >
          {copied === picked.id ? <Check className="size-3.5" /> : <Copy className="size-3.5" />} {copied === picked.id ? "Copied" : "Copy all"}
        </button>
      </div>
      <NotesEditor key={picked.id} single={single} writable={writable} sections={openSections} onCreate={single ? undefined : createPart} />
    </div>
  )

  return (
    <SectionCard id="outline" title="Rounds, tasks and notes" summary={waiting ? `${waiting} waiting on you` : rounds.summary()}>
      <div className="flex items-center gap-2">
        <SegmentedTabs
          label="Show"
          value={prefs.view}
          onChange={(view) => setPrefs({ view })}
          options={[
            { value: "all", label: "All" },
            { value: "rounds", label: "Rounds", count: ordered.length },
            { value: "tasks", label: "Tasks", count: waiting },
            { value: "questions", label: "Questions", count: questionCount },
            { value: "notes", label: "Notes" },
          ]}
        />
        <span className="flex-1" />
        <ViewOptions prefs={prefs} onChange={setPrefs} />
      </div>

      {/* Holds its height while a shorter view would move the page; see hold. */}
      <div ref={bodyRef} style={{ minHeight: hold || undefined }} className="relative">
        {addingAt && (
          // Adding from the outline: the card blurs and the task row floats on top.
          <div className="absolute inset-0 z-20 flex items-start justify-center rounded-lg bg-background/60 pt-24 backdrop-blur-sm animate-in fade-in duration-150" onMouseDown={(e) => e.target === e.currentTarget && setAddingAt(null)}>
            <div className="w-full max-w-xl rounded-xl border bg-background p-3 shadow-lg animate-in fade-in zoom-in-95 duration-150">
              <AddTaskRow where={addingAt} places={taskPlaces} onAdd={addTask} onAddQuestion={(prompt, where) => addQuestion(questionPlace(where), prompt, "")} startOpen closeOnAdd onClose={() => setAddingAt(null)} />
              <p className="px-2 pt-1 text-xs text-muted-foreground">Enter adds · Esc cancels · the box at the start switches between a task and a question</p>
            </div>
          </div>
        )}
        <div ref={innerRef} className="flex flex-col gap-3">
        {prefs.view === "all" && (
          <>
            <div className="flex items-center gap-3 rounded-lg border px-3.5 py-2 text-sm">
              <span className="text-[11px] font-semibold tracking-wider">NEXT</span>
              <button type="button" className="min-w-0 flex-1 truncate rounded-sm text-left outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring" onClick={jumpToNext} disabled={!next}>
                {next ? next.title : "Nothing waiting on you"}
              </button>
              {next && <span className="shrink-0 text-xs text-muted-foreground">{next.where}</span>}
              {waiting > 1 && (
                <button type="button" className="flex shrink-0 items-center gap-0.5 text-xs text-muted-foreground hover:text-foreground" onClick={() => setPrefs({ view: "tasks" })}>
                  {waiting - 1} more <ChevronRight className="size-3.5" />
                </button>
              )}
            </div>
            <div className="grid gap-8 pt-2 md:grid-cols-[230px_minmax(0,1fr)]">
              {outline}
              {allNotes}
            </div>
          </>
        )}
        {prefs.view === "rounds" && roundsView}
        {prefs.view === "tasks" && tasksView}
        {prefs.view === "questions" && questionsView}
        {prefs.view === "notes" && notesView}
        </div>
      </div>
      {rounds.error && <p className="text-xs text-destructive">{rounds.error}</p>}
      <CompiledNotes appId={appId} open={compiled} onOpenChange={setCompiled} />
    </SectionCard>
  )
}
