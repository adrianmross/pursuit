"use client"

import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"

// Catches a failure in the core application fetch itself (the one thing
// app/(app)/app/[id]/page.tsx awaits) — e.g. the exact FUNCTION_INVOCATION_TIMEOUT
// that originally motivated this rebuild. reset() re-runs the Server
// Component, retrying the fetch, instead of leaving a dead page.
export default function ApplicationError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-4">
      <Card>
        <CardContent className="flex items-center justify-between py-6">
          <p className="text-sm text-muted-foreground">Couldn&apos;t load this application.</p>
          <Button variant="outline" size="sm" onClick={reset}>
            Retry
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
