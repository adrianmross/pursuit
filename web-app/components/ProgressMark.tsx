import { outcomeKind } from "@/lib/types"

// The logo's p as a game piece: five ring segments plus a sixth, gold tail
// on an offer. Geometry from the Hire Pursuit design system's ProgressMark.
// The ring paths run clockwise from the upper left; stages fill them the other
// way round — bottom first, counter-clockwise up to the tail, then down it.
const PIECES = [
  { d: "M34.73 64.55 A20.5 20.5 0 0 1 17.34 64.58", fromEnd: true },
  { d: "M46.36 48.36 A20.5 20.5 0 0 1 37.23 63.15", fromEnd: true },
  { d: "M38.73 29.93 A20.5 20.5 0 0 1 46.49 45.50", fromEnd: true },
  { d: "M19.06 26.71 A20.5 20.5 0 0 1 36.37 28.32", fromEnd: true },
  { d: "M5.95 41.74 A20.5 20.5 0 0 1 16.44 27.87", fromEnd: true },
  { d: "M5.5 46 V84", fromEnd: false },
]
// Share of a piece left open between the sub-arcs of several rounds.
const GAP = 0.1

type Counts = Record<string, { done: number; total: number }>

type Props = {
  stage: number
  outcome?: string
  // Rounds per stage number: each one is a sub-arc of that stage's piece.
  interviews?: Counts
  size?: number
}

// Arc draws the stretch [from, to] of a piece, measured from the end where
// filling starts. pathLength=1 turns the dash pattern into fractions.
function Arc({
  d,
  fromEnd,
  from = 0,
  to = 1,
  className,
}: {
  d: string
  fromEnd: boolean
  from?: number
  to?: number
  className: string
}) {
  const partial = from > 0 || to < 1
  // A dash starting at `start` along the path covers [start, start + length].
  const start = fromEnd ? 1 - to : from
  return (
    <path
      d={d}
      pathLength={1}
      fill="none"
      strokeWidth={11}
      // Pieces fade in and out as the stage changes (the status chooser leans on this).
      className={`transition-[stroke,opacity] duration-300 ease-out ${className}`}
      strokeDasharray={partial ? `${to - from} 2` : undefined}
      strokeDashoffset={partial ? -start : undefined}
    />
  )
}

export function ProgressMark({ stage, outcome, interviews, size = 32 }: Props) {
  const n = Math.max(0, Math.min(6, stage))
  const kind = outcomeKind(outcome)
  const tint = kind === "stop" ? "stroke-red-500" : kind === "hold" ? "stroke-amber-500" : ""
  // The overlay colours the piece the application is on; before the first
  // piece it marks the one that fills next.
  const target = kind ? (n === 0 ? 1 : n) : 0

  return (
    <svg
      className="inline-block shrink-0 align-middle"
      width={Math.round((size * 74) / 80)}
      height={size}
      viewBox="-11 9 74 80"
      role="img"
      aria-label={`${n} of 6 pieces${outcome ? `, ${outcome.replace("_", " ")}` : ""}`}
    >
      {PIECES.map(({ d, fromEnd }, i) => {
        const piece = i + 1
        const reached = n >= piece
        const tinted = piece === target
        const base = tinted ? tint : piece === 6 ? "stroke-brand-gold" : "stroke-foreground"
        const faded = reached || tinted ? `${base} opacity-30` : "stroke-border"
        const rounds = interviews?.[piece]
        const total = rounds?.total ?? 0
        if (total === 0) {
          return <Arc key={piece} d={d} fromEnd={fromEnd} className={reached || tinted ? base : "stroke-border"} />
        }
        const done = rounds?.done ?? 0
        return (
          <g key={piece}>
            {Array.from({ length: total }, (_, k) => (
              <Arc
                key={k}
                d={d}
                fromEnd={fromEnd}
                from={total === 1 ? 0 : k / total + GAP / 2}
                to={total === 1 ? 1 : (k + 1) / total - GAP / 2}
                className={k < done ? base : faded}
              />
            ))}
          </g>
        )
      })}
    </svg>
  )
}
