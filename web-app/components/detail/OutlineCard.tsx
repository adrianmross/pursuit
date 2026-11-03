"use client"

import { useEffect, useState, type ReactNode } from "react"
import { ChevronDown, FileText, SlidersHorizontal, Trash2, X } from "lucide-react"
import { api } from "@/lib/api-client"
import { cn } from "@/lib/utils"
import { label, STAGE_STOPS, type Application, type Interview, type Page, type Question, type Task } from "@/lib/types"
import { RoundPie, useRounds, useTasks } from "@/components/Rounds"
import { Unanswered } from "@/components/PreparedAnswers"
import { InlineText } from "@/components/InlineEdit"
import { SectionCard } from "@/components/detail/SectionCard"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { Input } from "@/components/ui/input"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"

const ANYTIME = "anytime"

// What the card shows. Layout (outline now; board and timeline to come) is a
// separate choice in the view options, so the tabs stay about content.
type View = "all" | "rounds" | "tasks" | "questions" | "notes"
const VIEWS: [View, string][] = [
  ["all", "All"],
  ["rounds", "Rounds"],
  ["tasks", "Tasks"],
  ["questions", "Questions"],
  ["notes", "Notes"],
]
type Prefs = { view: View; showDone: boolean; hideLeftOut: boolean; sort: "stage" | "date"; layout: "outline" }
const DEFAULT_PREFS: Prefs = { view: "all", showDone: false, hideLeftOut: false, sort: "stage", layout: "outline" }
const PREFS_KEY = "pursuit.outline"

// Headings that end in ? and the text under them, as the server reads them.
function parseQuestions(md: string) {
  const out: { prompt: string; answer: string }[] = []
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
          Hide notes left out
          <Switch checked={prefs.hideLeftOut} onCheckedChange={(v) => onChange({ hideLeftOut: v })} />
        </label>
        <div className={row}>
          Sort
          <ToggleGroup type="single" variant="outline" size="sm" value={prefs.sort} onValueChange={(v) => v && onChange({ sort: v as Prefs["sort"] })}>
            <ToggleGroupItem value="stage" className="px-2.5 text-xs">Stage</ToggleGroupItem>
            <ToggleGroupItem value="date" className="px-2.5 text-xs">Date</ToggleGroupItem>
          </ToggleGroup>
        </div>
        <div className="flex flex-col gap-1.5 border-t pt-3">
          <span className="text-xs text-muted-foreground">Layout</span>
          <ToggleGroup type="single" variant="outline" size="sm" value={prefs.layout} onValueChange={(v) => v && onChange({ layout: v as Prefs["layout"] })}>
            <ToggleGroupItem value="outline" className="px-2.5 text-xs">Outline</ToggleGroupItem>
            <ToggleGroupItem value="board" disabled className="px-2.5 text-xs">Board</ToggleGroupItem>
            <ToggleGroupItem value="timeline" disabled className="px-2.5 text-xs">Timeline</ToggleGroupItem>
          </ToggleGroup>
          <span className="text-xs text-muted-foreground">Board and Timeline are on the way.</span>
        </div>
      </PopoverContent>
    </Popover>
  )
}
const day = (value?: string) => (value ? value.slice(0, 10) : "")
const roundName = (r: Interview, n: number) => r.kind || `Round ${n + 1}`
const stageName = (key: string) => (key === ANYTIME ? "Anytime" : STAGE_STOPS.find(([k]) => k === key)?.[1] ?? label(key))

// A markdown document edited in place: Save and Discard appear once it changes.
function MarkdownEditor({ value, placeholder, disabled, label: name, onSave }: { value: string; placeholder: string; disabled: boolean; label: string; onSave: (next: string) => Promise<unknown> }) {
  const [draft, setDraft] = useState(value)
  const [saving, setSaving] = useState(false)
  useEffect(() => {
    // Takes saves and agent edits that land while this is unchanged.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDraft(value)
  }, [value])
  const dirty = draft !== value
  return (
    <div className="flex flex-col gap-2">
      <Textarea
        aria-label={name}
        className="min-h-20 resize-y border-0 border-l bg-transparent px-3 py-0 font-mono text-[13px] leading-relaxed shadow-none focus-visible:ring-0 dark:bg-transparent"
        rows={Math.max(3, draft.split("\n").length + 1)}
        placeholder={placeholder}
        value={draft}
        disabled={disabled}
        onChange={(e) => setDraft(e.target.value)}
      />
      {dirty && !disabled && (
        <div className="flex gap-2 pl-3">
          <Button
            size="xs"
            disabled={saving}
            onClick={async () => {
              setSaving(true)
              try {
                await onSave(draft)
              } finally {
                setSaving(false)
              }
            }}
          >
            Save
          </Button>
          <Button size="xs" variant="ghost" onClick={() => setDraft(value)}>
            Discard
          </Button>
        </div>
      )}
    </div>
  )
}

