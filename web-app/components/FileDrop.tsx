"use client"

import { useState, type DragEvent } from "react"
import { FileUp } from "lucide-react"
import { apiHeaders, apiUrl } from "@/lib/api-client"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Spinner } from "@/components/ui/spinner"

// Drag-and-drop or browse upload to a multipart endpoint that takes
// (kind, file): /v1/documents and /v1/applications/{id}/artifacts.
export function FileDrop({
  path,
  kinds,
  description,
  onUploaded,
  compact,
}: {
  path: string
  kinds: Record<string, string>
  description: string
  onUploaded: () => void
  compact?: boolean
}) {
  const [kind, setKind] = useState(Object.keys(kinds)[0])
  const [busy, setBusy] = useState(false)
  const [over, setOver] = useState(false)
  const [error, setError] = useState("")

  async function upload(file?: File) {
    if (!file) return
    const form = new FormData()
    form.set("kind", kind)
    form.set("file", file)
    setBusy(true)
    try {
      const res = await fetch(apiUrl(path), { method: "POST", headers: apiHeaders(), body: form })
      if (!res.ok) {
        const body = await res.json().catch(() => null)
        return setError(body?.error?.message || "Upload failed")
      }
      setError("")
      onUploaded()
    } finally {
      setBusy(false)
    }
  }

  function drop(e: DragEvent) {
    e.preventDefault()
    setOver(false)
    void upload(e.dataTransfer.files[0])
  }

  return (
    <Empty
      onDragOver={(e) => {
        e.preventDefault()
        setOver(true)
      }}
      onDragLeave={() => setOver(false)}
      onDrop={drop}
      className={cn("border border-dashed transition-colors", compact ? "gap-3 p-4" : "p-6", over && "border-primary bg-primary/5")}
    >
      <EmptyHeader>
        <EmptyMedia variant="icon">{busy ? <Spinner /> : <FileUp />}</EmptyMedia>
        <EmptyTitle>Drop a file here</EmptyTitle>
        <EmptyDescription>{description}</EmptyDescription>
      </EmptyHeader>
      <EmptyContent>
        <ButtonGroup>
          <Select value={kind} onValueChange={setKind}>
            <SelectTrigger className="w-40" aria-label="File kind">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {Object.entries(kinds).map(([k, label]) => (
                <SelectItem key={k} value={k}>
                  {label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button variant="outline" disabled={busy} asChild>
            <label>
              Browse
              <input type="file" className="sr-only" onChange={(e) => void upload(e.target.files?.[0])} />
            </label>
          </Button>
        </ButtonGroup>
        {error && <p className="text-sm text-destructive">{error}</p>}
      </EmptyContent>
    </Empty>
  )
}
