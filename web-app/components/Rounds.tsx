"use client"

import { useState } from "react"
import { Minus, Plus } from "lucide-react"
import { api, ApiError } from "@/lib/api-client"
import { cn } from "@/lib/utils"
import { ROUND_STAGES, type Interview } from "@/lib/types"
import { Button } from "@/components/ui/button"

// Every stage is at least one round, as on the progress mark, where a stage
// with none stored fills as one whole piece. That implied round is done once
// the application has reached the stage.
export const shownRounds = (stored: Interview[]) => Math.max(1, stored.length)

// A round carrying anything typed into it takes a second press to delete.
export const hasDetails = (r: Interview) => !!(r.kind || r.interviewer || r.scheduled_at || r.prep || r.notes)

// A stage's rounds as one pie: a wedge per round, clockwise from the top,
// the first `done` filled. More rounds split the same pie finer.
export function RoundPie({ total, done, className }: { total: number; done: number; className?: string }) {
  const r = 9
  const at = (k: number) => {
    const a = (k / total) * 2 * Math.PI - Math.PI / 2
    return `${(12 + r * Math.cos(a)).toFixed(2)} ${(12 + r * Math.sin(a)).toFixed(2)}`
  }
  return (
    <svg viewBox="0 0 24 24" aria-hidden className={cn("size-5", className)}>
      {total === 1 ? (
        <circle cx="12" cy="12" r={r} className={done ? "fill-foreground" : "fill-foreground/15"} />
      ) : (
        Array.from({ length: total }, (_, k) => (
          <path
            key={k}
            d={`M12 12 L${at(k)} A${r} ${r} 0 ${total === 2 ? 1 : 0} 1 ${at(k + 1)} Z`}
            strokeWidth="1.25"
            className={cn("stroke-background transition-[fill]", k < done ? "fill-foreground" : "fill-foreground/15")}
          />
        ))
      )}
      <circle cx="12" cy="12" r={r} fill="none" strokeWidth="1.25" className="stroke-foreground/40" />
    </svg>
  )
}

// Add a round: a pie cut in two, with a plus.
export function AddRoundIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden className={className}>
      <path d="M17 7.6 A8 8 0 1 0 18.4 15" />
      <path d="M10.5 4 V12 H17" />
      <path d="M19 15 V22 M15.5 18.5 H22.5" strokeLinecap="square" />
    </svg>
  )
}

// Rounds of one application, and the operations on them. `stage` is the piece
// it's on, so a split stage knows whether its first round is already done.
export function useRounds(appId: string, stage: number, onChange?: () => Promise<unknown> | void) {
  const [list, setList] = useState<Interview[] | null>(null)
  const [busy, setBusy] = useState(false)
  const [armed, setArmed] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const base = `/v1/applications/${encodeURIComponent(appId)}/interviews`

  async function fetchRounds() {
    const page = await api<{ data: Interview[] }>(base)
    const data = page.data || []
    setList(data)
    return data
  }
  const storedFor = (all: Interview[] | null, key: string) => (all ?? []).filter((r) => r.stage === key)

  async function run(request: () => Promise<unknown>) {
    if (busy) return
    setBusy(true)
    setArmed(null)
    setError(null)
    try {
      await request()
      await onChange?.()
      await fetchRounds()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't save the round")
    } finally {
      setBusy(false)
    }
  }
  // Going from one round to two splits the stage: its implied round is stored
  // first (done if the stage is reached), then the new one.
  const add = (key: string, piece: number) =>
    run(async () => {
      if (storedFor(list ?? (await fetchRounds()), key).length === 0) {
        const first = await api<Interview>(base, { method: "POST", body: { stage: key } })
        if (piece <= stage)
          await api(`/v1/interviews/${encodeURIComponent(first.id)}`, { method: "PATCH", body: { version: first.version, done: true } })
      }
      await api(base, { method: "POST", body: { stage: key } })
    })
  const toggle = (r: Interview) =>
    run(() => api(`/v1/interviews/${encodeURIComponent(r.id)}`, { method: "PATCH", body: { version: r.version, done: !r.done_at } }))
  async function remove(key: string) {
    const stored = storedFor(list ?? (await fetchRounds().catch(() => [])), key)
    if (shownRounds(stored) <= 1) return
    const r = stored[stored.length - 1]
    if (hasDetails(r) && armed !== r.id) return setArmed(r.id)
    void run(() => api(`/v1/interviews/${encodeURIComponent(r.id)}?version=${r.version}`, { method: "DELETE" }))
  }
  // How many rounds in all, counting each stage's implied one; the pies show
  // which are done.
  function summary() {
    if (!list) return "Loading…"
    const total = ROUND_STAGES.reduce((n, [key]) => n + shownRounds(storedFor(list, key)), 0)
    return `${total} rounds`
  }

  return { list, busy, armed, error, fetchRounds, run, storedFor, add, toggle, remove, summary }
}

