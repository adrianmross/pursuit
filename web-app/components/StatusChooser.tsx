"use client"

import { useEffect, useRef, useState, type ReactNode } from "react"
import { ChevronLeft, ChevronRight, Minus, Plus } from "lucide-react"
import { api, ApiError } from "@/lib/api-client"
import { cn } from "@/lib/utils"
import {
  label,
  OUTCOMES,
  outcomeKind,
  ROUND_STAGES,
  shown,
  STAGE_STOPS,
  statusPieces,
  SUB_STEPS,
  type Application,
  type Interview,
} from "@/lib/types"
import { HoldIcon, OutcomePiece } from "@/components/Sketch"
import { Kbd } from "@/components/ui/kbd"
import { ProgressMark } from "@/components/ProgressMark"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

const STEP: Record<string, number> = { ArrowRight: 1, ArrowUp: 1, PageUp: 1, ArrowLeft: -1, ArrowDown: -1, PageDown: -1 }

// A pill track with a stop per position. The knob follows the pointer freely
// while dragging and only snaps to a stop on release, so the drag stays smooth;
// a click on the track glides the knob there instead of jumping.
export function StageSlider({
  stops,
  value,
  label: name,
  onDrag,
  onCommit,
}: {
  stops: [string, string][]
  value: number
  label: string
  // The stop under the knob while dragging, null when the drag ends.
  onDrag?: (index: number | null) => void
  onCommit: (index: number) => void
}) {
  const last = stops.length - 1
  const track = useRef<HTMLDivElement>(null)
  const [drag, setDrag] = useState<number | null>(null)
  const [sliding, setSliding] = useState(false)
  // The stop just released on, kept while `value` still shows the old one: the
  // URL or the server catches up a moment later, and without this the knob
  // would spring back and then jump forward.
  const [held, setHeld] = useState<{ at: number; from: number } | null>(null)
  const settled = held && held.from === value ? held.at : value
  const pos = drag ?? settled
  const at = Math.round(pos)
  const f = last ? pos / last : 0
  const ease = sliding ? "" : "transition-[left,width] duration-200 ease-out"

  // The knob's centre travels between the middles of the first and last 32px slots.
  function posAt(clientX: number) {
    const r = track.current!.getBoundingClientRect()
    return Math.min(1, Math.max(0, (clientX - r.left - 18) / (r.width - 36))) * last
  }
  function move(p: number) {
    setDrag(p)
    onDrag?.(Math.round(p))
  }
  function end(commit: boolean) {
    if (drag == null) return
    setDrag(null)
    setSliding(false)
    onDrag?.(null)
    if (commit) release(Math.round(drag))
  }
  function release(i: number) {
    setHeld(i === value ? null : { at: i, from: value })
    onCommit(i)
  }

  return (
    <div
      ref={track}
      role="slider"
      tabIndex={0}
      aria-label={name}
      aria-valuemin={0}
      aria-valuemax={last}
      aria-valuenow={at}
      aria-valuetext={stops[at][1]}
      className="relative h-9 cursor-grab touch-none rounded-full bg-muted p-0.5 outline-none select-none focus-visible:ring-2 focus-visible:ring-ring active:cursor-grabbing"
      onPointerDown={(e) => {
        if (e.button !== 0) return
        e.currentTarget.setPointerCapture(e.pointerId)
        move(posAt(e.clientX))
      }}
      onPointerMove={(e) => {
        if (drag == null) return
        setSliding(true)
        move(posAt(e.clientX))
      }}
      onPointerUp={() => end(true)}
      onPointerCancel={() => end(false)}
      onKeyDown={(e) => {
        const next = e.key === "Home" ? 0 : e.key === "End" ? last : STEP[e.key] ? settled + STEP[e.key] : null
        if (next == null) return
        e.preventDefault()
        if (next >= 0 && next <= last && next !== settled) release(next)
      }}
    >
      <div
        className={cn("absolute inset-y-0.5 left-0.5 rounded-full bg-primary", ease)}
        style={{ width: `calc((100% - 36px) * ${f} + 32px)` }}
      />
      <div className="relative flex h-8 items-center justify-between">
        {stops.map(([v], i) => (
          <span key={v || "all"} className="flex size-8 items-center justify-center">
            {v && (
              <span
                className={cn(
                  "size-[5px] rounded-full",
                  v === "offer" ? "bg-brand-gold" : i < pos ? "bg-primary-foreground/55" : "bg-input",
                )}
              />
            )}
          </span>
        ))}
      </div>
      <span
        className={cn(
          "absolute top-[3px] size-[30px] rounded-full bg-card shadow-[0_1px_3px_rgba(20,22,21,0.3),0_0_0_0.5px_rgba(20,22,21,0.1)]",
          ease,
        )}
        style={{ left: `calc((100% - 36px) * ${f} + 3px)` }}
      />
    </div>
  )
}

