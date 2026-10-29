import { useEffect, useRef, useState } from "react"
import { useNavigate } from "react-router-dom"
import { ArrowRight, Download, FileUp, Plus } from "lucide-react"
import { api, ApiError } from "@/lib/api"
import type { Application, ImportResult, Page } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { CommandBlock } from "@/components/CommandBlock"

const IMPORTERS: { key: string; path: string; label: string; contentType: string; accept: string }[] = [
  { key: "csv", path: "/v1/import/csv", label: "CSV", contentType: "text/csv", accept: ".csv,text/csv" },
  { key: "json", path: "/v1/import/json", label: "JSON", contentType: "application/json", accept: ".json,application/json" },
  { key: "markdown", path: "/v1/import/markdown", label: "Markdown", contentType: "text/markdown", accept: ".md,text/markdown" },
]

function ImportCard({ importer }: { importer: (typeof IMPORTERS)[number] }) {
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

export function HomePage() {
  const navigate = useNavigate()
  const [hasApplications, setHasApplications] = useState<boolean | null>(null)

  useEffect(() => {
    let cancelled = false
    api<Page<Application>>("/v1/applications?limit=1")
      .then((page) => {
        if (!cancelled) setHasApplications(page.page.total > 0)
      })
      .catch(() => {
        if (!cancelled) setHasApplications(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-8 py-8">
      <div className="flex flex-col items-center gap-3 text-center">
        <h1 className="font-display text-xl font-semibold">For your higher pursuits.</h1>
        <p className="max-w-md text-muted-foreground">Land the role.</p>
        <div className="flex w-64 flex-col items-stretch gap-2">
          {hasApplications && (
            <Button size="lg" className="w-full" onClick={() => navigate("/pipeline")}>
              View applications <ArrowRight className="size-4" />
            </Button>
          )}
          <Button
            size="lg"
            className="w-full"
            variant={hasApplications ? "secondary" : "default"}
            onClick={() => navigate("/app/new")}
          >
            <Plus className="size-4" /> Start a new application
          </Button>
        </div>
      </div>

      <Card size="sm">
        <CardHeader>
          <CardTitle>Prefer the terminal?</CardTitle>
          <CardDescription>
            Everything here is also a <code className="font-mono text-foreground">pursuit</code> command.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <CommandBlock command="pursuit list --stale" />
        </CardContent>
      </Card>

      <Card size="sm">
        <CardHeader>
          <CardTitle>Browser extension</CardTitle>
          <CardDescription>
            Save the posting you have open and autofill Greenhouse, Ashby, and Lever application forms from this server.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="outline" size="sm" asChild>
            <a href="https://github.com/adrianmross/pursuit#browser-extension" target="_blank" rel="noopener noreferrer">
              <Download className="size-3.5" /> Get the extension
            </a>
          </Button>
        </CardContent>
      </Card>

      <div className="flex flex-col gap-3">
        <h2 className="text-sm font-medium text-muted-foreground">Import existing data</h2>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          {IMPORTERS.map((importer) => (
            <ImportCard key={importer.key} importer={importer} />
          ))}
        </div>
      </div>
    </div>
  )
}
