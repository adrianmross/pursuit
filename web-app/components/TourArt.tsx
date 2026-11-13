import { useId } from "react"

// A trail of three stops to a flag, built in order: each stop pops, the dotted line grows to the next,
// then the pole grows and the flag unfurls last. The dots are one fixed pattern revealed by a growing
// mask, so they never shift while it draws. Reduced motion shows it finished.
const SEGMENTS = ["M16 92 C60 92 70 40 112 52", "M112 52 C154 64 170 100 208 62", "M208 62 C246 24 262 22 292 30"]
const STOPS = [[16, 92], [112, 52], [208, 62]]
const STEP = 0.5 // seconds from one stop to the next

export function TrailArt() {
  const id = useId().replace(/:/g, "")
  const at = (s: number) => ({ animationDelay: `${s}s` })
  return (
    <svg viewBox="0 0 320 120" className="h-28 w-full text-primary" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" aria-hidden>
      <defs>
        {SEGMENTS.map((d, n) => (
          <mask key={d} id={`${id}-${n}`} maskUnits="userSpaceOnUse" x="0" y="0" width="320" height="120">
            <path d={d} pathLength="1" stroke="white" strokeWidth="10" strokeDasharray="1" className="motion-safe:animate-[tour-line_0.4s_ease-in-out_both]" style={at(0.3 + n * STEP)} />
          </mask>
        ))}
      </defs>
      {SEGMENTS.map((d, n) => (
        <path key={d} d={d} strokeDasharray="0.1 7" strokeWidth="3.5" className="opacity-70" mask={`url(#${id}-${n})`} />
      ))}
      {STOPS.map(([x, y], n) => (
        <circle key={n} cx={x} cy={y} r="6" className="fill-background [transform-box:fill-box] [transform-origin:center] motion-safe:animate-[tour-pop_0.35s_ease-out_both]" style={at(n * STEP)} />
      ))}
      <rect x="291" y="6" width="2" height="24" rx="1" className="fill-current [transform-box:fill-box] [transform-origin:50%_100%] motion-safe:animate-[tour-grow_0.3s_ease-out_both]" style={at(0.3 + 3 * STEP)} />
      <path d="M293 6 L313 13 L293 20 Z" strokeWidth="1" className="fill-current [transform-box:fill-box] [transform-origin:0_50%] motion-safe:animate-[tour-unfurl_0.4s_ease-out_both]" style={at(0.55 + 3 * STEP)} />
    </svg>
  )
}
