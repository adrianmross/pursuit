import { ChevronDown } from "lucide-react"
import { label } from "@/lib/types"
import type { Event } from "@/lib/types"
import { Card, CardContent, CardTitle } from "@/components/ui/card"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"

function fmt(value?: string) {
  if (!value) return "—"
  const match = value.match(/^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})/)
  if (!match) return value
  const date = new Date(`${match[1]}T${match[2]}Z`)
  if (Number.isNaN(date.getTime())) return match[1]
  return date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" })
}

export function EventsHistoryCard({ events }: { events: Event[] }) {
  return (
    <Collapsible>
      <Card>
        <CollapsibleTrigger className="flex w-full items-center justify-between p-6 text-left">
          <CardTitle className="text-base">History ({events.length})</CardTitle>
          <ChevronDown className="size-4 text-muted-foreground" />
        </CollapsibleTrigger>
        <CollapsibleContent>
          <CardContent className="pt-0">
            {events.length === 0 ? (
              <p className="text-sm text-muted-foreground">No history yet.</p>
            ) : (
              <ol className="flex flex-col gap-3">
                {events.map((e, idx) => (
                  <li key={idx} className="text-sm">
                    <div className="font-medium">{e.title || e.kind}</div>
                    <div className="text-muted-foreground">
                      {fmt(e.occurred_at)}
                      {e.actor?.name ? ` · ${e.actor.name}` : ""}
                    </div>
                    {(e.from_status || e.to_status) && (
                      <div>
                        {label(e.from_status)} → {label(e.to_status)}
                      </div>
                    )}
                    {e.body && <div className="text-muted-foreground">{e.body}</div>}
                  </li>
                ))}
              </ol>
            )}
          </CardContent>
        </CollapsibleContent>
      </Card>
    </Collapsible>
  )
}
