"use client"

import { useEffect, useRef, useState } from "react"
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands"
import { markdown } from "@codemirror/lang-markdown"
import { codeFolding, foldAll, foldGutter, foldKeymap, foldService, foldedRanges, HighlightStyle, syntaxHighlighting, unfoldEffect } from "@codemirror/language"
import { EditorState, type Extension } from "@codemirror/state"
import { EditorView, keymap } from "@codemirror/view"
import { tags } from "@lezer/highlight"
import { vim } from "@replit/codemirror-vim"
import { getPref } from "@/lib/settings"

export type NoteSection = {
  id: string
  title: string
  md: string
  onSave: (md: string) => Promise<unknown>
  // Renaming the # heading renames the round; absent, the title is fixed.
  onRename?: (title: string) => Promise<unknown>
}

// The whole document as one file: each part under its own # heading.
function compose(sections: NoteSection[]) {
  return sections.map((s) => `# ${s.title}\n${s.md.trim() ? `\n${s.md.trim()}\n` : ""}`).join("\n")
}

// Splits the file back at its # headings, one part per section, in order.
function split(text: string): { title: string; body: string }[] {
  const out: { title: string; body: string[] }[] = []
  let fence = false
  for (const line of text.split("\n")) {
    if (/^\s*(```|~~~)/.test(line)) fence = !fence
    const h = !fence && line.match(/^#\s+(.*?)\s*$/)
    if (h) out.push({ title: h[1], body: [] })
    else if (out.length) out[out.length - 1].body.push(line)
  }
  return out.map((p) => ({ title: p.title, body: p.body.join("\n").trim() }))
}

// A heading folds everything down to the next heading at its level or above.
const headingFolds = foldService.of((state, from) => {
  const line = state.doc.lineAt(from)
  const m = line.text.match(/^(#{1,6})\s/)
  if (!m) return null
  const level = m[1].length
  let end = line
  for (let n = line.number + 1; n <= state.doc.lines; n++) {
    const next = state.doc.line(n)
    const h = next.text.match(/^(#{1,6})\s/)
    if (h && h[1].length <= level) break
    end = next
  }
  // Trailing blank lines stay visible as the gap before the next heading.
  while (end.number > line.number && !end.text.trim()) end = state.doc.line(end.number - 1)
  return end.number > line.number ? { from: line.to, to: end.to } : null
})

const highlight = HighlightStyle.define([
  { tag: tags.heading, fontWeight: "600" },
  { tag: tags.processingInstruction, color: "var(--muted-foreground)", opacity: "0.6" },
  { tag: tags.list, color: "var(--muted-foreground)" },
  { tag: tags.emphasis, fontStyle: "italic" },
  { tag: tags.strong, fontWeight: "600" },
  { tag: tags.monospace, backgroundColor: "var(--muted)", borderRadius: "3px" },
])

const theme = EditorView.theme({
  "&": { fontSize: "13px", backgroundColor: "transparent", color: "var(--foreground)" },
  "&.cm-focused": { outline: "none" },
  ".cm-content": { fontFamily: "var(--font-mono, ui-monospace, monospace)", lineHeight: "1.5rem", padding: "8px 0", caretColor: "var(--foreground)" },
  ".cm-gutters": { backgroundColor: "transparent", border: "none", color: "var(--muted-foreground)" },
  ".cm-foldGutter .cm-gutterElement": { width: "1.25rem", textAlign: "center", cursor: "pointer", fontFamily: "var(--font-mono, ui-monospace, monospace)" },
  ".cm-foldPlaceholder": { background: "transparent", border: "none", color: "var(--muted-foreground)", padding: "0 4px", margin: "0", cursor: "pointer" },
  ".cm-activeLine": { backgroundColor: "color-mix(in oklab, var(--muted) 60%, transparent)" },
  ".cm-selectionBackground, &.cm-focused .cm-selectionBackground": { backgroundColor: "color-mix(in oklab, var(--foreground) 15%, transparent)" },
  ".cm-line": { padding: "0 12px 0 4px" },
})

// The notes as one markdown file in an editor. It opens condensed, every
// section folded behind a ---> marker; the cursor reaching a folded heading
// opens it, and leaving the editor saves every part and folds it all back.
// zc, zo and za fold with Vim keys on (Settings, Appearance).
export function NotesEditor({ sections, writable }: { sections: NoteSection[]; writable: boolean }) {
  const host = useRef<HTMLDivElement>(null)
  const view = useRef<EditorView | null>(null)
  const latest = useRef(sections)
  const [error, setError] = useState("")
  const text = compose(sections)

  useEffect(() => {
    latest.current = sections
  })

  useEffect(() => {
    if (!host.current) return
    const save = async (v: EditorView) => {
      const current = latest.current
      const parts = split(v.state.doc.toString())
      if (parts.length !== current.length) {
        setError(`Keep one # heading for each of the ${current.length} parts; the file has ${parts.length}.`)
        return
      }
      setError("")
      await Promise.all(
        current.map((s, i) => {
          const p = parts[i]
          const jobs: Promise<unknown>[] = []
          if (p.body !== s.md.trim()) jobs.push(s.onSave(p.body))
          if (s.onRename && p.title && p.title !== s.title) jobs.push(s.onRename(p.title))
          return Promise.all(jobs)
        }),
      )
    }
    // Opens the fold that starts on the cursor's line.
    const openAtCursor = EditorView.updateListener.of((u) => {
      if (!u.selectionSet) return
      const line = u.state.doc.lineAt(u.state.selection.main.head)
      const effects: ReturnType<typeof unfoldEffect.of>[] = []
      foldedRanges(u.state).between(line.to, line.to + 1, (from, to) => {
        if (from === line.to) effects.push(unfoldEffect.of({ from, to }))
      })
      if (effects.length) u.view.dispatch({ effects })
    })
    const blur = EditorView.domEventHandlers({
      blur: (_e, v) => {
        void save(v)
        foldAll(v)
      },
      keydown: (e, v) => {
        if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
          e.preventDefault()
          void save(v)
          return true
        }
        return false
      },
    })
    const extensions: Extension[] = [
      getPref("vimKeys") === "on" ? vim() : [],
      history(),
      markdown(),
      headingFolds,
      codeFolding({ placeholderText: "--->" }),
      foldGutter({ openText: "v", closedText: ">" }),
      syntaxHighlighting(highlight),
      keymap.of([...foldKeymap, ...defaultKeymap, ...historyKeymap]),
      EditorView.lineWrapping,
      EditorState.readOnly.of(!writable),
      EditorView.contentAttributes.of({ "aria-label": "Notes", spellcheck: "true" }),
      openAtCursor,
      blur,
      theme,
    ]
    const v = new EditorView({ parent: host.current, state: EditorState.create({ doc: text, extensions }) })
    view.current = v
    foldAll(v)
    return () => {
      v.destroy()
      view.current = null
    }
    // The editor is made once; text changes arrive through the effect below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [writable])

  // Saves and agent edits land when you are not typing in it.
  useEffect(() => {
    const v = view.current
    if (!v || v.hasFocus || v.state.doc.toString() === text) return
    v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: text } })
    foldAll(v)
  }, [text])

  return (
    <div className="flex flex-col gap-1">
      <div ref={host} className="overflow-hidden rounded-lg border bg-muted/30 focus-within:ring-2 focus-within:ring-ring" />
      <p className={error ? "text-xs text-destructive" : "text-xs text-muted-foreground"}>
        {error || "Move onto a ---> line to open it · click out or ⌘↵ saves and folds · # starts a round · a heading ending in ? is a question"}
      </p>
    </div>
  )
}

