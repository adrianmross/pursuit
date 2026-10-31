"use client"

import { useId } from "react"
import { FILTERS, MORE } from "@/lib/types"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

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
  const id = useId()
  function set<K extends keyof AttributesFormState>(key: K, v: AttributesFormState[K]) {
    onChange({ ...value, [key]: v })
  }

  return (
    <FieldGroup className="grid grid-cols-2 gap-4">
      {showCompany && (
        <Field className="col-span-2">
          <FieldLabel htmlFor={`${id}-company`}>Company</FieldLabel>
          <Input id={`${id}-company`} value={value.company} onChange={(e) => set("company", e.target.value)} required />
        </Field>
      )}
      <Field className="col-span-2">
        <FieldLabel htmlFor={`${id}-title`}>Title</FieldLabel>
        <Input id={`${id}-title`} value={value.title} onChange={(e) => set("title", e.target.value)} />
      </Field>
      <Field>
        <FieldLabel htmlFor={`${id}-location`}>Location</FieldLabel>
        <Input id={`${id}-location`} value={value.location} onChange={(e) => set("location", e.target.value)} />
      </Field>
      <Field>
        <FieldLabel htmlFor={`${id}-salary`}>Salary</FieldLabel>
        <Input id={`${id}-salary`} value={value.salary} onChange={(e) => set("salary", e.target.value)} />
      </Field>
      <Field>
        <FieldLabel htmlFor={`${id}-fit`}>Fit</FieldLabel>
        <Select value={value.fit || "__none"} onValueChange={(v) => set("fit", v === "__none" ? "" : v)}>
          <SelectTrigger id={`${id}-fit`} className="w-full">
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
      </Field>
      <Field>
        <FieldLabel htmlFor={`${id}-status`}>Status</FieldLabel>
        <Select value={value.status} onValueChange={(v) => set("status", v)}>
          <SelectTrigger id={`${id}-status`} className="w-full">
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
      </Field>
      <Field className="col-span-2">
        <FieldLabel htmlFor={`${id}-posting`}>Posting URL</FieldLabel>
        <Input id={`${id}-posting`} value={value.posting_url} onChange={(e) => set("posting_url", e.target.value)} />
      </Field>
    </FieldGroup>
  )
}
