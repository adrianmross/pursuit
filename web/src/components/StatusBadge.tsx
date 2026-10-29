import { label, statusTone } from "@/lib/types"
import { Badge } from "@/components/ui/badge"

// Told apart by lightness and outline, not hue (docs/brand/README.md).
const TONE_CLASS: Record<string, string> = {
  default: "bg-muted text-foreground",
  attention: "bg-card text-foreground ring-1 ring-inset ring-foreground",
  strong: "bg-primary text-primary-foreground",
  danger: "bg-card text-destructive",
  muted: "bg-transparent text-muted-foreground ring-1 ring-inset ring-border",
  win: "bg-brand-gold text-[#141615]",
}

export function StatusBadge({ status }: { status: string }) {
  return (
    <Badge variant="secondary" className={TONE_CLASS[statusTone(status)]}>
      {label(status)}
    </Badge>
  )
}