// The word above the slider with its pieces beside it; the mark's pieces fade
// in and out as the knob crosses stops.
export function StageHeading({
  stage,
  outcome,
  interviews,
  title,
  gold,
  sub,
}: {
  stage: number
  outcome?: string
  interviews?: Application["interviews"]
  title: string
  gold?: boolean
  sub: ReactNode
}) {
  return (
    <div className="flex min-w-0 flex-col items-center">
      <span className="flex items-center gap-1.5">
        <ProgressMark stage={stage} outcome={outcome} interviews={interviews} size={22} />
        <span className={cn("font-heading text-lg leading-6 font-semibold", gold && "text-gold-ink")}>{title}</span>
      </span>
      {sub}
    </div>
  )
}

const HOLD_TONE = "text-amber-600 dark:text-amber-400"
const STOP_TONE = "text-red-600 dark:text-red-400"
const toneOf = (status: string) => (outcomeKind(status) === "hold" ? HOLD_TONE : STOP_TONE)

// An outcome's piece with its name, for the pipeline filter. `dim` greys out
// an outcome the filter hides.
export function OutcomeChip({
  status,
  pressed,
  dim,
  onClick,
}: {
  status: string
  pressed: boolean
  dim?: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      onClick={onClick}
      className={cn(
        "flex h-7 items-center gap-1.5 rounded-full border pr-2.5 pl-1 text-xs outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
        dim ? "border-border bg-muted/50 text-muted-foreground hover:text-foreground" : "border-input bg-card text-foreground",
      )}
    >
      <OutcomePiece status={status} className={cn("size-5", dim ? "text-muted-foreground/60" : toneOf(status))} />
      {label(status)}
    </button>
  )
}

function SubStepChip({ status, pressed, onClick }: { status: string; pressed: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      onClick={onClick}
      className={cn(
        "h-7 rounded-full border px-2.5 text-xs outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
        pressed ? "border-primary bg-primary text-primary-foreground" : "border-border text-muted-foreground hover:text-foreground",
      )}
    >
      {label(status)}
    </button>
  )
}

export function SubSteps({ current, onPick }: { current: string; onPick: (status: string) => void }) {
  return (
    <div className="flex flex-wrap gap-1.5">
      {SUB_STEPS.map((s) => (
        <SubStepChip key={s} status={s} pressed={current === s} onClick={() => onPick(s)} />
      ))}
    </div>
  )
}

export function SectionLabel({ children }: { children: ReactNode }) {
  return <span className="text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">{children}</span>
}

// What the knob should do when released on piece `to` of an application on
// piece `from`: move to that stop, clear an outcome back to its stage, or
// nothing (so a sub-step like approved or submitted isn't rewritten).
export function stageTarget(app: Pick<Application, "status" | "outcome">, from: number, to: number): string | null {
  if (to !== from) return STAGE_STOPS[to][0]
  return app.outcome ? app.status : null
}

// Order on the outcome page: the two holds, then the stops. Key n picks the
// outcome whose die shows n.
const HOLDS = ["waiting", "needs_input"]
const STOPS = ["failed", "rejected", "ghosted", "withdrawn"]
const HOLD_NOTE: Record<string, string> = { waiting: "On them", needs_input: "On you" }

type View = "stage" | "outcome" | "steps" | "rounds"

// A round carrying anything typed into it takes a second click to delete.
const hasDetails = (r: Interview) => !!(r.kind || r.interviewer || r.scheduled_at || r.prep || r.notes)

// Every stage is at least one round, as on the progress mark, where a stage
// with none stored fills as one whole piece. That implied round is done once
// the application has reached the stage.
const shownRounds = (stored: Interview[]) => Math.max(1, stored.length)

