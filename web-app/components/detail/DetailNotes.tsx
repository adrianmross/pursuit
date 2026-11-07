"use client"

import { InlineText } from "@/components/InlineEdit"
import { SectionCard } from "@/components/detail/SectionCard"

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
    <SectionCard id="notes" title="Notes">
      <InlineText
        multiline
        value={notes}
        label="Notes"
        placeholder="Anything worth remembering about this one…"
        disabled={!writable}
        className="text-sm [&>[role=textbox]]:min-h-40"
        onSave={onSave}
      />
    </SectionCard>
  )
}
