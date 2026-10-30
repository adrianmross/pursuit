"use client"

import { useEffect, useState } from "react"
import { useTheme } from "next-themes"
import { FILTERS, MORE, SORTS, type Audit, type Page } from "@/lib/types"
import { getDefaultSort, getDefaultStatus, setDefaultSort, setDefaultStatus } from "@/lib/settings"
import { api } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { CustomFieldsManager } from "@/components/CustomFieldsManager"
import { Button } from "@/components/ui/button"
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

function DemoSection({ active }: { active: boolean }) {
  const { startDemo, isDemo } = useAuth()
  const [available, setAvailable] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!active) return
    api<{ demo_available: boolean }>("/v1/auth/providers")
      .then((result) => setAvailable(result.demo_available))
      .catch(() => {})
  }, [active])

  if (!available) return null

  async function onClick() {
    setSubmitting(true)
    setError(null)
    try {
      await startDemo()
    } catch {
      setError("Could not start the demo.")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <h3 className="text-sm font-medium">Tutorial</h3>
      {error && <p className="text-sm text-destructive">{error}</p>}
      <Button type="button" variant="outline" disabled={submitting} onClick={onClick}>
        {submitting ? "Starting…" : isDemo ? "Replay tutorial" : "Try the tutorial"}
      </Button>
      <p className="text-xs text-muted-foreground">
        {isDemo
          ? "Starts a brand-new sandbox, abandoning whatever you've changed in this one."
          : "Switches you into a throwaway sandbox pre-loaded with example data — sign back in to return to your account."}
      </p>
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
          <DemoSection active={open} />
          <AuditSection active={open} />
        </div>
      </SheetContent>
    </Sheet>
  )
}
