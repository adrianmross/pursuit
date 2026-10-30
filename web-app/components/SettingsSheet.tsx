"use client"

import { useEffect, useState } from "react"
import { useTheme } from "next-themes"
import { FILTERS, MORE, SORTS, type Audit, type Page } from "@/lib/types"
import { getDefaultSort, getDefaultStatus, setDefaultSort, setDefaultStatus } from "@/lib/settings"
import { api } from "@/lib/api-client"
import { CustomFieldsManager } from "@/components/CustomFieldsManager"
import { Label } from "@/components/ui/label"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"

const STATUSES = FILTERS.concat(MORE)

function AuditSection({ active }: { active: boolean }) {
  const [entries, setEntries] = useState<Audit[] | null>(null)

  useEffect(() => {
    if (!active) return
    api<Page<Audit>>("/v1/audit?limit=50")
      .then((page) => setEntries(page.data || []))
      .catch(() => setEntries([]))
  }, [active])

  return (
    <div className="flex flex-col gap-2">
      <h3 className="text-sm font-medium">Audit</h3>
      <div className="flex max-h-64 flex-col gap-1.5 overflow-y-auto rounded-md border p-2 text-sm">
        {entries == null && <p className="text-muted-foreground">Loading…</p>}
        {entries?.length === 0 && <p className="text-muted-foreground">No activity yet.</p>}
        {entries?.map((entry) => (
          <div key={entry.id} className="flex flex-col gap-0.5 border-b pb-1.5 last:border-b-0 last:pb-0">
            <span>{entry.summary || `${entry.action} ${entry.entity_type}`}</span>
            <span className="text-xs text-muted-foreground">
              {entry.actor_name || "System"} · {new Date(entry.created_at).toLocaleString()}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

export function SettingsSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { theme, setTheme } = useTheme()
  const [status, setStatus] = useState(getDefaultStatus())
  const [sort, setSort] = useState(getDefaultSort())

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Settings</SheetTitle>
          <SheetDescription>Appearance, table defaults, custom fields, and recent activity.</SheetDescription>
        </SheetHeader>
        <div className="flex flex-col gap-6 px-4">
          <div className="flex flex-col gap-2">
            <h3 className="text-sm font-medium">Appearance</h3>
            <ToggleGroup
              type="single"
              variant="outline"
              value={theme}
              onValueChange={(value) => value && setTheme(value)}
            >
              <ToggleGroupItem value="light" className="flex-1">
                Light
              </ToggleGroupItem>
              <ToggleGroupItem value="dark" className="flex-1">
                Dark
              </ToggleGroupItem>
              <ToggleGroupItem value="system" className="flex-1">
                System
              </ToggleGroupItem>
            </ToggleGroup>
          </div>
          <div className="flex flex-col gap-3">
            <h3 className="text-sm font-medium">Table</h3>
            <div className="flex flex-col gap-2">
              <Label>Default status filter</Label>
              <Select
                value={status || "__all"}
                onValueChange={(value) => {
                  const next = value === "__all" ? "" : value
                  setStatus(next)
                  setDefaultStatus(next)
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__all">All</SelectItem>
                  {STATUSES.filter(([value]) => value).map(([value, text]) => (
                    <SelectItem key={value} value={value}>
                      {text}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label>Default sort</Label>
              <Select
                value={sort}
                onValueChange={(value) => {
                  setSort(value)
                  setDefaultSort(value)
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SORTS.map(([value, text]) => (
                    <SelectItem key={value} value={value}>
                      {text}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <CustomFieldsManager active={open} />
          </div>
          <AuditSection active={open} />
        </div>
      </SheetContent>
    </Sheet>
  )
}