// One line of the notes: ▸ Title ─── meta. It opens to its markdown and
// nothing else. The checkbox keeps a round in the notes or leaves it out.
function Fold({ id, title, meta, open, onToggle, included, onInclude, writable, children }: { id: string; title: string; meta?: string; open: boolean; onToggle: () => void; included?: boolean; onInclude?: (next: boolean) => void; writable: boolean; children: ReactNode }) {
  const dim = included === false
  return (
    <div id={`note-${id}`} className="scroll-mt-24">
      <div className="group flex items-center gap-2">
        <button type="button" className="flex min-w-0 flex-1 items-center gap-2.5 rounded-sm py-1.5 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring" onClick={onToggle} aria-expanded={open}>
          <span aria-hidden className="w-3 shrink-0 font-mono text-xs text-muted-foreground">
            {open ? "▾" : "▸"}
          </span>
          <span className={cn("truncate text-sm font-medium", dim && "font-normal text-muted-foreground")}>{title}</span>
          <span aria-hidden className="h-px min-w-4 flex-1 bg-border" />
          <span className="shrink-0 text-xs text-muted-foreground">{meta}</span>
        </button>
        {onInclude && open && (
          <Checkbox
            aria-label={`${title} in notes`}
            title={included ? "In notes. Untick to leave it out." : "Left out of notes"}
            checked={included}
            disabled={!writable}
            onCheckedChange={(v) => onInclude(v === true)}
          />
        )}
      </div>
      {open && <div className="pt-1 pb-3 pl-5">{children}</div>}
    </div>
  )
}

