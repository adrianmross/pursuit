"use client"

import { useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { AlertCircle, ArrowLeft } from "lucide-react"
import { api, ApiError } from "@/lib/api-client"
import { applicationHref } from "@/lib/links"
import type { Application } from "@/lib/types"
import { AttributesForm, EMPTY_ATTRIBUTES, type AttributesFormState } from "@/components/AttributesForm"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Spinner } from "@/components/ui/spinner"
import { Textarea } from "@/components/ui/textarea"

export function CreateApplicationView() {
  const router = useRouter()
  const [attrs, setAttrs] = useState<AttributesFormState>(EMPTY_ATTRIBUTES)
  const [notes, setNotes] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function create() {
    setBusy(true)
    setError(null)
    try {
      const created = await api<Application>("/v1/applications", { method: "POST", body: { ...attrs, notes } })
      router.replace(applicationHref(created.id))
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save")
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-4">
      <Link href="/pipeline" aria-label="Back to pipeline" className="text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" />
      </Link>
      <h1 className="text-xl font-semibold">Add application</h1>
      <Card>
        <CardContent className="pt-6">
          <AttributesForm value={attrs} onChange={setAttrs} showCompany />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Notes</CardTitle>
        </CardHeader>
        <CardContent>
          <Textarea
            className="min-h-40 rounded-lg border-transparent bg-transparent px-3 py-3 shadow-none focus-visible:border-transparent focus-visible:ring-1 focus-visible:ring-border"
            placeholder="Anything worth remembering about this one…"
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
          />
        </CardContent>
      </Card>
      {error && (
        <Alert variant="destructive">
          <AlertCircle />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <div className="flex gap-2">
        <Button onClick={create} disabled={busy || !attrs.company.trim()}>
          {busy && <Spinner data-icon="inline-start" />}
          Add application
        </Button>
        <Button variant="outline" asChild>
          <Link href="/pipeline">Cancel</Link>
        </Button>
      </div>
    </div>
  )
}