// One stage's rounds: a pill per round (filled when done, click to toggle),
// then remove-last (never below one) and add.
function RoundRow({
  name,
  current,
  selected,
  reached,
  rounds,
  armed,
  busy,
  onSelect,
  onToggle,
  onRemove,
  onAdd,
}: {
  name: string
  current: boolean
  selected: boolean
  reached: boolean
  rounds: Interview[]
  armed: boolean
  busy: boolean
  onSelect: () => void
  onToggle: (r: Interview) => void
  onRemove: () => void
  onAdd: () => void
}) {
  return (
    <li className={cn("flex h-8 items-center gap-2 rounded-lg px-1.5", selected && "bg-muted")} onMouseEnter={onSelect}>
      <span className={cn("w-[5.5rem] shrink-0 truncate text-[13px]", current ? "font-semibold" : "text-muted-foreground")}>{name}</span>
      <span className="flex min-w-0 flex-1 flex-wrap items-center gap-1">
        {rounds.length === 0 && (
          <span
            title="The stage itself; + splits it into rounds"
            className={cn("h-2.5 w-5 rounded-full border-[1.5px]", reached ? "border-primary bg-primary" : "border-foreground/50")}
          />
        )}
        {rounds.map((r, i) => (
          <button
            key={r.id}
            type="button"
            aria-pressed={!!r.done_at}
            aria-label={`${r.kind || `Round ${i + 1}`}: ${r.done_at ? "done" : "not done"}`}
            title={r.kind || `Round ${i + 1}`}
            disabled={busy}
            onClick={() => onToggle(r)}
            className={cn(
              "h-2.5 w-5 rounded-full border-[1.5px] outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
              r.done_at ? "border-primary bg-primary" : "border-foreground/50 hover:bg-foreground/15",
            )}
          />
        ))}
      </span>
      <Button
        variant="ghost"
        size="icon"
        className={cn("size-6", armed ? "text-destructive hover:text-destructive" : "text-muted-foreground")}
        aria-label={armed ? `Confirm deleting the last ${name} round` : `Remove the last ${name} round`}
        disabled={shownRounds(rounds) <= 1 || busy}
        onClick={onRemove}
      >
        <Minus className="size-3.5" />
      </Button>
      <Button variant="ghost" size="icon" className="size-6 text-muted-foreground" aria-label={`Add a ${name} round`} disabled={busy} onClick={onAdd}>
        <Plus className="size-3.5" />
      </Button>
    </li>
  )
}

function ViewHeader({ title, sub, onBack }: { title: string; sub: string; onBack: () => void }) {
  return (
    <div className="flex items-start justify-between">
      <Tooltip>
        <TooltipTrigger asChild>
          <Button variant="ghost" size="icon" className="size-7 text-muted-foreground" aria-label="Back to stage" aria-keyshortcuts="Escape" onClick={onBack}>
            <ChevronLeft className="size-4" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>
          Back <Kbd>Esc</Kbd>
        </TooltipContent>
      </Tooltip>
      <div className="flex flex-col items-center">
        <span className="font-heading text-lg leading-6 font-semibold">{title}</span>
        <span className="text-[13px] leading-[18px] text-muted-foreground">{sub}</span>
      </div>
      <span className="size-7" />
    </div>
  )
}