// A round's details, one click away in the outline instead of on the page.
function RoundDetails({ r, name, writable, onPatch, onDelete, onOpenNotes, children }: { r: Interview; name: string; writable: boolean; onPatch: (body: Record<string, unknown>) => Promise<unknown>; onDelete: () => void; onOpenNotes: () => void; children: ReactNode }) {
  return (
    <Popover>
      <PopoverTrigger asChild>{children}</PopoverTrigger>
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
        <label className="flex items-center gap-2">
          <Checkbox checked={r.in_notes !== false} disabled={!writable} onCheckedChange={(v) => void onPatch({ in_notes: v === true })} />
          In notes
        </label>
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

// Everything waiting on you, behind the NEXT line: open tasks grouped Anytime
// first, form questions to answer, and one place to add a task.
function Todo({ tasks, form, stageFor, focusId, showDone, onShowDone, writable, onToggle, onRemove, onAdd }: { tasks: Task[]; form: Question[]; stageFor: string; focusId: string | null; showDone: boolean; onShowDone: (v: boolean) => void; writable: boolean; onToggle: (t: Task) => void; onRemove: (t: Task) => void; onAdd: (title: string, stage: string) => Promise<unknown> }) {
  const [title, setTitle] = useState("")
  const [stage, setStage] = useState(stageFor)
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setStage(stageFor), [stageFor])
  // NEXT brings you to its task: scrolled into view, highlighted, focused.
  useEffect(() => {
    if (!focusId) return
    const id = requestAnimationFrame(() => {
      const row = document.getElementById(`task-${focusId}`)
      row?.scrollIntoView({ block: "center", behavior: "smooth" })
      row?.querySelector<HTMLElement>("button, input")?.focus({ preventScroll: true })
    })
    return () => cancelAnimationFrame(id)
  }, [focusId])
  const groups = [ANYTIME, ...STAGE_STOPS.map(([k]) => k)]
    .map((key) => ({ key, items: tasks.filter((t) => t.stage === key && (showDone || !t.done_at)), answers: key === "to_apply" ? form : [] }))
    .filter((g) => g.items.length || g.answers.length)
  const done = tasks.filter((t) => t.done_at).length
  return (
    <div className="flex flex-col gap-3 pt-3">
      {groups.map((g) => (
        <div key={g.key} className="flex flex-col">
          <div className="pb-1 text-xs text-muted-foreground">{stageName(g.key)}</div>
          {g.answers.map((q) => (
            <Collapsible key={q.prompt}>
              <CollapsibleTrigger className="flex min-h-8 w-full items-center gap-2.5 rounded-md text-left text-sm hover:bg-muted/60">
                <span className="size-4 shrink-0 rounded-[5px] border border-dashed border-muted-foreground" aria-hidden />
                Answer: {q.prompt}
              </CollapsibleTrigger>
              <CollapsibleContent className="py-2 pl-6">{q.prompt && <Unanswered prompt={q.prompt} />}</CollapsibleContent>
            </Collapsible>
          ))}
          {g.items.map((t) => (
            <div key={t.id} id={`task-${t.id}`} className={cn("group flex min-h-8 scroll-mt-24 items-center gap-2.5 rounded-md px-1 -mx-1 hover:bg-muted/60", t.id === focusId && "bg-muted ring-1 ring-foreground")}>
              <Checkbox aria-label={`Done: ${t.title}`} checked={!!t.done_at} disabled={!writable} onCheckedChange={() => onToggle(t)} />
              <span className={cn("min-w-0 flex-1 text-sm break-words", t.done_at && "text-muted-foreground line-through")}>{t.title}</span>
              {writable && (
                <Button variant="ghost" size="icon-xs" className="opacity-0 group-hover:opacity-100 focus-visible:opacity-100" aria-label={`Delete ${t.title}`} onClick={() => onRemove(t)}>
                  <X />
                </Button>
              )}
            </div>
          ))}
        </div>
      ))}
      {!groups.length && <p className="text-sm text-muted-foreground">Nothing waiting on you.</p>}
      {writable && (
        <form
          className="flex gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            if (title.trim()) void onAdd(title.trim(), stage).then(() => setTitle(""))
          }}
        >
          <Input className="h-8 flex-1 text-sm" placeholder="Add a task" aria-label="New task" value={title} onChange={(e) => setTitle(e.target.value)} />
          <Select value={stage} onValueChange={setStage}>
            <SelectTrigger size="sm" className="w-36" aria-label="Task stage">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {[ANYTIME, ...STAGE_STOPS.map(([k]) => k)].map((k) => (
                <SelectItem key={k} value={k}>
                  {stageName(k)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </form>
      )}
      {done > 0 && (
        <button type="button" className="self-start text-xs text-muted-foreground hover:text-foreground" onClick={() => onShowDone(!showDone)}>
          {showDone ? "Hide done" : `${done} done`}
        </button>
      )}
    </div>
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
          <SheetTitle>All notes</SheetTitle>
          <SheetDescription>The application&apos;s notes and every round in notes, as one document. The same text as pursuit apps notes.</SheetDescription>
        </SheetHeader>
        <pre className="min-h-0 flex-1 overflow-auto px-4 pb-6 font-mono text-[13px] leading-relaxed whitespace-pre-wrap">{md ?? "Loading…"}</pre>
      </SheetContent>
    </Sheet>
  )
}

// Rounds, tasks and notes, one concern at a time: the NEXT line opens what is
// waiting on you, the outline is where rounds live, and the notes are prose.
export function OutlineCard({ application, writable, onChange, onSaveNotes }: { application: Application; writable: boolean; onChange: () => Promise<void>; onSaveNotes: (notes: string) => Promise<unknown> }) {
  const appId = application.id
  const current = application.stage ?? 0
  const currentKey = STAGE_STOPS[current][0]
  const rounds = useRounds(appId, current, onChange)
  const tasks = useTasks(appId, current, onChange)
  const [form, setForm] = useState<Question[]>([])
  const [shut, setShut] = useState<Record<string, boolean>>({})
  const [open, setOpen] = useState<Record<string, boolean>>({})
  const [todo, setTodo] = useState(false)
  const [taskStage, setTaskStage] = useState(ANYTIME)
  const [compiled, setCompiled] = useState(false)
  const [prefs, setPrefsState] = useState<Prefs>(DEFAULT_PREFS)
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

  useEffect(() => {
    rounds.fetchRounds().catch(() => {})
    tasks.fetchTasks().catch(() => {})
    api<Page<Question>>(`/v1/applications/${encodeURIComponent(appId)}/questions`)
      .then((page) => setForm(page.data || []))
      .catch(() => {})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [appId])

  const toAnswer = form.filter((q) => q.required && !q.answer)
  const allTasks = tasks.list ?? []
  const openTasks = allTasks.filter((t) => !t.done_at)
  const nextTask = openTasks.find((t) => t.stage === currentKey) ?? openTasks.find((t) => t.stage === ANYTIME) ?? openTasks[0]
  const next = nextTask ? { title: nextTask.title, where: stageName(nextTask.stage) } : toAnswer[0] ? { title: `Answer: ${toAnswer[0].prompt}`, where: "Apply" } : null
  const waiting = openTasks.length + toAnswer.length

  const patch = (r: Interview, body: Record<string, unknown>) =>
    rounds.run(() => api(`/v1/rounds/${encodeURIComponent(r.id)}`, { method: "PATCH", body: { version: r.version, ...body } }))
  const remove = (r: Interview) => rounds.run(() => api(`/v1/rounds/${encodeURIComponent(r.id)}?version=${r.version}`, { method: "DELETE" }))
  const toggleFold = (id: string) => setOpen((o) => ({ ...o, [id]: !o[id] }))
  const reveal = (id: string) => {
    setOpen((o) => ({ ...o, [id]: true }))
    requestAnimationFrame(() => document.getElementById(`note-${id}`)?.scrollIntoView({ block: "nearest", behavior: "smooth" }))
  }

  const all = rounds.list ?? []
  const ordered = STAGE_STOPS.flatMap(([key]) => rounds.storedFor(all, key).map((r, n) => ({ r, n })))
  const included = ordered.filter(({ r }) => r.in_notes !== false)
  const short = (v?: string) => (v ? new Date(`${day(v)}T12:00:00`).toLocaleDateString(undefined, { month: "short", day: "numeric" }) : "")
  const meta = (r: Interview) => [stageName(r.stage), short(r.scheduled_at), r.questions?.length ? `${r.questions.length}?` : ""].filter(Boolean).join(" · ")

  const setPrefs = (next: Partial<Prefs>) => {
    const merged = { ...prefs, ...next }
    setPrefsState(merged)
    try {
      localStorage.setItem(PREFS_KEY, JSON.stringify(merged))
    } catch {
      // Private windows: the choice lasts until reload.
    }
  }
  const go = (view: View, noteId?: string) => {
    setPrefs({ view })
    if (noteId) reveal(noteId)
  }

  const byDate = (list: { r: Interview; n: number }[]) => [...list].sort((a, b) => (a.r.scheduled_at || "9999").localeCompare(b.r.scheduled_at || "9999"))
  const notesList = prefs.hideLeftOut ? included : ordered
  const shownNotes = prefs.sort === "date" ? byDate(notesList) : notesList
  const roundRows = prefs.sort === "date" ? byDate(ordered) : ordered

  const stageRow = (key: string, name: string, i: number, stored: Interview[]) => (
    <div className="group relative flex h-8 items-center rounded-md px-2 hover:bg-muted">
      <button
        type="button"
        className="flex min-w-0 flex-1 items-center gap-2 rounded-sm text-left text-[13px] outline-none focus-visible:ring-2 focus-visible:ring-ring"
        aria-expanded={stored.length ? !shut[key] : undefined}
        onClick={() => stored.length && setShut((x) => ({ ...x, [key]: !x[key] }))}
      >
        {i < STAGE_STOPS.length - 1 ? (
          <RoundPie className="size-4 shrink-0" total={Math.max(1, stored.length)} done={stored.length ? stored.filter((r) => r.done_at).length : i <= current ? 1 : 0} />
        ) : (
          <RoundPie className="size-4 shrink-0" total={1} done={i <= current ? 1 : 0} />
        )}
        <span className={cn("truncate", i === current ? "font-semibold" : i > current && "text-muted-foreground")}>{name}</span>
      </button>
      {writable && (
        <span className="absolute right-1 flex gap-1 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100">
          {i > 0 && i < STAGE_STOPS.length - 1 && (
            <Button size="xs" variant="outline" className="h-5 rounded-full px-1.5 text-[11px]" disabled={rounds.busy} onClick={() => { setShut((x) => ({ ...x, [key]: false })); void rounds.add(key, i) }}>
              + round
            </Button>
          )}
          <Button size="xs" variant="outline" className="h-5 rounded-full px-1.5 text-[11px]" onClick={() => { setTaskStage(key); if (prefs.view === "all") setTodo(true); else setPrefs({ view: "tasks" }) }}>
            + task
          </Button>
        </span>
      )}
    </div>
  )

  const roundButton = (r: Interview, n: number, indent: boolean, extra?: string) => (
    <RoundDetails key={r.id} r={r} name={roundName(r, n)} writable={writable} onPatch={(body) => patch(r, body)} onDelete={() => void remove(r)} onOpenNotes={() => go(prefs.view === "all" ? "all" : "notes", r.id)}>
      <button type="button" className={cn("flex h-8 items-center gap-2 rounded-md pr-2 text-left text-[13px] outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring", indent ? "pl-7" : "pl-2")}>
        <RoundPie className="size-3.5 shrink-0" total={1} done={r.done_at ? 1 : 0} />
        <span className={cn("min-w-0 flex-1 truncate", r.in_notes === false && "text-muted-foreground")}>{roundName(r, n)}</span>
        {extra && <span className="shrink-0 text-xs text-muted-foreground">{extra}</span>}
      </button>
    </RoundDetails>
  )

  const outline = (
    <nav aria-label="Stages and rounds">
      <ul className="flex flex-col">
        {STAGE_STOPS.map(([key, name], i) => {
          const stored = rounds.storedFor(all, key)
          return (
            <li key={key} className="flex flex-col">
              {stageRow(key, name, i, stored)}
              {!shut[key] && stored.map((r, n) => roundButton(r, n, true))}
            </li>
          )
        })}
      </ul>
    </nav>
  )

  const notes = (
    <article aria-label="Notes" className="min-w-0">
      <Fold id="about" title="About this application" meta={application.notes ? "" : "empty"} open={!!open.about} onToggle={() => toggleFold("about")} writable={writable}>
        <MarkdownEditor value={application.notes || ""} label="Application notes" placeholder={"Anything worth remembering about this one.\n\n### A question not tied to a round?\nIts answer."} disabled={!writable} onSave={onSaveNotes} />
      </Fold>
      {shownNotes.map(({ r, n }) => (
        <Fold key={r.id} id={r.id} title={roundName(r, n)} meta={r.in_notes === false ? "left out" : meta(r)} open={!!open[r.id]} onToggle={() => toggleFold(r.id)} included={r.in_notes !== false} onInclude={(v) => void patch(r, { in_notes: v })} writable={writable}>
          <MarkdownEditor value={r.notes || ""} label={`${roundName(r, n)} notes`} placeholder={"## Prep\n\n### A question they asked?\nYour answer.\n\n## Notes"} disabled={!writable} onSave={(notes) => patch(r, { notes })} />
        </Fold>
      ))}
      <button type="button" className="mt-3 flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground" onClick={() => setCompiled(true)}>
        <FileText className="size-3.5" /> All notes as one document
      </button>
    </article>
  )

  const todoList = (focus: boolean) => (
    <Todo
      tasks={allTasks}
      form={toAnswer}
      stageFor={taskStage}
      focusId={focus ? (nextTask?.id ?? null) : null}
      showDone={prefs.showDone}
      onShowDone={(v) => setPrefs({ showDone: v })}
      writable={writable}
      onToggle={(t) => void tasks.toggle(t)}
      onRemove={(t) => void tasks.remove(t)}
      onAdd={(title, stage) => tasks.add(title, stage)}
    />
  )

  // Every question in one list: those in the application's notes (not tied
  // to a round), each round's, then the form's. A click opens where it lives.
  const appQuestions = parseQuestions(application.notes || "")
  const questionGroups = [
    { id: "about", title: "Not tied to a round", items: appQuestions },
    ...ordered.filter(({ r }) => r.questions?.length).map(({ r, n }) => ({ id: r.id, title: `${roundName(r, n)} · ${stageName(r.stage)}`, items: r.questions ?? [] })),
  ]
  const questions = (
    <div className="flex flex-col gap-5">
      {questionGroups.map((g) =>
        g.items.length ? (
          <section key={g.id} className="flex flex-col gap-1">
            <h3 className="text-xs text-muted-foreground">{g.title}</h3>
            {g.items.map((q) => (
              <button key={q.prompt} type="button" className="flex flex-col rounded-md px-2 py-1.5 text-left -mx-2 hover:bg-muted" onClick={() => go("notes", g.id)}>
                <span className="text-sm font-medium">{q.prompt}</span>
                <span className={cn("line-clamp-2 text-sm text-muted-foreground", !q.answer && "italic")}>{q.answer || "No answer yet"}</span>
              </button>
            ))}
          </section>
        ) : null,
      )}
      {form.length > 0 && (
        <section className="flex flex-col gap-1">
          <h3 className="text-xs text-muted-foreground">Application form</h3>
          {form.map((q) => (
            <div key={q.prompt} className="flex flex-col gap-1 py-1.5">
              <span className="text-sm font-medium">{q.prompt}</span>
              {q.answer ? <span className="line-clamp-2 text-sm text-muted-foreground">{q.answer}</span> : q.prompt && writable ? <Unanswered prompt={q.prompt} /> : null}
            </div>
          ))}
        </section>
      )}
      {!appQuestions.length && !form.length && questionGroups.length === 1 && (
        <p className="text-sm text-muted-foreground">No questions yet. In any note, a heading that ends in ? is a question.</p>
      )}
    </div>
  )

  const roundsView = (
    <div className="flex flex-col">
      {prefs.sort === "date"
        ? roundRows.map(({ r, n }) => roundButton(r, n, false, [stageName(r.stage), short(r.scheduled_at)].filter(Boolean).join(" · ")))
        : STAGE_STOPS.slice(1, -1).map(([key, name]) => {
            const i = STAGE_STOPS.findIndex(([k]) => k === key)
            const stored = rounds.storedFor(all, key)
            return (
              <div key={key} className="flex flex-col">
                {stageRow(key, name, i, rounds.storedFor(all, key))}
                {!shut[key] && stored.map((r) => roundButton(r, rounds.storedFor(all, key).indexOf(r), true, [short(r.scheduled_at), r.interviewer, r.questions?.length ? `${r.questions.length}?` : ""].filter(Boolean).join(" · ")))}
              </div>
            )
          })}
    </div>
  )

  return (
    <SectionCard id="outline" title="Rounds, tasks and notes" summary={waiting ? `${waiting} waiting on you` : rounds.summary()}>
      <div className="flex items-center gap-2">
        <ToggleGroup type="single" variant="outline" size="sm" value={prefs.view} onValueChange={(v) => v && setPrefs({ view: v as View })} aria-label="Show">
          {VIEWS.map(([v, name]) => (
            <ToggleGroupItem key={v} value={v} className="px-3 text-xs">
              {name}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
        <span className="flex-1" />
        <ViewOptions prefs={prefs} onChange={setPrefs} />
      </div>

      {prefs.view === "all" && (
        <>
          <Collapsible open={todo} onOpenChange={setTodo} className="rounded-lg border px-3.5 py-2">
            <CollapsibleTrigger className="flex w-full items-center gap-3 text-left text-sm outline-none">
              <span className="text-[11px] font-semibold tracking-wider">NEXT</span>
              <span className="min-w-0 flex-1 truncate">{next ? next.title : "Nothing waiting on you"}</span>
              {next && <span className="shrink-0 text-xs text-muted-foreground">{next.where}</span>}
              <span className="flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
                {waiting > 1 ? `${waiting - 1} more` : ""}
                <ChevronDown className={cn("size-4 transition-transform", !todo && "-rotate-90")} />
              </span>
            </CollapsibleTrigger>
            <CollapsibleContent>{todoList(todo)}</CollapsibleContent>
          </Collapsible>
          <div className="grid gap-8 pt-2 md:grid-cols-[180px_minmax(0,1fr)]">
            {outline}
            {notes}
          </div>
        </>
      )}
      {prefs.view === "rounds" && roundsView}
      {prefs.view === "tasks" && todoList(true)}
      {prefs.view === "questions" && questions}
      {prefs.view === "notes" && notes}
      {rounds.error && <p className="text-xs text-destructive">{rounds.error}</p>}
      <CompiledNotes appId={appId} open={compiled} onOpenChange={setCompiled} />
    </SectionCard>
  )
}
