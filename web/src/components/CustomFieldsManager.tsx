import { useEffect, useState } from "react"
import { Plus, Trash2 } from "lucide-react"
import { api } from "@/lib/api"
import type { CustomField, Page } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

const TYPES: [CustomField["type"], string][] = [
  ["text", "Text"],
  ["number", "Number"],
  ["date", "Date"],
]

export function CustomFieldsManager({ active }: { active: boolean }) {
  const [fields, setFields] = useState<CustomField[] | null>(null)
  const [name, setName] = useState("")
  const [type, setType] = useState<CustomField["type"]>("text")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function load() {
    const page = await api<Page<CustomField>>("/v1/fields?limit=100")
    setFields(page.data || [])
  }

  useEffect(() => {
    if (active) void load()
  }, [active])

  async function create() {
    if (!name.trim()) return
    setBusy(true)
    setError(null)
    try {
      await api("/v1/fields", { method: "POST", body: { name: name.trim(), type } })
      setName("")
      await load()
    } catch {
      setError("Failed to create field")
    } finally {
      setBusy(false)
    }
  }

  async function remove(id: string) {
    setBusy(true)
    try {
      await api(`/v1/fields/${encodeURIComponent(id)}`, { method: "DELETE" })
      await load()
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <Label>Custom fields</Label>
      {fields == null ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : fields.length === 0 ? (
        <p className="text-sm text-muted-foreground">None yet — add one below.</p>
      ) : (
        <div className="flex flex-col gap-1">
          {fields.map((f) => (
            <div key={f.id} className="flex items-center justify-between rounded-md border px-2 py-1.5 text-sm">
              <span>
                {f.name} <span className="text-muted-foreground">({f.type})</span>
              </span>
              <Button variant="ghost" size="icon" className="size-6" disabled={busy} onClick={() => remove(f.id)}>
                <Trash2 className="size-3.5" />
              </Button>
            </div>
          ))}
        </div>
      )}
      <div className="flex gap-2">
        <Input placeholder="Field name" value={name} onChange={(e) => setName(e.target.value)} className="flex-1" />
        <Select value={type} onValueChange={(v) => setType(v as CustomField["type"])}>
          <SelectTrigger className="w-28">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {TYPES.map(([value, text]) => (
              <SelectItem key={value} value={value}>
                {text}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button size="icon" disabled={busy || !name.trim()} onClick={create} aria-label="Add field">
          <Plus />
        </Button>
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}
    </div>
  )
}
