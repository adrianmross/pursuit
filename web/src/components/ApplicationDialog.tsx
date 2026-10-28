import { useEffect, useState } from "react"
import { api, ApiError } from "@/lib/api"
import { FILTERS, MORE, type Application } from "@/lib/types"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"

const STATUS_OPTIONS = FILTERS.concat(MORE).filter(([value]) => value)
const FIT_OPTIONS = ["strong", "reach", "maybe"]

type FormState = {
  company: string
  title: string
  location: string
  salary: string
  fit: string
  posting_url: string
  status: string
  notes: string
}

const EMPTY: FormState = { company: "", title: "", location: "", salary: "", fit: "", posting_url: "", status: "to_apply", notes: "" }

export function ApplicationDialog({
  open,
  onOpenChange,
  application,
  onSaved,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  application?: Application | null
  onSaved: (application: Application) => void
}) {
  const editing = !!application
  const [form, setForm] = useState<FormState>(EMPTY)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) return
    setError(null)
    setForm(
      application
        ? {
            company: application.company || "",
            title: application.title || "",
            location: application.location || "",
            salary: application.salary || "",
            fit: application.fit || "",
            posting_url: application.posting_url || "",
            status: application.status,
            notes: application.notes || "",
          }
        : EMPTY,
    )
  }, [open, application])

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  async function save() {
    setBusy(true)
    setError(null)
    try {
      const saved = editing
        ? await api<Application>(`/v1/applications/${encodeURIComponent(application!.id)}`, {
            method: "PATCH",
            body: {
              version: application!.version,
              title: form.title,
              location: form.location,
              salary: form.salary,
              fit: form.fit,
              posting_url: form.posting_url,
              status: form.status,
              notes: form.notes,
            },
          })
        : await api<Application>("/v1/applications", {
            method: "POST",
            body: {
              company: form.company,
              title: form.title,
              location: form.location,
              salary: form.salary,
              fit: form.fit,
              posting_url: form.posting_url,
              status: form.status,
              notes: form.notes,
            },
          })
      onSaved(saved)
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editing ? "Edit application" : "Add application"}</DialogTitle>
          <DialogDescription>
            {editing ? "Update the details Pursuit has on file for this application." : "Add an application you're tracking by hand."}
          </DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-2 gap-3">
          {!editing && (
            <div className="col-span-2 flex flex-col gap-1.5">
              <Label>Company</Label>
              <Input value={form.company} onChange={(e) => set("company", e.target.value)} required />
            </div>
          )}
          <div className="col-span-2 flex flex-col gap-1.5">
            <Label>Title</Label>
            <Input value={form.title} onChange={(e) => set("title", e.target.value)} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Location</Label>
            <Input value={form.location} onChange={(e) => set("location", e.target.value)} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Salary</Label>
            <Input value={form.salary} onChange={(e) => set("salary", e.target.value)} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Fit</Label>
            <Select value={form.fit || "__none"} onValueChange={(v) => set("fit", v === "__none" ? "" : v)}>
              <SelectTrigger>
                <SelectValue placeholder="—" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none">—</SelectItem>
                {FIT_OPTIONS.map((f) => (
                  <SelectItem key={f} value={f}>
                    {f}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Status</Label>
            <Select value={form.status} onValueChange={(v) => set("status", v)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {STATUS_OPTIONS.map(([value, text]) => (
                  <SelectItem key={value} value={value}>
                    {text}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="col-span-2 flex flex-col gap-1.5">
            <Label>Posting URL</Label>
            <Input value={form.posting_url} onChange={(e) => set("posting_url", e.target.value)} />
          </div>
          <div className="col-span-2 flex flex-col gap-1.5">
            <Label>Notes</Label>
            <Textarea value={form.notes} onChange={(e) => set("notes", e.target.value)} />
          </div>
        </div>
        {error && <p className="text-sm text-destructive">{error}</p>}
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={save} disabled={busy || (!editing && !form.company.trim())}>
            {editing ? "Save" : "Add"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
