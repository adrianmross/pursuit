"use client"

import { useEffect, useRef, useState } from "react"
import { EditorState } from "@codemirror/state"
import { EditorView } from "@codemirror/view"
import { MergeView, unifiedMergeView } from "@codemirror/merge"
import { api } from "@adrianmross/appkit-ui"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@adrianmross/appkit-ui/ui/dialog"
import { Spinner } from "@adrianmross/appkit-ui/ui/spinner"

type Side = { title: string; text: string; source?: string }
type Comparison = { default: Side; current: Side; same: boolean }

// Read-only, wrapped, in the page's own colors.
const base = [
  EditorState.readOnly.of(true),
  EditorView.editable.of(false),
  EditorView.lineWrapping,
  EditorView.theme({
    "&": { fontSize: "13px", backgroundColor: "transparent", color: "var(--foreground)" },
    ".cm-content": { fontFamily: "var(--font-sans)", padding: "8px 0" },
    ".cm-changedLine": { backgroundColor: "color-mix(in oklab, var(--primary) 6%, transparent)" },
    ".cm-changedText": { backgroundColor: "color-mix(in oklab, var(--primary) 22%, transparent)" },
    ".cm-deletedChunk": { backgroundColor: "color-mix(in oklab, var(--destructive) 8%, transparent)" },
  }),
]

// The application's résumé or cover letter beside the main one, as text.
// Side by side when there's room, one column with changes inline on a phone.
function Diff({ cmp }: { cmp: Comparison }) {
  const host = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const parent = host.current
    if (!parent) return
    if (window.matchMedia("(max-width: 639px)").matches) {
      const view = new EditorView({ parent, doc: cmp.current.text, extensions: [...base, unifiedMergeView({ original: cmp.default.text, mergeControls: false })] })
      return () => view.destroy()
    }
    const view = new MergeView({ parent, a: { doc: cmp.default.text, extensions: base }, b: { doc: cmp.current.text, extensions: base } })
    return () => view.destroy()
  }, [cmp])
  return <div ref={host} className="max-h-[70vh] overflow-auto rounded-md border" />
}

export function CompareDialog({ appId, kind, open, onOpenChange }: { appId: string; kind: "resume_pdf" | "cover_letter"; open: boolean; onOpenChange: (o: boolean) => void }) {
  const [cmp, setCmp] = useState<Comparison | null>(null)
  const [error, setError] = useState("")
  useEffect(() => {
    if (!open) return
    let live = true
    api<Comparison>(`/v1/applications/${encodeURIComponent(appId)}/compare?kind=${kind}`)
      .then((c) => live && setCmp(c))
      .catch((e) => live && setError(e instanceof Error ? e.message : "Could not compare"))
    return () => {
      live = false
    }
  }, [open, appId, kind])
  const what = kind === "resume_pdf" ? "résumé" : "cover letter"
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-5xl">
        <DialogHeader>
          <DialogTitle>Compare with your main {what}</DialogTitle>
          <DialogDescription>
            {cmp ? `What changes from ${cmp.default.title} (main) to ${cmp.current.title} (this application). Text only: layout and styling aren't compared.` : `Reading both files…`}
          </DialogDescription>
        </DialogHeader>
        {error ? (
          <p className="text-sm text-destructive">{error}</p>
        ) : !cmp ? (
          <Spinner className="mx-auto my-8 text-muted-foreground" />
        ) : cmp.same ? (
          <p className="py-6 text-center text-sm text-muted-foreground">Same text as your main {what}.</p>
        ) : (
          <Diff cmp={cmp} />
        )}
      </DialogContent>
    </Dialog>
  )
}
