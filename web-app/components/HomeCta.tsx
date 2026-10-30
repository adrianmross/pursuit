import Link from "next/link"
import { ArrowRight, Plus } from "lucide-react"
import { Button } from "@/components/ui/button"

// Pure presentation — hasApplications is resolved by the caller (server-side
// on hosted, client-side in export mode; see app/(app)/page.tsx).
export function HomeCta({ hasApplications }: { hasApplications: boolean }) {
  return (
    <div className="flex w-64 flex-col items-stretch gap-2">
      {hasApplications && (
        <Button size="lg" className="w-full" asChild>
          <Link href="/pipeline">
            View applications <ArrowRight className="size-4" />
          </Link>
        </Button>
      )}
      <Button size="lg" className="w-full" variant={hasApplications ? "secondary" : "default"} asChild>
        <Link href="/app/new">
          <Plus className="size-4" /> Start a new application
        </Link>
      </Button>
    </div>
  )
}
