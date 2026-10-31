import { label, outcomeKind, statusCategory, statusTone } from "@/lib/types"
import { Badge } from "@/components/ui/badge"

const CATEGORY_CLASS: Record<string, string> = {
  action: "bg-amber-500/15 text-amber-700 dark:bg-amber-500/20 dark:text-amber-400",
  closed: "bg-muted text-muted-foreground",
}

// Outcomes read the same colour as the segment they turn on the progress mark.
const OUTCOME_CLASS = {
  hold: CATEGORY_CLASS.action,
  stop: "bg-destructive/10 text-destructive dark:bg-destructive/15",
}

export function StatusBadge({ status }: { status: string }) {
  const kind = outcomeKind(status)
  const className =
    statusTone(status) === "win"
      ? "bg-brand-gold text-[#141615]"
      : kind
        ? OUTCOME_CLASS[kind]
        : CATEGORY_CLASS[statusCategory(status)]
  return (
    <Badge variant="secondary" className={className}>
      {label(status)}
    </Badge>
  )
}
