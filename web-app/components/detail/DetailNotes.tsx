"use client"

import { InlineText } from "@/components/InlineEdit"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export function DetailNotes({
  notes,
  writable,
  onSave,
}: {
  notes: string
  writable: boolean
  onSave: (next: string) => Promise<unknown>
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Notes</CardTitle>
      </CardHeader>
      <CardContent>
        <InlineText
          multiline
          value={notes}
          label="Notes"
          placeholder="Anything worth remembering about this one…"
          disabled={!writable}
          className="text-sm [&>[role=textbox]]:min-h-40"
          onSave={onSave}
        />
      </CardContent>
    </Card>
  )
}
