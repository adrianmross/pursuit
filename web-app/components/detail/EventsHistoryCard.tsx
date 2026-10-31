import { ChevronDown } from "lucide-react"
import { label } from "@/lib/types"
import type { Event } from "@/lib/types"
import { Card, CardContent, CardTitle } from "@/components/ui/card"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { Empty, EmptyHeader, EmptyTitle } from "@/components/ui/empty"
import { Item, ItemContent, ItemDescription, ItemGroup, ItemTitle } from "@/components/ui/item"

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
              <Empty className="p-4">
                <EmptyHeader>
                  <EmptyTitle className="font-normal text-muted-foreground">No history yet.</EmptyTitle>
                </EmptyHeader>
              </Empty>
            ) : (
              <ItemGroup className="gap-1">
                {events.map((e, idx) => (
                  <Item key={idx} size="sm" role="listitem" className="px-0 py-1.5">
                    <ItemContent>
                      <ItemTitle>{e.title || e.kind}</ItemTitle>
                      <ItemDescription>
                        {fmt(e.occurred_at)}
                        {e.actor?.name ? ` · ${e.actor.name}` : ""}
                      </ItemDescription>
                      {(e.from_status || e.to_status) && (
                        <ItemDescription className="text-foreground">
                          {label(e.from_status)} → {label(e.to_status)}
                        </ItemDescription>
                      )}
                      {e.body && <ItemDescription className="line-clamp-none">{e.body}</ItemDescription>}
                    </ItemContent>
                  </Item>
                ))}
              </ItemGroup>
            )}
          </CardContent>
        </CollapsibleContent>
      </Card>
    </Collapsible>
  )
}
