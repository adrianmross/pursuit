"use client"

import { useEffect, useRef, useState } from "react"
import { ChevronDown, ChevronUp } from "lucide-react"
import { usePref } from "@adrianmross/appkit-ui"
import { cn } from "@adrianmross/appkit-ui"

const MIN = 1
const MAX = 9

// The pips of a die face, on a -1..1 grid.
const PIPS: Record<number, [number, number][]> = {
  1: [[0, 0]],
  2: [[-1, -1], [1, 1]],
  3: [[-1, -1], [0, 0], [1, 1]],
  4: [[-1, -1], [1, -1], [-1, 1], [1, 1]],
  5: [[-1, -1], [1, -1], [0, 0], [-1, 1], [1, 1]],
  6: [[-1, -1], [1, -1], [-1, 0], [1, 0], [-1, 1], [1, 1]],
}

// A short tick where the device can give one.
function tick() {
  try {
    navigator.vibrate?.(4)
  } catch {
    // Not every browser lets a page vibrate.
  }
}

// How many rounds a stage has: scroll it, click its ends, press the arrow
// keys or type a digit. Settings, Appearance picks a wheel or a die.
export function RoundCounter({ value, onChange, label, disabled }: { value: number; onChange: (next: number) => void; label: string; disabled?: boolean }) {
  const ref = useRef<HTMLDivElement>(null)
  const [dir, setDir] = useState<1 | -1>(1)
  const [turn, setTurn] = useState(0)
  const last = useRef(0)
  const dice = usePref("roundCounter") === "dice"

  const step = (d: 1 | -1) => {
    const next = Math.min(MAX, Math.max(MIN, value + d))
    if (next === value || disabled) return
    setDir(d)
    setTurn((t) => t + d)
    tick()
    onChange(next)
  }
  const set = (n: number) => {
    const next = Math.min(MAX, Math.max(MIN, n))
    if (next === value || disabled) return
    setDir(next > value ? 1 : -1)
    setTurn((t) => t + (next > value ? 1 : -1))
    tick()
    onChange(next)
  }

  // The wheel needs a non-passive listener to keep the page from scrolling.
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      const now = performance.now()
      if (now - last.current < 140 || Math.abs(e.deltaY) < 4) return
      last.current = now
      step(e.deltaY < 0 ? 1 : -1)
    }
    el.addEventListener("wheel", onWheel, { passive: false })
    return () => el.removeEventListener("wheel", onWheel)
  })

  // Press and drag up or down to dial a number in; it lands when you let go.
  // A press that does not move is a click.
  const [preview, setPreview] = useState<number | null>(null)
  const drag = useRef<{ y: number; from: number; moved: boolean } | null>(null)
  const dragged = useRef(false)
  const shown = preview ?? value
  const pointer = {
    onPointerDown: (e: React.PointerEvent<HTMLDivElement>) => {
      if (disabled || e.button !== 0) return
      e.currentTarget.setPointerCapture(e.pointerId)
      drag.current = { y: e.clientY, from: value, moved: false }
    },
    onPointerMove: (e: React.PointerEvent<HTMLDivElement>) => {
      const d = drag.current
      if (!d) return
      const dy = d.y - e.clientY
      if (Math.abs(dy) > 4) d.moved = true
      if (!d.moved) return
      const n = Math.min(MAX, Math.max(MIN, d.from + Math.round(dy / 14)))
      const at = preview ?? value
      if (n === at) return
      setDir(n > at ? 1 : -1)
      setTurn((t) => t + (n > at ? 1 : -1))
      tick()
      setPreview(n)
    },
    onPointerUp: () => {
      const d = drag.current
      drag.current = null
      if (!d?.moved) return
      dragged.current = true
      if (preview !== null && preview !== value) onChange(preview)
      setPreview(null)
    },
  }
  const click = (fn: () => void) => () => {
    if (dragged.current) {
      dragged.current = false
      return
    }
    fn()
  }

  const keys = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowUp" || e.key === "ArrowRight" || e.key === "+") step(1)
    else if (e.key === "ArrowDown" || e.key === "ArrowLeft" || e.key === "-") step(-1)
    else if (/^[1-9]$/.test(e.key)) set(Number(e.key))
    else return
    e.preventDefault()
  }

  if (dice)
    return (
      <div
        ref={ref}
        role="spinbutton"
        tabIndex={disabled ? -1 : 0}
        aria-label={label}
        aria-valuenow={value}
        aria-valuemin={MIN}
        aria-valuemax={MAX}
        title="Click to roll up, right-click to roll down, drag, scroll or type a number"
        onKeyDown={keys}
        {...pointer}
        onClick={click(() => step(1))}
        onContextMenu={(e) => {
          e.preventDefault()
          step(-1)
        }}
        className={cn("flex size-7 shrink-0 cursor-pointer touch-none items-center justify-center rounded-md outline-none select-none focus-visible:ring-2 focus-visible:ring-ring", disabled && "pointer-events-none opacity-40")}
      >
        <svg viewBox="0 0 24 24" aria-hidden className="size-6 transition-transform duration-300 ease-[cubic-bezier(.3,1.6,.5,1)] motion-reduce:transition-none" style={{ transform: `rotate(${turn * 90}deg)` }}>
          <rect x="3" y="3" width="18" height="18" rx="4" className="fill-background" stroke="currentColor" strokeWidth="1.5" />
          {shown <= 6 ? (
            PIPS[shown].map(([x, y]) => <circle key={`${x},${y}`} cx={12 + x * 4.5} cy={12 + y * 4.5} r="1.6" fill="currentColor" />)
          ) : (
            <text x="12" y="16" textAnchor="middle" className="fill-current font-mono text-[11px] font-semibold">
              {shown}
            </text>
          )}
        </svg>
      </div>
    )

  return (
    <div
      ref={ref}
      role="spinbutton"
      tabIndex={disabled ? -1 : 0}
      aria-label={label}
      aria-valuenow={value}
      aria-valuemin={MIN}
      aria-valuemax={MAX}
      title="Drag, scroll, click the top or bottom half, or type a number"
      onKeyDown={keys}
      {...pointer}
      onClick={(e) => {
        const box = e.currentTarget.getBoundingClientRect()
        click(() => step(e.clientY < box.top + box.height / 2 ? 1 : -1))()
      }}
      className={cn(
        "group/wheel relative flex h-7 w-7 shrink-0 cursor-ns-resize touch-none items-center justify-center overflow-hidden rounded-md border bg-background outline-none select-none focus-visible:ring-2 focus-visible:ring-ring",
        // Ridges top and bottom, like a thumbwheel.
        "before:absolute before:inset-x-0 before:top-0 before:h-1.5 before:bg-[repeating-linear-gradient(90deg,var(--border)_0_1px,transparent_1px_3px)] after:absolute after:inset-x-0 after:bottom-0 after:h-1.5 after:bg-[repeating-linear-gradient(90deg,var(--border)_0_1px,transparent_1px_3px)]",
        disabled && "pointer-events-none opacity-40",
      )}
    >
      <ChevronUp aria-hidden className="absolute top-0 size-2.5 text-muted-foreground opacity-0 group-hover/wheel:opacity-100" />
      <span key={shown} className={cn("font-mono text-xs leading-none font-semibold tabular-nums animate-in fade-in duration-200", dir > 0 ? "slide-in-from-bottom-2" : "slide-in-from-top-2")}>
        {shown}
      </span>
      <ChevronDown aria-hidden className="absolute bottom-0 size-2.5 text-muted-foreground opacity-0 group-hover/wheel:opacity-100" />
    </div>
  )
}
