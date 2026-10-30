"use client"

import { useEffect, useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Textarea } from "@/components/ui/textarea"
import { Button } from "@/components/ui/button"

export function DetailNotes({
  notes,
  writable,
  onSave,
}: {
  notes: string
  writable: boolean
  onSave: (next: string) => Promise<void>
}) {
  const [draft, setDraft] = useState(notes)
  const [busy, setBusy] = useState(false)
  const dirty = draft !== notes

  // Re-syncs the draft when the server value changes externally.
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setDraft(notes), [notes])

  async function save() {
    setBusy(true)
    try {
      await onSave(draft)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Notes</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-2">
        <Textarea
          className="min-h-40 rounded-lg border-transparent bg-transparent px-3 py-3 shadow-none focus-visible:border-transparent focus-visible:ring-1 focus-visible:ring-border"
          placeholder="Anything worth remembering about this one…"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          disabled={!writable}
        />
        {dirty && writable && (
          <div className="flex gap-2">
            <Button size="sm" onClick={save} disabled={busy}>
              Save notes
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setDraft(notes)}>
              Discard
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
