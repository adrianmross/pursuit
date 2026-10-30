"use client"

import { useRef, useState } from "react"
import { FileUp } from "lucide-react"
import { api, ApiError } from "@/lib/api-client"
import type { ImportResult } from "@/lib/types"
import type { IMPORTERS } from "@/lib/importers"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export function ImportCard({ importer }: { importer: (typeof IMPORTERS)[number] }) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState<ImportResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function onFile(file: File) {
    setBusy(true)
    setError(null)
    setResult(null)
    try {
      const text = await file.text()
      const imported = await api<ImportResult>(importer.path, { rawBody: { text, contentType: importer.contentType } })
      setResult(imported)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Import failed")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle className="text-sm">{importer.label}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-2">
        <input
          ref={inputRef}
          type="file"
          accept={importer.accept}
          className="hidden"
          onChange={(event) => {
            const file = event.target.files?.[0]
            if (file) void onFile(file)
            event.target.value = ""
          }}
        />
        <Button variant="outline" size="sm" disabled={busy} onClick={() => inputRef.current?.click()}>
          <FileUp className="size-3.5" /> Import {importer.label}
        </Button>
        {result && (
          <p className="text-xs text-muted-foreground">
            {result.applications_created} created, {result.applications_updated} updated, {result.skipped} skipped
          </p>
        )}
        {error && <p className="text-xs text-destructive">{error}</p>}
      </CardContent>
    </Card>
  )
}
