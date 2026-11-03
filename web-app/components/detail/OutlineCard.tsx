"use client"

import { useEffect, useState, type ReactNode } from "react"
import { ChevronDown, FileText, Trash2, X } from "lucide-react"
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
import { Textarea } from "@/components/ui/textarea"

const ANYTIME = "anytime"
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
        {onInclude && (
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
function Todo({ tasks, form, stageFor, writable, onToggle, onRemove, onAdd }: { tasks: Task[]; form: Question[]; stageFor: string; writable: boolean; onToggle: (t: Task) => void; onRemove: (t: Task) => void; onAdd: (title: string, stage: string) => Promise<unknown> }) {
  const [title, setTitle] = useState("")
  const [stage, setStage] = useState(stageFor)
  const [showDone, setShowDone] = useState(false)
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setStage(stageFor), [stageFor])
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
            <div key={t.id} className="group flex min-h-8 items-center gap-2.5 rounded-md hover:bg-muted/60">
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
        <button type="button" className="self-start text-xs text-muted-foreground hover:text-foreground" onClick={() => setShowDone((v) => !v)}>
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
  const [showLeftOut, setShowLeftOut] = useState(false)
  const [compiled, setCompiled] = useState(false)

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
  const leftOut = ordered.filter(({ r }) => r.in_notes === false)
  const short = (v?: string) => (v ? new Date(`${day(v)}T12:00:00`).toLocaleDateString(undefined, { month: "short", day: "numeric" }) : "")
  const meta = (r: Interview) => [stageName(r.stage), short(r.scheduled_at), r.questions?.length ? `${r.questions.length}?` : ""].filter(Boolean).join(" · ")

  return (
    <SectionCard id="outline" title="Rounds, tasks and notes" summary={waiting ? `${waiting} waiting on you` : rounds.summary()}>
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
        <CollapsibleContent>
          <Todo
            tasks={allTasks}
            form={toAnswer}
            stageFor={taskStage}
            writable={writable}
            onToggle={(t) => void tasks.toggle(t)}
            onRemove={(t) => void tasks.remove(t)}
            onAdd={(title, stage) => tasks.add(title, stage)}
          />
        </CollapsibleContent>
      </Collapsible>

      <div className="grid gap-8 pt-2 md:grid-cols-[180px_minmax(0,1fr)]">
        <nav aria-label="Stages and rounds">
          <ul className="flex flex-col">
            {STAGE_STOPS.map(([key, name], i) => {
              const stored = rounds.storedFor(all, key)
              return (
                <li key={key} className="flex flex-col">
                  <div className="group relative flex h-8 items-center rounded-md px-2 hover:bg-muted">
                    <button
                      type="button"
                      className="flex min-w-0 flex-1 items-center gap-2 rounded-sm text-left text-[13px] outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      aria-expanded={stored.length ? !shut[key] : undefined}
                      onClick={() => stored.length && setShut((x) => ({ ...x, [key]: !x[key] }))}
                    >
                      <RoundPie className="size-4 shrink-0" total={Math.max(1, stored.length)} done={stored.length ? stored.filter((r) => r.done_at).length : i <= current ? 1 : 0} />
                      <span className={cn("truncate", i === current ? "font-semibold" : i > current && "text-muted-foreground")}>{name}</span>
                    </button>
                    {writable && (
                      <span className="absolute right-1 flex gap-1 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100">
                        {i > 0 && (
                          <Button size="xs" variant="outline" className="h-5 rounded-full px-1.5 text-[11px]" disabled={rounds.busy} onClick={() => { setShut((x) => ({ ...x, [key]: false })); void rounds.add(key, i) }}>
                            + round
                          </Button>
                        )}
                        <Button size="xs" variant="outline" className="h-5 rounded-full px-1.5 text-[11px]" onClick={() => { setTaskStage(key); setTodo(true) }}>
                          + task
                        </Button>
                      </span>
                    )}
                  </div>
                  {!shut[key] &&
                    stored.map((r, n) => (
                      <RoundDetails key={r.id} r={r} name={roundName(r, n)} writable={writable} onPatch={(body) => patch(r, body)} onDelete={() => void remove(r)} onOpenNotes={() => { if (r.in_notes === false) setShowLeftOut(true); reveal(r.id) }}>
                        <button type="button" className="flex h-7 items-center gap-2 rounded-md pr-2 pl-7 text-left text-[13px] outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring">
                          <RoundPie className="size-3.5 shrink-0" total={1} done={r.done_at ? 1 : 0} />
                          <span className={cn("min-w-0 flex-1 truncate", r.in_notes === false && "text-muted-foreground")}>{roundName(r, n)}</span>
                        </button>
                      </RoundDetails>
                    ))}
                </li>
              )
            })}
          </ul>
        </nav>

        <article aria-label="Notes" className="min-w-0">
          <Fold id="about" title="About this application" meta={application.notes ? "" : "empty"} open={!!open.about} onToggle={() => toggleFold("about")} writable={writable}>
            <MarkdownEditor value={application.notes || ""} label="Application notes" placeholder={"Anything worth remembering about this one.\n\n### A question not tied to a round?\nIts answer."} disabled={!writable} onSave={onSaveNotes} />
          </Fold>
          {included.map(({ r, n }) => (
            <Fold key={r.id} id={r.id} title={roundName(r, n)} meta={meta(r)} open={!!open[r.id]} onToggle={() => toggleFold(r.id)} included onInclude={(v) => void patch(r, { in_notes: v })} writable={writable}>
              <MarkdownEditor
                value={r.notes || ""}
                label={`${roundName(r, n)} notes`}
                placeholder={"## Prep\n\n### A question they asked?\nYour answer.\n\n## Notes"}
                disabled={!writable}
                onSave={(notes) => patch(r, { notes })}
              />
            </Fold>
          ))}
          {showLeftOut &&
            leftOut.map(({ r, n }) => (
              <Fold key={r.id} id={r.id} title={roundName(r, n)} meta="left out" open={!!open[r.id]} onToggle={() => toggleFold(r.id)} included={false} onInclude={(v) => void patch(r, { in_notes: v })} writable={writable}>
                <MarkdownEditor value={r.notes || ""} label={`${roundName(r, n)} notes`} placeholder="" disabled={!writable} onSave={(notes) => patch(r, { notes })} />
              </Fold>
            ))}
          <div className="flex items-center gap-4 pt-3 text-xs text-muted-foreground">
            <button type="button" className="flex items-center gap-1.5 hover:text-foreground" onClick={() => setCompiled(true)}>
              <FileText className="size-3.5" /> All notes as one document
            </button>
            {leftOut.length > 0 && (
              <button type="button" className="hover:text-foreground" onClick={() => setShowLeftOut((v) => !v)}>
                {showLeftOut ? "Hide left out" : `${leftOut.length} left out · show`}
              </button>
            )}
          </div>
          {rounds.error && <p className="pt-2 text-xs text-destructive">{rounds.error}</p>}
        </article>
      </div>
      <CompiledNotes appId={appId} open={compiled} onOpenChange={setCompiled} />
    </SectionCard>
  )
}
