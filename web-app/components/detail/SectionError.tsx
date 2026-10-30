"use client"

import { useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"

// Shown when one section's fetch fails or times out — the point of the
// whole per-section-Suspense design: this never blocks the rest of the
// page, and there's always a way out of it besides a full page refresh.
export function SectionError({ label, onRetry }: { label: string; onRetry?: () => void }) {
  const router = useRouter()
  return (
    <Card>
      <CardContent className="flex items-center justify-between py-4">
        <p className="text-sm text-muted-foreground">Couldn&apos;t load {label}.</p>
        <Button variant="outline" size="sm" onClick={onRetry ?? (() => router.refresh())}>
          Retry
        </Button>
      </CardContent>
    </Card>
  )
}
