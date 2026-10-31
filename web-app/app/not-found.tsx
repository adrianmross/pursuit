"use client"

import Link from "next/link"
import { ArrowLeft } from "lucide-react"
import { useAuth } from "@/lib/auth"
import { Die, ErrorScene, Four, Ring, Sketch, Track } from "@/components/Sketch"
import { buttonVariants } from "@/components/ui/button"

// Exported as 404.html; the Go server serves it, with a 404 status, for any
// path that has no file of its own (internal/dashboard).
export default function NotFound() {
  const { me } = useAuth()
  return (
    <ErrorScene
      graphic={<LostTrail />}
      title="You rolled a 404."
      actions={
        <>
          <Link href={me ? "/start" : "/"} className={buttonVariants()}>
            <ArrowLeft /> Roll again
          </Link>
          <a href="/docs/" className={buttonVariants({ variant: "outline" })}>
            Read the docs
          </a>
        </>
      }
    >
      That square isn&apos;t on the board. The page doesn&apos;t exist, or it moved. The pipeline lives at
      /applications now.
    </ErrorScene>
  )
}

// The mark's ring is the 0; its gold stem has fallen off onto a board track
// that fades out, next to a die that rolled a 4.
function LostTrail() {
  return (
    <Sketch viewBox="-8 0 350 186" className="w-full max-w-md">
      <Four x={10} tilt={-3} />
      <g transform="rotate(2 180 72)">
        <Ring cx={180} cy={72} />
      </g>
      <Four x={242} tilt={4} />
      <Track y={164} />
      <path d="M118 158 L198 146" stroke="var(--brand-gold)" strokeWidth="22" />
      <Die x={312} y={166} tilt={16} pips={4} />
    </Sketch>
  )
}
