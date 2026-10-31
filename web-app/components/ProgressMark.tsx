import { outcomeKind } from "@/lib/types"

// The logo's p as a game piece: five ring segments plus a sixth, gold tail
// on an offer. Geometry from the Hire Pursuit design system's ProgressMark.
// Stages fill them bottom first, counter-clockwise up to the tail, then down it.
const PIECES = [
  "M34.73 64.55 A20.5 20.5 0 0 1 17.34 64.58",
  "M46.36 48.36 A20.5 20.5 0 0 1 37.23 63.15",
  "M38.73 29.93 A20.5 20.5 0 0 1 46.49 45.50",
  "M19.06 26.71 A20.5 20.5 0 0 1 36.37 28.32",
  "M5.95 41.74 A20.5 20.5 0 0 1 16.44 27.87",
  "M5.5 46 V84",
]

type Counts = Record<string, { done: number; total: number }>

type Props = {
  stage: number
  outcome?: string
  // Rounds per stage number. A piece with rounds isn't split: it fills in
  // levels as they're done, so the mark stays as calm as the logo.
  interviews?: Counts
  size?: number
}

// How filled a piece reads for its rounds, in four steps rather than an exact
// share so the levels stay tellable apart at 20px.
export function roundLevel(done: number, total: number) {
  if (total === 0 || done >= total) return 1
  if (done === 0) return 0.3
  return done / total < 0.5 ? 0.5 : 0.75
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
      {PIECES.map((d, i) => {
        const piece = i + 1
        const tinted = piece === target
        const rounds = interviews?.[piece]
        const done = rounds?.done ?? 0
        // A piece shows once it's reached, or ahead of time if a round on it is done.
        const shown = n >= piece || tinted || done > 0
        const base = tinted ? tint : piece === 6 ? "stroke-brand-gold" : "stroke-foreground"
        return (
          <path
            key={piece}
            d={d}
            fill="none"
            strokeWidth={11}
            // Pieces fade in and out as the stage changes (the status chooser leans on this).
            className={`transition-[stroke,opacity] duration-300 ease-out ${shown ? base : "stroke-border"}`}
            opacity={shown ? roundLevel(done, rounds?.total ?? 0) : 1}
          />
        )
      })}
    </svg>
  )
}