export type RoundsApi = ReturnType<typeof useRounds>

const doneOf = (rounds: Interview[]) => rounds.filter((r) => r.done_at).length
// Clicking the pie fills the next wedge; once all are done it empties the last.
const nextToggle = (rounds: Interview[]) => rounds.find((r) => !r.done_at) ?? rounds[rounds.length - 1]

// One stage's rounds: a pie with a wedge per round (click to fill the next),
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
    <li className={cn("flex h-7 items-center gap-2 rounded-lg px-1.5", selected && "bg-muted")} onMouseEnter={onSelect}>
      <span className={cn("w-[5.5rem] shrink-0 truncate text-[13px]", current ? "font-semibold" : "text-muted-foreground")}>{name}</span>
      <span className="flex min-w-0 flex-1 items-center gap-1.5">
        {rounds.length === 0 ? (
          <span title="The stage itself; + splits it into rounds" className="flex">
            <RoundPie total={1} done={reached ? 1 : 0} />
          </span>
        ) : (
          <button
            type="button"
            aria-label={`${name}: ${doneOf(rounds)} of ${rounds.length} rounds done. ${nextToggle(rounds)?.done_at ? "Unmark" : "Mark"} the next one`}
            title={rounds.map((r, i) => `${r.done_at ? "✓" : "○"} ${r.kind || `Round ${i + 1}`}`).join("\n")}
            disabled={busy}
            onClick={() => {
              const r = nextToggle(rounds)
              if (r) onToggle(r)
            }}
            className="flex rounded-full outline-none hover:opacity-80 focus-visible:ring-2 focus-visible:ring-ring"
          >
            <RoundPie total={rounds.length} done={doneOf(rounds)} />
          </button>
        )}
        <span className="text-[11px] tabular-nums text-muted-foreground">{shownRounds(rounds)}</span>
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

// Every stage's rounds, one row each. `sel` is the row + and - keys act on.
export function RoundsList({
  rounds,
  stage,
  sel,
  onSelect,
}: {
  rounds: RoundsApi
  stage: number
  sel: number
  onSelect: (i: number) => void
}) {
  const armedRound = rounds.list?.find((r) => r.id === rounds.armed)
  return (
    <>
      <ul className="flex flex-col gap-0.5">
        {ROUND_STAGES.map(([key, name], i) => {
          const stored = rounds.storedFor(rounds.list, key)
          return (
            <RoundRow
              key={key}
              name={name}
              current={i + 1 === stage}
              selected={i === sel}
              reached={i + 1 <= stage}
              rounds={stored}
              armed={!!rounds.armed && stored.some((r) => r.id === rounds.armed)}
              busy={rounds.busy || !rounds.list}
              onSelect={() => onSelect(i)}
              onToggle={rounds.toggle}
              onRemove={() => rounds.remove(key)}
              onAdd={() => rounds.add(key, i + 1)}
            />
          )
        })}
      </ul>
      {armedRound && (
        <p className="text-[12px] text-destructive">“{armedRound.kind || "This round"}” has details. Press − again to delete it.</p>
      )}
      {rounds.error && <p className="text-[12px] text-destructive">{rounds.error}</p>}
    </>
  )
}
