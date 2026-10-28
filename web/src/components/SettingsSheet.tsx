import { useState } from "react"
import { FILTERS, MORE, SORTS } from "@/lib/types"
import { getDefaultSort, getDefaultStatus, setDefaultSort, setDefaultStatus } from "@/lib/settings"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"

const STATUSES = FILTERS.concat(MORE)

export function SettingsSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const [status, setStatus] = useState(getDefaultStatus())
  const [sort, setSort] = useState(getDefaultSort())

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Settings</SheetTitle>
          <SheetDescription>Defaults applied the next time you open the pipeline with no filters set.</SheetDescription>
        </SheetHeader>
        <div className="flex flex-col gap-4 px-4">
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
        </div>
      </SheetContent>
    </Sheet>
  )
}
