import { SiteFooter } from "@/components/SiteFooter"

// Hand-drawn board-game pieces for the error states (404, crash, failed
// sign-in). Everything is plain strokes in currentColor plus the brand gold
// and forest, roughened by one displacement filter. Every <Sketch> defines
// the same filter under the same id, so repeats on a page are harmless.

export function Sketch({
  viewBox,
  roughness = 5,
  className,
  children,
}: {
  viewBox: string
  // Displacement in viewBox units: keep it near 5 at the 404's scale and
  // lower for small viewBoxes, or the strokes melt.
  roughness?: number
  className?: string
  children: React.ReactNode
}) {
  const id = `sketch-${roughness}`
  return (
    <svg viewBox={viewBox} fill="none" aria-hidden className={className}>
      <defs>
        <filter id={id} x="-10%" y="-10%" width="120%" height="120%">
          <feTurbulence type="fractalNoise" baseFrequency="0.04" numOctaves="2" seed="7" />
          <feDisplacementMap in="SourceGraphic" scale={roughness} />
        </filter>
      </defs>
      <g filter={`url(#${id})`}>{children}</g>
    </svg>
  )
}

// The mark's ring: five segments of a circle, open at the lower left where
// the stem joins (angles measured off hire-pursuit-mark.svg).
const RING_SEGMENTS = [
  [-168, -117.8],
  [-109.8, -59.6],
  [-51.6, -1.4],
  [6.6, 56.8],
  [64.8, 115],
]

export function Ring({ cx, cy, r = 45, stem = false }: { cx: number; cy: number; r?: number; stem?: boolean }) {
  const at = (deg: number) => {
    const rad = (deg * Math.PI) / 180
    return `${(cx + r * Math.cos(rad)).toFixed(2)} ${(cy + r * Math.sin(rad)).toFixed(2)}`
  }
  const width = (r * 24) / 45
  return (
    <g strokeWidth={width}>
      {RING_SEGMENTS.map(([a, b]) => (
        <path key={a} d={`M${at(a)} A${r} ${r} 0 0 1 ${at(b)}`} stroke="currentColor" />
      ))}
      {stem && <path d={`M${cx - r} ${cy} V${cy + (r * 38) / 20.5}`} stroke="var(--brand-gold)" />}
    </g>
  )
}

export function Four({ x, tilt = 0 }: { x: number; tilt?: number }) {
  return (
    <g transform={`rotate(${tilt} ${x + 44} 72)`} stroke="currentColor" strokeWidth="24">
      <path d={`M${x + 68} 15 V129`} />
      <path d={`M${x} 95 H${x + 52}`} />
      <path d={`M${x + 42} 24 L${x + 6} 79`} />
    </g>
  )
}

const PIPS: Record<number, [number, number][]> = {
  0: [],
  1: [[0, 0]],
  2: [[-1, -1], [1, 1]],
  3: [[-1, -1], [0, 0], [1, 1]],
  4: [[-1, -1], [1, -1], [-1, 1], [1, 1]],
  5: [[-1, -1], [1, -1], [0, 0], [-1, 1], [1, 1]],
  6: [[-1, -1], [1, -1], [-1, 0], [1, 0], [-1, 1], [1, 1]],
}

export function Die({ x, y, size = 26, tilt = 0, pips }: { x: number; y: number; size?: number; tilt?: number; pips: number }) {
  const h = size / 2
  return (
    <g transform={`translate(${x} ${y}) rotate(${tilt})`}>
      <rect x={-h} y={-h} width={size} height={size} rx={size / 5} className="fill-background" stroke="currentColor" strokeWidth={size / 9} />
      {PIPS[pips].map(([px, py]) => (
        <circle key={`${px},${py}`} cx={px * size * 0.24} cy={py * size * 0.24} r={size / 10} fill="currentColor" />
      ))}
    </g>
  )
}

