"use client"

import { useEffect, useRef, useState, type ReactNode } from "react"
import { ChevronLeft, ChevronRight, Plus } from "lucide-react"
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
} from "@/lib/types"
import { OutcomePiece } from "@/components/Sketch"
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
  const pos = drag ?? value
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
    if (commit) onCommit(Math.round(drag))
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
        const next = e.key === "Home" ? 0 : e.key === "End" ? last : STEP[e.key] ? value + STEP[e.key] : null
        if (next == null) return
        e.preventDefault()
        if (next >= 0 && next <= last && next !== value) onCommit(next)
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

// Order on the outcome page, so 1-6 pick them: the two holds, then the stops.
const HOLDS = ["waiting", "needs_input"]
const STOPS = ["failed", "rejected", "ghosted", "withdrawn"]
const HOLD_NOTE: Record<string, string> = { waiting: "On them", needs_input: "On you" }

type View = "stage" | "outcome" | "steps"

function ViewHeader({ title, sub, onBack }: { title: string; sub: string; onBack: () => void }) {
  return (
    <div className="flex items-start justify-between">
      <Button variant="ghost" size="icon" className="size-7 text-muted-foreground" aria-label="Back to stage" onClick={onBack}>
        <ChevronLeft className="size-4" />
      </Button>
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
// (holds as tiles, stops as dice), S the steps between stages, 1-6 picks on
// either, and Backspace comes back.
export function StatusChooser({
  app,
  open,
  onOpenChange,
  onStatus,
  onAddRound,
  align = "start",
  children,
}: {
  app: Application
  open?: boolean
  onOpenChange?: (open: boolean) => void
  onStatus: (next: string) => Promise<unknown> | void
  onAddRound?: (stage: string) => Promise<unknown> | void
  align?: "start" | "center" | "end"
  children: ReactNode
}) {
  const content = useRef<HTMLDivElement>(null)
  const [pending, setPending] = useState<string | null>(null)
  const [drag, setDrag] = useState<number | null>(null)
  const [view, setView] = useState<View>("stage")
  const [adding, setAdding] = useState(false)

  // Keep keyboard focus inside the popover as the view changes.
  useEffect(() => {
    content.current?.querySelector<HTMLElement>(view === "stage" ? "[role=slider]" : "[data-view]")?.focus()
  }, [view])

  const stage = Math.min(6, Math.max(0, app.stage ?? statusPieces(app.status)))
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
  async function addRound() {
    if (!roundStage || !onAddRound) return
    setAdding(true)
    try {
      await onAddRound(roundStage[0])
    } finally {
      setAdding(false)
    }
  }

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
    else if (k === "backspace" && view !== "stage") setView("stage")
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
        onKeyDown={onKey}
      >
        {view === "outcome" && (
          <div data-view tabIndex={-1} className="flex flex-col gap-3 outline-none">
            <ViewHeader title={outcome ? label(outcome) : "Outcome"} sub={`at ${STAGE_STOPS[stage][1]}`} onBack={() => setView("stage")} />
            <div className="grid grid-cols-2 gap-1.5">
              {HOLDS.map((s, i) => {
                const on = outcome === s
                return (
                  <button
                    key={s}
                    type="button"
                    aria-pressed={on}
                    onClick={() => toggleOutcome(s)}
                    className={cn(
                      "relative flex items-center gap-2 rounded-xl border border-dashed p-2 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
                      on ? "border-amber-500 bg-amber-500/10" : "border-input hover:bg-muted",
                    )}
                  >
                    <OutcomePiece status={s} className={cn("size-7 shrink-0", HOLD_TONE)} />
                    <span className="flex min-w-0 flex-col">
                      <span className="text-[13px] leading-5 font-medium whitespace-nowrap">{label(s)}</span>
                      <span className="text-[11px] leading-4 text-muted-foreground">{HOLD_NOTE[s]}</span>
                    </span>
                    <Kbd className="absolute top-1.5 right-1.5 h-4 min-w-4 text-[10px]">{i + 1}</Kbd>
                  </button>
                )
              })}
            </div>
            <div className="grid grid-cols-4 gap-1">
              {STOPS.map((s, i) => {
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
                    <Kbd className="h-4 min-w-4 text-[10px]">{i + 3}</Kbd>
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
        {view === "stage" && (
          <div className="flex flex-col gap-3">
            <div className="flex items-start justify-between">
              {onAddRound ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-7 text-muted-foreground"
                      aria-label={roundStage ? `Add a round to ${roundStage[1]}` : "Add a round"}
                      disabled={!roundStage || adding || drag != null}
                      onClick={addRound}
                    >
                      <Plus className="size-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{roundStage ? `Add a round to ${roundStage[1]}` : "Rounds start at Prepared"}</TooltipContent>
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
