"use client"

import { useEffect, useState, type ReactNode } from "react"
import { Trash2 } from "lucide-react"
import { api } from "@/lib/api-client"
import { cn } from "@/lib/utils"
import { STAGE_STOPS, type Application, type Interview, type Page, type Question } from "@/lib/types"
import { RoundPie, TaskList, useRounds, useTasks } from "@/components/Rounds"
import { PreparedAnswers } from "@/components/PreparedAnswers"
import { InlineText } from "@/components/InlineEdit"
import { SectionCard } from "@/components/detail/SectionCard"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"

const day = (value?: string) => (value ? value.slice(0, 10) : "")

const openTasks = (list: { done_at?: string }[]) => list.filter((t) => !t.done_at).length

function plural(n: number, word: string) {
  return `${n} ${word}${n === 1 ? "" : "s"}`
}

// A markdown document edited in place: Save and Discard appear once it changes.
function MarkdownEditor({ value, placeholder, disabled, label, onSave }: { value: string; placeholder: string; disabled: boolean; label: string; onSave: (next: string) => Promise<unknown> }) {
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
        aria-label={label}
        className="min-h-24 font-mono text-[13px] leading-relaxed"
        rows={Math.max(4, draft.split("\n").length + 1)}
        placeholder={placeholder}
        value={draft}
        disabled={disabled}
        onChange={(e) => setDraft(e.target.value)}
      />
      {dirty && !disabled && (
        <div className="flex gap-2">
          <Button
            size="sm"
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
          <Button size="sm" variant="ghost" onClick={() => setDraft(value)}>
            Discard
          </Button>
        </div>
      )}
    </div>
  )
}

// One line of the notes outline: ▸ Title ─── meta, opening to what is under it.
// A round can be left out of notes, which folds it to a dim line.
function Fold({
  id,
  title,
  meta,
  open,
  included = true,
  writable,
  onToggle,
  onInclude,
  children,
}: {
  id: string
  title: string
  meta?: string
  open: boolean
  included?: boolean
  writable: boolean
  onToggle: () => void
  onInclude?: (next: boolean) => void
  children: ReactNode
}) {
  const shown = open && included
  return (
    <div id={`note-${id}`} className="scroll-mt-24 border-t py-1.5 first:border-t-0">
      <div className="flex items-center gap-3">
        <button
          type="button"
          className="flex min-w-0 flex-1 items-center gap-2.5 rounded-sm py-1 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-default"
          onClick={onToggle}
          disabled={!included}
          aria-expanded={shown}
        >
          <span aria-hidden className="w-3 shrink-0 font-mono text-xs text-muted-foreground">
            {included ? (shown ? "▾" : "▸") : "·"}
          </span>
          <span className={cn("truncate text-[15px] font-semibold", !included && "font-normal text-muted-foreground")}>{title}</span>
          <span aria-hidden className="h-px min-w-6 flex-1 bg-border" />
          <span className="shrink-0 text-xs text-muted-foreground">{included ? meta : "left out of notes"}</span>
        </button>
        {onInclude && (
          <label className="flex shrink-0 items-center gap-1.5 text-xs text-muted-foreground">
            <Checkbox checked={included} disabled={!writable} onCheckedChange={(v) => onInclude(v === true)} />
            In notes
          </label>
        )}
      </div>
      {shown && <div className="pt-2 pb-2 pl-5">{children}</div>}
    </div>
  )
}