// A row of board squares that fades out to the right. `broken` squares are
// left out, the one either side of the break knocked askew.
export function Track({ y, count = 15, broken = [] }: { y: number; count?: number; broken?: number[] }) {
  return (
    <g>
      {Array.from({ length: count }, (_, i) => {
        if (broken.includes(i)) return null
        const x = 2 + i * 20
        const askew = broken.includes(i - 1) ? 14 : broken.includes(i + 1) ? -12 : 0
        const fill = i % 5 === 2 ? "var(--brand-gold)" : i % 5 === 4 ? "var(--brand-forest)" : "none"
        return (
          <rect
            key={i}
            x={x}
            y={y}
            width="14"
            height="14"
            rx="2"
            fill={fill}
            stroke="currentColor"
            strokeWidth="2.5"
            opacity={Math.max(0, 1 - i / (count - 2))}
            transform={askew ? `rotate(${askew} ${x + 7} ${y + 7})` : undefined}
          />
        )
      })}
    </g>
  )
}

// Two dice showing ones: the universal bad roll.
export function SnakeEyes({ className, roughness = 2 }: { className?: string; roughness?: number }) {
  return (
    <Sketch viewBox="0 0 64 40" roughness={roughness} className={className}>
      <Die x={18} y={21} size={22} tilt={-14} pips={1} />
      <Die x={45} y={18} size={22} tilt={11} pips={1} />
    </Sketch>
  )
}

// Every outcome is a die whose face is its number key in the status chooser
// (1 waiting ... 6 withdrawn), so the die doubles as the shortcut hint.
// Withdrawn's die lies tipped over.
const OUTCOME_DICE: Record<string, { pips: number; tilt: number }> = {
  waiting: { pips: 1, tilt: -8 },
  needs_input: { pips: 2, tilt: 6 },
  failed: { pips: 3, tilt: -4 },
  rejected: { pips: 4, tilt: -14 },
  ghosted: { pips: 5, tilt: 10 },
  withdrawn: { pips: 6, tilt: 90 },
}

export function OutcomePiece({ status, className }: { status: string; className?: string }) {
  const die = OUTCOME_DICE[status] ?? { pips: 1, tilt: 0 }
  return (
    <Sketch viewBox="-16 -16 32 32" roughness={2} className={className}>
      <Die x={0} y={0} size={22} tilt={die.tilt} pips={die.pips} />
    </Sketch>
  )
}

// Small marks for whose move a hold is: an hourglass for waiting on them, a
// card turned up with a question for needs input.
export function HoldIcon({ status, className }: { status: string; className?: string }) {
  return (
    <Sketch viewBox="-14 -14 28 28" roughness={1.5} className={className}>
      {status === "waiting" ? (
        <g stroke="currentColor" strokeWidth="2.6">
          <path d="M-9 -12 H9 M-9 12 H9" />
          <path d="M-7 -12 C-7 -4 0 -2 0 0 C0 2 -7 4 -7 12 M7 -12 C7 -4 0 -2 0 0 C0 2 7 4 7 12" />
          <path d="M-4 9 Q0 4 4 9 Z" fill="currentColor" strokeWidth="1.2" />
        </g>
      ) : (
        <g transform="rotate(8)">
          <rect x="-8.5" y="-12" width="17" height="24" rx="3" stroke="currentColor" strokeWidth="2.6" />
          <path d="M-3.2 -3.5 A3.4 3.4 0 1 1 1 -0.2 C0 0.4 0 1.2 0 3" stroke="currentColor" strokeWidth="2.4" />
          <circle cx="0" cy="7" r="1.6" fill="currentColor" />
        </g>
      )}
    </Sketch>
  )
}

// Full-page layout shared by the 404 and crash pages: graphic, heading,
// copy, actions, footer.
export function ErrorScene({
  graphic,
  title,
  children,
  actions,
}: {
  graphic: React.ReactNode
  title: string
  children: React.ReactNode
  actions: React.ReactNode
}) {
  return (
    <div className="flex min-h-svh flex-col bg-background text-foreground">
      <main className="flex flex-1 flex-col items-center justify-center gap-4 px-4 py-16 text-center">
        {graphic}
        <h1 className="mt-4 font-heading text-4xl font-semibold tracking-tight text-balance sm:text-5xl">{title}</h1>
        <p className="max-w-md text-balance text-muted-foreground">{children}</p>
        <div className="mt-4 flex flex-wrap justify-center gap-3">{actions}</div>
      </main>
      <SiteFooter />
    </div>
  )
}
