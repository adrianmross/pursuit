// The logo's p as a game piece: five ring segments plus a sixth, gold tail
// on an offer. Geometry from the Hire Pursuit design system's ProgressMark.
const SEGMENTS = [
  "M5.95 41.74 A20.5 20.5 0 0 1 16.44 27.87",
  "M19.06 26.71 A20.5 20.5 0 0 1 36.37 28.32",
  "M38.73 29.93 A20.5 20.5 0 0 1 46.49 45.50",
  "M46.36 48.36 A20.5 20.5 0 0 1 37.23 63.15",
  "M34.73 64.55 A20.5 20.5 0 0 1 17.34 64.58",
]
const TAIL = "M5.5 46 V84"

export function ProgressMark({ pieces, size = 32 }: { pieces: number; size?: number }) {
  const n = Math.max(0, Math.min(6, pieces))
  return (
    <svg
      className="inline-block shrink-0 align-middle"
      width={Math.round((size * 74) / 80)}
      height={size}
      viewBox="-11 9 74 80"
      role="img"
      aria-label={`${n} of 6 pieces`}
    >
      {SEGMENTS.map((d, i) => (
        <path key={i} d={d} fill="none" strokeWidth={11} className={i < n ? "stroke-foreground" : "stroke-border"} />
      ))}
      <path d={TAIL} fill="none" strokeWidth={11} className={n >= 6 ? "stroke-brand-gold" : "stroke-border"} />
    </svg>
  )
}
