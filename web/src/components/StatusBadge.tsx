import { label, statusCategory, statusTone } from "@/lib/types"
import { Badge } from "@/components/ui/badge"

const CATEGORY_CLASS: Record<string, string> = {
  action: "bg-amber-500/15 text-amber-700 dark:bg-amber-500/20 dark:text-amber-400",
  rejected: "bg-rose-500/10 text-rose-600 dark:bg-rose-500/15 dark:text-rose-400",
  closed: "bg-muted text-muted-foreground",
}

export function StatusBadge({ status }: { status: string }) {
  const className =
    statusTone(status) === "win" ? "bg-brand-gold text-[#141615]" : CATEGORY_CLASS[statusCategory(status)]
  return (
    <Badge variant="secondary" className={className}>
      {label(status)}
    </Badge>
  )
}
