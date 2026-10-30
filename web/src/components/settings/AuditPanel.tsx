import { useEffect, useMemo, useState } from "react"
import { api } from "@/lib/api"
import type { Audit, Page } from "@/lib/types"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

export function AuditPanel() {
  const [entries, setEntries] = useState<Audit[] | null>(null)
  const [cursor, setCursor] = useState<string | null>(null)
  const [filter, setFilter] = useState("")

  async function load(after?: string) {
    const page = await api<Page<Audit>>("/v1/audit?limit=100" + (after ? "&cursor=" + encodeURIComponent(after) : ""))
    setEntries((prev) => (after ? [...(prev || []), ...(page.data || [])] : page.data || []))
    setCursor(page.page?.next_cursor || null)
  }

  useEffect(() => {
    load().catch(() => setEntries([]))
  }, [])

  const shown = useMemo(() => {
    const q = filter.trim().toLowerCase()
    if (!q) return entries
    return entries?.filter((e) => [e.summary, e.action, e.entity_type, e.actor_name].join(" ").toLowerCase().includes(q))
  }, [entries, filter])

  return (
    <div className="flex flex-col gap-4">
      <Input
        data-search="audit"
        aria-label="Search audit log"
        placeholder="Search actor, action, entity, or summary"
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
      />
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>When</TableHead>
              <TableHead>Actor</TableHead>
              <TableHead>Action</TableHead>
              <TableHead>Entity</TableHead>
              <TableHead className="w-full">Summary</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {shown == null && (
              <TableRow>
                <TableCell colSpan={5} className="text-muted-foreground">Loading…</TableCell>
              </TableRow>
            )}
            {shown?.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-muted-foreground">No activity.</TableCell>
              </TableRow>
            )}
            {shown?.map((e) => (
              <TableRow key={e.id}>
                <TableCell className="whitespace-nowrap text-muted-foreground">{new Date(e.created_at).toLocaleString()}</TableCell>
                <TableCell className="whitespace-nowrap">{e.actor_name || "System"}</TableCell>
                <TableCell className="whitespace-nowrap font-mono text-xs">{e.action}</TableCell>
                <TableCell className="whitespace-nowrap">{e.entity_type}</TableCell>
                <TableCell>{e.summary}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {filter && cursor && <p className="text-center text-xs text-muted-foreground">Searching loaded entries only. Load more to widen it.</p>}
      {cursor && (
        <Button variant="outline" className="self-center" onClick={() => load(cursor)}>
          Load more
        </Button>
      )}
    </div>
  )
}
