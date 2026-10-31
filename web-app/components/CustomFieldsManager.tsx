"use client"

import { useEffect, useState } from "react"
import { Plus, SlidersHorizontal, Trash2 } from "lucide-react"
import { api } from "@/lib/api-client"
import type { CustomField, Page } from "@/lib/types"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Field, FieldDescription, FieldError, FieldLegend, FieldSet } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Item, ItemActions, ItemContent, ItemGroup, ItemTitle } from "@/components/ui/item"
import { Spinner } from "@/components/ui/spinner"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

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
    // Loads once the sheet opens; the state it sets comes from the
    // request's response, not synchronously in the effect body.
    // eslint-disable-next-line react-hooks/set-state-in-effect
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
    <FieldSet>
      <FieldLegend className="sr-only">Custom fields</FieldLegend>
      <FieldDescription>Extra attributes you can set on each application.</FieldDescription>
      {fields == null ? (
        <Spinner className="text-muted-foreground" />
      ) : fields.length === 0 ? (
        <Empty className="border p-4">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <SlidersHorizontal />
            </EmptyMedia>
            <EmptyTitle>No custom fields</EmptyTitle>
            <EmptyDescription>None yet — add one below.</EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <ItemGroup className="gap-2">
          {fields.map((f) => (
            <Item key={f.id} variant="outline" size="xs">
              <ItemContent>
                <ItemTitle>
                  {f.name}
                  <Badge variant="secondary">{f.type}</Badge>
                </ItemTitle>
              </ItemContent>
              <ItemActions>
                <Button variant="ghost" size="icon-xs" aria-label={`Delete ${f.name}`} disabled={busy} onClick={() => remove(f.id)}>
                  <Trash2 />
                </Button>
              </ItemActions>
            </Item>
          ))}
        </ItemGroup>
      )}
      <Field>
        <ButtonGroup className="w-full">
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
          <Button variant="outline" size="icon" disabled={busy || !name.trim()} onClick={create} aria-label="Add field">
            <Plus />
          </Button>
        </ButtonGroup>
        {error && <FieldError>{error}</FieldError>}
      </Field>
    </FieldSet>
  )
}
