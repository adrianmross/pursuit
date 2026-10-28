import { FILTERS, MORE } from "@/lib/types"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

export const STATUS_OPTIONS = FILTERS.concat(MORE).filter(([value]) => value)
export const FIT_OPTIONS = ["strong", "reach", "maybe"]

export type AttributesFormState = {
  company: string
  title: string
  location: string
  salary: string
  fit: string
  posting_url: string
  status: string
}

export const EMPTY_ATTRIBUTES: AttributesFormState = {
  company: "",
  title: "",
  location: "",
  salary: "",
  fit: "",
  posting_url: "",
  status: "to_apply",
}

export function AttributesForm({
  value,
  onChange,
  showCompany,
}: {
  value: AttributesFormState
  onChange: (next: AttributesFormState) => void
  showCompany?: boolean
}) {
  function set<K extends keyof AttributesFormState>(key: K, v: AttributesFormState[K]) {
    onChange({ ...value, [key]: v })
  }

  return (
    <div className="grid grid-cols-2 gap-3">
      {showCompany && (
        <div className="col-span-2 flex flex-col gap-1.5">
          <Label>Company</Label>
          <Input value={value.company} onChange={(e) => set("company", e.target.value)} required />
        </div>
      )}
      <div className="col-span-2 flex flex-col gap-1.5">
        <Label>Title</Label>
        <Input value={value.title} onChange={(e) => set("title", e.target.value)} />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label>Location</Label>
        <Input value={value.location} onChange={(e) => set("location", e.target.value)} />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label>Salary</Label>
        <Input value={value.salary} onChange={(e) => set("salary", e.target.value)} />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label>Fit</Label>
        <Select value={value.fit || "__none"} onValueChange={(v) => set("fit", v === "__none" ? "" : v)}>
          <SelectTrigger className="w-full">
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
        <Select value={value.status} onValueChange={(v) => set("status", v)}>
          <SelectTrigger className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {STATUS_OPTIONS.map(([v, text]) => (
              <SelectItem key={v} value={v}>
                {text}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="col-span-2 flex flex-col gap-1.5">
        <Label>Posting URL</Label>
        <Input value={value.posting_url} onChange={(e) => set("posting_url", e.target.value)} />
      </div>
    </div>
  )
}
