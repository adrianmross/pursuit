"use client"

import Link from "next/link"
import { RotateCcw } from "lucide-react"
import { useAuth } from "@/lib/auth"
import { Die, ErrorScene, Ring, Sketch, Track } from "@/components/Sketch"
import { Button, buttonVariants } from "@/components/ui/button"

// Any render or fetch failure below the root layout lands here. reset()
// re-renders the segment (re-running its fetch) instead of leaving a dead page.
export default function AppError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const { me } = useAuth()
  return (
    <ErrorScene
      graphic={<KnockedOver />}
      title="Lose a turn."
      actions={
        <>
          <Button onClick={reset}>
            <RotateCcw /> Try again
          </Button>
          <Link href={me ? "/start" : "/"} className={buttonVariants({ variant: "outline" })}>
            {me ? "Back to start" : "Back home"}
          </Link>
        </>
      }
    >
      Something went wrong loading this page. Trying again usually does it.
    </ErrorScene>
  )
}

// The whole mark, knocked flat across a broken stretch of board, beside a
// roll of snake eyes.
function KnockedOver() {
  return (
    <Sketch viewBox="-8 36 350 150" className="w-full max-w-md">
      <Track y={166} broken={[7, 8, 9]} />
      <g transform="rotate(-90 92 108)">
        <Ring cx={92} cy={108} stem />
      </g>
      <Die x={258} y={130} size={40} tilt={-18} pips={1} />
      <Die x={306} y={106} size={40} tilt={14} pips={1} />
    </Sketch>
  )
}