// Rounds, tasks and notes in one card: an outline of the stages and their
// rounds on the left, the application's notes on the right with each round's
// markdown folded under its own line.
export function OutlineCard({
  application,
  writable,
  onChange,
  onSaveNotes,
}: {
  application: Application
  writable: boolean
  onChange: () => Promise<void>
  onSaveNotes: (notes: string) => Promise<unknown>
}) {
  const appId = application.id
  const current = application.stage ?? 0
  const currentKey = STAGE_STOPS[current][0]
  const rounds = useRounds(appId, current, onChange)
  const tasks = useTasks(appId, current, onChange)
  const [form, setForm] = useState<Question[]>([])
  const [shut, setShut] = useState<Record<string, boolean>>({})
  const [open, setOpen] = useState<Record<string, boolean>>({})
  const [taskStage, setTaskStage] = useState<string | null>(null)
  const [pendingOpen, setPendingOpen] = useState<string | null>(null)

  useEffect(() => {
    rounds.fetchRounds().catch(() => {})
    tasks.fetchTasks().catch(() => {})
    api<Page<Question>>(`/v1/applications/${encodeURIComponent(appId)}/questions`)
      .then((page) => setForm(page.data || []))
      .catch(() => {})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [appId])

  // After + round, open the round it made.
  useEffect(() => {
    if (!pendingOpen || !rounds.list) return
    const made = rounds.storedFor(rounds.list, pendingOpen)
    const last = made[made.length - 1]
    if (!last) return
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setOpen((o) => ({ ...o, [last.id]: true }))
    setPendingOpen(null)
    document.getElementById(`note-${last.id}`)?.scrollIntoView({ block: "nearest", behavior: "smooth" })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rounds.list, pendingOpen])

  const unansweredForm = form.filter((q) => q.required && !q.answer)
  const isOpen = (r: Interview) => open[r.id] ?? (r.stage === currentKey && r.in_notes !== false)
  const patch = (r: Interview, body: Record<string, unknown>) =>
    rounds.run(() => api(`/v1/rounds/${encodeURIComponent(r.id)}`, { method: "PATCH", body: { version: r.version, ...body } }))
  const remove = (r: Interview) => rounds.run(() => api(`/v1/rounds/${encodeURIComponent(r.id)}?version=${r.version}`, { method: "DELETE" }))
  const reveal = (id: string) => {
    setOpen((o) => ({ ...o, [id]: true }))
    requestAnimationFrame(() => document.getElementById(`note-${id}`)?.scrollIntoView({ block: "nearest", behavior: "smooth" }))
  }

  // The next step: an open task on the current stage, then on any stage, then
  // a form question waiting on an answer.
  const allTasks = tasks.list ?? []
  const nextTask = allTasks.find((t) => !t.done_at && t.stage === currentKey) ?? allTasks.find((t) => !t.done_at)
  const nextStage = nextTask && STAGE_STOPS.find(([k]) => k === nextTask.stage)
  const next = nextTask
    ? { title: nextTask.title, where: nextStage?.[1] ?? "" }
    : unansweredForm[0]?.prompt
      ? { title: `Answer: ${unansweredForm[0].prompt}`, where: "Apply · form question" }
      : null

  const stages = STAGE_STOPS.map(([key, name], i) => {
    const stored = rounds.storedFor(rounds.list, key)
    const own = tasks.forStage(key)
    const waiting = openTasks(own) + (i === 0 ? unansweredForm.length : 0)
    const questions = stored.reduce((n, r) => n + (r.questions?.length ?? 0), 0)
    const counts = [waiting ? `${waiting} open` : "", questions ? `${questions}?` : ""].filter(Boolean).join(" · ")
    const shown = stored.length > 0 || own.length > 0 || (i === 0 && form.length > 0) || taskStage === key || i === current
    return { key, name, i, stored, own, counts, shown }
  })

  return (
    <SectionCard id="outline" title="Rounds, tasks and notes" summary={rounds.summary()}>
      {next && (
        <div className="flex items-center gap-3 rounded-lg border border-foreground px-3.5 py-2.5 text-sm">
          <span className="text-[11px] font-semibold tracking-wider">NEXT</span>
          <span className="min-w-0 flex-1 truncate font-medium">{next.title}</span>
          <span className="shrink-0 text-xs text-muted-foreground">{next.where}</span>
        </div>
      )}
      <div className="grid gap-6 md:grid-cols-[200px_minmax(0,1fr)]">
        <nav aria-label="Stages and rounds">
          <ul className="flex flex-col gap-px">
            {stages.map((s) => (
              <li key={s.key} className="flex flex-col gap-px">
                <div className="group relative flex h-8 items-center gap-2 rounded-md px-2 hover:bg-muted">
                  <button
                    type="button"
                    className="flex min-w-0 flex-1 items-center gap-2 rounded-sm text-left text-[13px] outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    aria-expanded={s.stored.length ? !shut[s.key] : undefined}
                    onClick={() => (s.stored.length ? setShut((x) => ({ ...x, [s.key]: !x[s.key] })) : document.getElementById(`stage-${s.key}`)?.scrollIntoView({ block: "nearest", behavior: "smooth" }))}
                  >
                    <RoundPie className="size-4 shrink-0" total={Math.max(1, s.stored.length)} done={s.stored.length ? s.stored.filter((r) => r.done_at).length : s.i <= current ? 1 : 0} />
                    <span className={cn("truncate", s.i === current ? "font-semibold" : s.i > current && "text-muted-foreground")}>{s.name}</span>
                  </button>
                  <span className="shrink-0 text-xs text-muted-foreground group-focus-within:opacity-0 group-hover:opacity-0">{s.counts}</span>
                  {writable && (
                    <span className="absolute right-1.5 flex gap-1 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100">
                      {s.i > 0 && (
                        <Button
                          size="xs"
                          variant="outline"
                          className="h-5 rounded-full px-1.5 text-[11px]"
                          disabled={rounds.busy}
                          onClick={() => {
                            setShut((x) => ({ ...x, [s.key]: false }))
                            setPendingOpen(s.key)
                            void rounds.add(s.key, s.i)
                          }}
                        >
                          + round
                        </Button>
                      )}
                      <Button
                        size="xs"
                        variant="outline"
                        className="h-5 rounded-full px-1.5 text-[11px]"
                        onClick={() => {
                          setTaskStage(s.key)
                          requestAnimationFrame(() => document.getElementById(`stage-${s.key}`)?.scrollIntoView({ block: "nearest", behavior: "smooth" }))
                        }}
                      >
                        + task
                      </Button>
                    </span>
                  )}
                </div>
                {!shut[s.key] &&
                  s.stored.map((r, n) => (
                    <button
                      key={r.id}
                      type="button"
                      className="flex h-7 items-center gap-2 rounded-md pr-2 pl-7 text-left text-[13px] outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
                      onClick={() => reveal(r.id)}
                    >
                      <RoundPie className="size-3.5 shrink-0" total={1} done={r.done_at ? 1 : 0} />
                      <span className={cn("min-w-0 flex-1 truncate", r.in_notes === false && "text-muted-foreground")}>{r.kind || `Round ${n + 1}`}</span>
                      {(r.questions?.length ?? 0) > 0 && <span className="text-xs text-muted-foreground">{r.questions?.length}?</span>}
                    </button>
                  ))}
              </li>
            ))}
          </ul>
          <p className="mt-3 px-2 text-xs text-muted-foreground">Hover a stage for + round and + task.</p>
        </nav>

        <article aria-label="Notes" className="min-w-0">
          <Fold id="about" title="About this application" meta={application.notes ? "" : "empty"} open={open.about ?? !!application.notes} writable={writable} onToggle={() => setOpen((o) => ({ ...o, about: !(o.about ?? !!application.notes) }))}>
            <MarkdownEditor value={application.notes || ""} label="Application notes" placeholder="Anything worth remembering about this one…" disabled={!writable} onSave={onSaveNotes} />
          </Fold>
          {stages
            .filter((s) => s.shown)
            .map((s) => (
              <section key={s.key} id={`stage-${s.key}`} className="scroll-mt-24 pt-4">
                <h3 className={cn("mb-1 text-xs font-semibold tracking-wider uppercase", s.i === current ? "text-foreground" : "text-muted-foreground")}>{s.name}</h3>
                {s.i === 0 && form.length > 0 && (
                  <Fold id="form" title="Form questions" meta={unansweredForm.length ? `${plural(unansweredForm.length, "answer")} needed` : plural(form.length, "question")} open={open.form ?? unansweredForm.length > 0} writable={writable} onToggle={() => setOpen((o) => ({ ...o, form: !(o.form ?? unansweredForm.length > 0) }))}>
                    <PreparedAnswers questions={form} />
                  </Fold>
                )}
                {s.stored.map((r, n) => {
                  const name = r.kind || `Round ${n + 1}`
                  const meta = [day(r.scheduled_at), r.interviewer, r.questions?.length ? `${r.questions.length}?` : "", r.done_at ? "done" : ""].filter(Boolean).join(" · ")
                  return (
                    <Fold
                      key={r.id}
                      id={r.id}
                      title={name}
                      meta={meta}
                      open={isOpen(r)}
                      included={r.in_notes !== false}
                      writable={writable}
                      onToggle={() => setOpen((o) => ({ ...o, [r.id]: !isOpen(r) }))}
                      onInclude={(v) => void patch(r, { in_notes: v })}
                    >
                      <div className="flex flex-col gap-2.5">
                        <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-sm">
                          <label className="flex items-center gap-1.5 text-muted-foreground">
                            <Checkbox checked={!!r.done_at} disabled={!writable} onCheckedChange={(v) => void patch(r, { done: v === true })} />
                            Done
                          </label>
                          <InlineText value={r.kind || ""} placeholder={`Round ${n + 1}`} label="Round name" disabled={!writable} className="w-auto font-medium" onSave={(v) => patch(r, { kind: v })} />
                          <Input type="date" aria-label="Date" className="h-7 w-36 text-sm" value={day(r.scheduled_at)} disabled={!writable} onChange={(e) => void patch(r, { scheduled_at: e.target.value })} />
                          <InlineText value={r.interviewer || ""} placeholder="Who" label="Interviewer" disabled={!writable} className="w-auto text-muted-foreground" onSave={(v) => patch(r, { interviewer: v })} />
                          <span className="flex-1" />
                          {writable && (
                            <Button variant="ghost" size="icon-xs" aria-label={`Delete ${name}`} onClick={() => void remove(r)}>
                              <Trash2 className="size-3.5" />
                            </Button>
                          )}
                        </div>
                        <MarkdownEditor
                          value={r.notes || ""}
                          label={`${name} notes`}
                          placeholder={"## Prep\nWhat to read, stories to tell.\n\n### A question they asked?\nYour answer.\n\n## Notes\nHow it went."}
                          disabled={!writable}
                          onSave={(notes) => patch(r, { notes })}
                        />
                        <p className="text-xs text-muted-foreground">Markdown. A heading that ends in ? is a question, and the text under it is its answer.</p>
                      </div>
                    </Fold>
                  )
                })}
                {(s.own.length > 0 || taskStage === s.key) && (
                  <div className="pt-1.5">
                    <TaskList stageKey={s.key} name={s.name} tasks={s.own} api={tasks} autoFocus={taskStage === s.key} />
                  </div>
                )}
              </section>
            ))}
          {rounds.error && <p className="pt-2 text-xs text-destructive">{rounds.error}</p>}
        </article>
      </div>
    </SectionCard>
  )
}
