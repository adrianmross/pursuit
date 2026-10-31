"use client"

import Link from "next/link"
import { RotateCcw } from "lucide-react"
import { SnakeEyes } from "@/components/Sketch"
import { Button, buttonVariants } from "@/components/ui/button"

// Catches a failure in the core application fetch itself (the one thing
// app/(app)/app/[id]/page.tsx awaits) — e.g. the exact FUNCTION_INVOCATION_TIMEOUT
// that originally motivated this rebuild. reset() re-runs the Server
// Component, retrying the fetch, instead of leaving a dead page.
export default function ApplicationError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <div className="mx-auto flex max-w-3xl flex-col items-center gap-3 py-16 text-center">
      <SnakeEyes className="w-28" roughness={2.5} />
      <h1 className="font-heading text-2xl font-semibold tracking-tight">Bad roll.</h1>
      <p className="text-sm text-muted-foreground">Couldn&apos;t load this application.</p>
      <div className="mt-2 flex gap-2">
        <Button size="sm" onClick={reset}>
          <RotateCcw /> Retry
        </Button>
        <Link href="/applications" className={buttonVariants({ size: "sm", variant: "outline" })}>
          Back to applications
        </Link>
      </div>
    </div>
  )
}
