"use client"

import { useEffect, useRef } from "react"

// A short looping clip with play, pause and replay controls. It plays while at
// least half of it is on screen and pauses when it scrolls away, unless the
// viewer paused it themselves: then it stays paused until they press play.
// A capped width and a reserved height keep the page from jumping as it loads,
// and a poster shows until the first frame arrives.
// `src` is the path without an extension; the .webm, .mp4 and .png sit beside each other.
export function Clip({ src, width, height, caption }: { src: string; width: number; height: number; caption: string }) {
  const ref = useRef<HTMLVideoElement>(null)
  const userPaused = useRef(false)
  const visible = useRef(false)
  const byCode = useRef(false) // the next pause event is ours, not the viewer's

  useEffect(() => {
    const v = ref.current
    if (!v) return
    const io = new IntersectionObserver(
      ([entry]) => {
        visible.current = entry.isIntersecting
        if (entry.isIntersecting) {
          if (!userPaused.current) void v.play().catch(() => {})
        } else if (!v.paused) {
          byCode.current = true
          v.pause()
        }
      },
      { threshold: 0.5 },
    )
    io.observe(v)
    return () => io.disconnect()
  }, [])

  return (
    <figure style={{ maxWidth: Math.min(width, 720), margin: "1.5rem 0" }}>
      <video
        ref={ref}
        loop
        muted
        controls
        playsInline
        preload="metadata"
        poster={`${src}.png`}
        width={width}
        height={height}
        aria-label={caption}
        onPause={() => {
          if (byCode.current) byCode.current = false
          else if (visible.current) userPaused.current = true
        }}
        onPlay={() => {
          userPaused.current = false
        }}
        style={{ width: "100%", height: "auto", aspectRatio: `${width} / ${height}`, borderRadius: 8, border: "1px solid var(--line, rgba(128,128,128,.25))" }}
      >
        <source src={`${src}.webm`} type="video/webm" />
        <source src={`${src}.mp4`} type="video/mp4" />
      </video>
      <figcaption style={{ marginTop: 8, fontSize: 13, opacity: 0.7 }}>{caption}</figcaption>
    </figure>
  )
}

// Marks a feature that needs a plan on the hosted service.
export function Premium() {
  return <span className="badge-premium">Premium</span>
}