// The status chooser for one application. Its first view is only the stage:
// drag or [ ] to move along the pieces, + to add a round. O opens the outcomes
// (each a die showing its key), S the steps between stages, R the rounds of
// every stage, 1-6 picks on outcomes and steps, and Backspace comes back.
export function StatusChooser({
  app,
  open,
  onOpenChange,
  onStatus,
  onRoundsChange,
  align = "start",
  children,
}: {
  app: Application
  open?: boolean
  onOpenChange?: (open: boolean) => void
  onStatus: (next: string) => Promise<unknown> | void
  // Called after a round is added, removed or marked done, to refetch the app.
  // Without it the chooser offers no rounds.
  onRoundsChange?: () => Promise<unknown> | void
  align?: "start" | "center" | "end"
  children: ReactNode
}) {
  const content = useRef<HTMLDivElement>(null)
  const [pending, setPending] = useState<string | null>(null)
  const [drag, setDrag] = useState<number | null>(null)
  const [view, setView] = useState<View>("stage")
  const [list, setList] = useState<Interview[] | null>(null)
  const [busy, setBusy] = useState(false)
  const [armed, setArmed] = useState<string | null>(null)
  const [roundError, setRoundError] = useState<string | null>(null)
  const base = `/v1/applications/${encodeURIComponent(app.id)}/interviews`

  // Keep keyboard focus inside the popover as the view changes.
  useEffect(() => {
    content.current?.querySelector<HTMLElement>(view === "stage" ? "[role=slider]" : "[data-view]")?.focus()
  }, [view])

  async function fetchRounds() {
    const page = await api<{ data: Interview[] }>(base)
    const data = page.data || []
    setList(data)
    return data
  }
  useEffect(() => {
    if (view !== "rounds") return
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchRounds().catch(() => setRoundError("Couldn't load the rounds"))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, app.id])

  const stage = Math.min(6, Math.max(0, app.stage ?? statusPieces(app.status)))
  // The row the + and - keys act on in the rounds view.
  const [sel, setSel] = useState(0)
  const storedFor = (all: Interview[] | null, key: string) => (all ?? []).filter((r) => r.stage === key)

  async function roundOp(request: () => Promise<unknown>) {
    if (busy) return
    setBusy(true)
    setArmed(null)
    setRoundError(null)
    try {
      await request()
      await onRoundsChange?.()
      await fetchRounds()
    } catch (err) {
      setRoundError(err instanceof ApiError ? err.message : "Couldn't save the round")
    } finally {
      setBusy(false)
    }
  }
  // Going from one round to two splits the stage: its implied round is stored
  // first (done if the stage is reached), then the new one.
  const addRoundTo = (key: string, piece: number) =>
    roundOp(async () => {
      if (storedFor(list ?? (await fetchRounds()), key).length === 0) {
        const first = await api<Interview>(base, { method: "POST", body: { stage: key } })
        if (piece <= stage)
          await api(`/v1/interviews/${encodeURIComponent(first.id)}`, { method: "PATCH", body: { version: first.version, done: true } })
      }
      await api(base, { method: "POST", body: { stage: key } })
    })
  const toggleRound = (r: Interview) =>
    roundOp(() => api(`/v1/interviews/${encodeURIComponent(r.id)}`, { method: "PATCH", body: { version: r.version, done: !r.done_at } }))
  async function removeLast(key: string) {
    const stored = storedFor(list ?? (await fetchRounds().catch(() => [])), key)
    if (shownRounds(stored) <= 1) return
    const r = stored[stored.length - 1]
    if (hasDetails(r) && armed !== r.id) return setArmed(r.id)
    void roundOp(() => api(`/v1/interviews/${encodeURIComponent(r.id)}?version=${r.version}`, { method: "DELETE" }))
  }
  // Done out of shown, counting each stage's implied round.
  function roundsSummary(all: Interview[]) {
    let done = 0
    let total = 0
    ROUND_STAGES.forEach(([key], i) => {
      const stored = storedFor(all, key)
      total += shownRounds(stored)
      done += stored.length ? stored.filter((r) => r.done_at).length : i + 1 <= stage ? 1 : 0
    })
    return `${done} of ${total} rounds done`
  }
  function openRounds() {
    setSel(Math.max(0, stage - 1))
    setView("rounds")
  }
  // While a change is in flight, show where it's going.
  const outcome = pending ? (OUTCOMES.includes(pending) ? pending : "") : app.outcome || ""
  const current = pending ?? shown(app)
  const settled = pending && !OUTCOMES.includes(pending) && pending !== app.status ? statusPieces(pending) : stage
  const at = drag ?? settled
  const status = drag != null ? STAGE_STOPS[drag][0] : current
  const rounds = app.interviews?.[at]
  const roundStage = at > 0 ? ROUND_STAGES[at - 1] : null

  async function set(next: string | null) {
    if (!next || next === shown(app) || pending) return
    setPending(next)
    try {
      await onStatus(next)
    } finally {
      setPending(null)
    }
  }
  const toggleOutcome = (s: string) => set(outcome === s ? app.status : s)
  const armedRound = list?.find((r) => r.id === armed)

  const sub =
    drag != null
      ? `Piece ${at} of 6`
      : outcome
        ? `${label(outcome)} at ${STAGE_STOPS[stage][1]}`
        : rounds?.total
          ? `${rounds.done} of ${rounds.total} round${rounds.total === 1 ? "" : "s"} done`
          : SUB_STEPS.includes(current)
            ? `${label(current)}, a step of ${STAGE_STOPS[stage][1]}`
            : `Piece ${at} of 6`

  function onKey(e: React.KeyboardEvent) {
    if (e.metaKey || e.ctrlKey || e.altKey) return
    const k = e.key.toLowerCase()
    const n = Number(e.key)
    if (k === "[" || k === "]") {
      const to = settled + (k === "]" ? 1 : -1)
      if (to >= 0 && to <= 6) void set(stageTarget(app, stage, to))
    } else if (k === "o") setView(view === "outcome" ? "stage" : "outcome")
    else if (k === "s") setView(view === "steps" ? "stage" : "steps")
    else if (k === "r" && onRoundsChange) {
      if (view === "rounds") setView("stage")
      else openRounds()
    }
    else if ((k === "backspace" || k === "arrowleft") && view !== "stage") setView("stage")
    // + and - add or remove a round: on the selected row in the rounds view,
    // else on the stage the application is on.
    else if ((k === "+" || k === "=" || k === "-" || k === "_") && onRoundsChange && (view === "rounds" || view === "stage")) {
      const piece = view === "rounds" ? sel + 1 : settled
      if (piece < 1) return
      const key = ROUND_STAGES[piece - 1][0]
      if (k === "-" || k === "_") void removeLast(key)
      else void addRoundTo(key, piece)
    } else if (view === "rounds" && (k === "arrowdown" || k === "arrowup"))
      setSel(Math.min(ROUND_STAGES.length - 1, Math.max(0, sel + (k === "arrowdown" ? 1 : -1))))
    else if (view === "outcome" && n >= 1 && n <= 6) void toggleOutcome([...HOLDS, ...STOPS][n - 1])
    else if (view === "steps" && n >= 1 && n <= SUB_STEPS.length) void set(SUB_STEPS[n - 1])
    else return
    e.preventDefault()
  }

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        if (!next) setView("stage")
        onOpenChange?.(next)
      }}
    >
      <PopoverTrigger asChild>{children}</PopoverTrigger>
      <PopoverContent
        ref={content}
        align={align}
        className="w-72 rounded-2xl p-3"
        onOpenAutoFocus={(e) => {
          e.preventDefault()
          content.current?.querySelector<HTMLElement>("[role=slider]")?.focus()
        }}
        onCloseAutoFocus={(e) => e.preventDefault()}
        // Esc on a sub-view steps back to the stage; on the stage it closes.
        onEscapeKeyDown={(e) => {
          if (view === "stage") return
          e.preventDefault()
          setView("stage")
        }}
        onKeyDown={onKey}
      >
        {view === "outcome" && (
          <div data-view tabIndex={-1} className="flex flex-col gap-3 outline-none">
            <ViewHeader title={outcome ? label(outcome) : "Outcome"} sub={`at ${STAGE_STOPS[stage][1]}`} onBack={() => setView("stage")} />
            <div className="grid grid-cols-2 gap-1.5">
              {HOLDS.map((s) => {
                const on = outcome === s
                return (
                  <button
                    key={s}
                    type="button"
                    aria-pressed={on}
                    onClick={() => toggleOutcome(s)}
                    className={cn(
                      "flex items-center gap-2 rounded-xl border border-dashed p-2 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
                      on ? "border-amber-500 bg-amber-500/10" : "border-input hover:bg-muted",
                    )}
                  >
                    <OutcomePiece status={s} className={cn("size-8 shrink-0", HOLD_TONE)} />
                    <span className="flex min-w-0 flex-col">
                      <span className="text-[13px] leading-5 font-medium whitespace-nowrap">{label(s)}</span>
                      <span className="flex items-center gap-1 text-[11px] leading-4 text-muted-foreground">
                        <HoldIcon status={s} className="size-3" />
                        {HOLD_NOTE[s]}
                      </span>
                    </span>
                  </button>
                )
              })}
            </div>
            <div className="grid grid-cols-4 gap-1">
              {STOPS.map((s) => {
                const on = outcome === s
                return (
                  <button
                    key={s}
                    type="button"
                    aria-pressed={on}
                    onClick={() => toggleOutcome(s)}
                    className={cn(
                      "flex flex-col items-center gap-1 rounded-lg py-1.5 outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
                      on ? "bg-red-500/10 ring-1 ring-red-500/50" : "hover:bg-muted",
                    )}
                  >
                    <OutcomePiece status={s} className={cn("size-8", on || !outcome ? STOP_TONE : "text-muted-foreground/60")} />
                    <span className="text-[11px] leading-4">{label(s)}</span>
                  </button>
                )
              })}
            </div>
          </div>
        )}
        {view === "steps" && (
          <div data-view tabIndex={-1} className="flex flex-col gap-3 outline-none">
            <ViewHeader title="Steps between" sub="the agent's work between stages" onBack={() => setView("stage")} />
            {/* A short stretch of board: each step a square on one path. */}
            <ol className="relative flex flex-col">
              <span aria-hidden className="absolute top-3 bottom-3 left-[13px] w-0.5 bg-border" />
              {SUB_STEPS.map((s, i) => {
                const on = current === s
                const group = STAGE_STOPS[statusPieces(s)][1]
                const first = i === 0 || statusPieces(SUB_STEPS[i - 1]) !== statusPieces(s)
                return (
                  <li key={s}>
                    <button
                      type="button"
                      aria-pressed={on}
                      onClick={() => set(s)}
                      className="relative flex w-full items-center gap-3 rounded-lg py-1 pr-1 pl-1.5 text-left outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
                    >
                      <span
                        className={cn(
                          "size-3.5 shrink-0 rounded-[3px] border-2",
                          on ? "border-primary bg-primary" : "border-foreground/70 bg-background",
                        )}
                      />
                      <span className={cn("flex-1 text-sm", on && "font-semibold")}>{label(s)}</span>
                      {first && <span className="text-[11px] text-muted-foreground">{group}</span>}
                      <Kbd className="h-4 min-w-4 text-[10px]">{i + 1}</Kbd>
                    </button>
                  </li>
                )
              })}
            </ol>
          </div>
        )}
        {view === "rounds" && (
          <div data-view tabIndex={-1} className="flex flex-col gap-3 outline-none">
            <ViewHeader
              title="Rounds"
              sub={list ? roundsSummary(list) : "Loading…"}
              onBack={() => setView("stage")}
            />
            <ul className="flex flex-col gap-0.5">
              {ROUND_STAGES.map(([key, name], i) => {
                const rounds = storedFor(list, key)
                return (
                  <RoundRow
                    key={key}
                    name={name}
                    current={i + 1 === stage}
                    selected={i === sel}
                    reached={i + 1 <= stage}
                    rounds={rounds}
                    armed={!!armed && rounds.some((r) => r.id === armed)}
                    busy={busy || !list}
                    onSelect={() => setSel(i)}
                    onToggle={toggleRound}
                    onRemove={() => removeLast(key)}
                    onAdd={() => addRoundTo(key, i + 1)}
                  />
                )
              })}
            </ul>
            <p className="text-center text-[11px] text-muted-foreground">
              <Kbd>+</Kbd> <Kbd>−</Kbd> rounds · <Kbd>↑</Kbd> <Kbd>↓</Kbd> stage · <Kbd>Esc</Kbd> back
            </p>
            {armedRound && (
              <p className="text-[12px] text-destructive">
                “{armedRound.kind || "This round"}” has details. Press − again to delete it.
              </p>
            )}
            {roundError && <p className="text-[12px] text-destructive">{roundError}</p>}
          </div>
        )}
        {view === "stage" && (
          <div className="flex flex-col gap-3">
            <div className="flex items-start justify-between">
              {onRoundsChange ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-7 text-muted-foreground"
                      aria-label={roundStage ? `Add a round to ${roundStage[1]}` : "Add a round"}
                      disabled={!roundStage || busy || drag != null}
                      onClick={() => roundStage && addRoundTo(roundStage[0], at)}
                    >
                      <Plus className="size-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    {roundStage ? `Add a round to ${roundStage[1]}` : "Rounds start at Prepared"} · all rounds <Kbd>R</Kbd>
                  </TooltipContent>
                </Tooltip>
              ) : (
                <span className="size-7" />
              )}
              <StageHeading
                stage={at}
                outcome={drag == null ? outcome : undefined}
                interviews={app.interviews}
                title={label(status)}
                gold={status === "offer"}
                sub={
                  <button
                    type="button"
                    aria-keyshortcuts="S"
                    onClick={() => setView("steps")}
                    className="flex items-center gap-0.5 rounded text-[13px] leading-[18px] text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    {sub}
                    <ChevronRight className="size-3" />
                  </button>
                }
              />
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-7 text-muted-foreground"
                    aria-label="Outcome"
                    aria-keyshortcuts="O"
                    onClick={() => setView("outcome")}
                  >
                    <OutcomePiece status={outcome || "rejected"} className={cn("size-5", outcome && toneOf(outcome))} />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  Outcome <Kbd>O</Kbd>
                </TooltipContent>
              </Tooltip>
            </div>
            <StageSlider
              stops={STAGE_STOPS}
              value={settled}
              label="Stage"
              onDrag={setDrag}
              onCommit={(to) => void set(stageTarget(app, stage, to))}
            />
          </div>
        )}
      </PopoverContent>
    </Popover>
  )
}
