"use client"

import { useState } from "react"
import { cn } from "@/lib/utils"
import { Checkbox } from "@/components/ui/checkbox"
import { Decode } from "@/components/detail/notes/Markdown"

export type DocSession = {
  id: string
  title: string
  meta: string
  md: string
  hidden?: boolean
  onSave: (md: string) => Promise<unknown>
  onShow?: (show: boolean) => void
}

type Line = { text: string; level: number; at: number }

function lines(md: string): Line[] {
  let fence = false
  return md.split("\n").map((text, at) => {
    if (/^\s*(```|~~~)/.test(text)) fence = !fence
    const h = !fence && text.match(/^\s*(#{1,6})\s/)
    return { text, level: h ? h[1].length : 0, at }
  })
}

// One markdown line with light highlighting: dim markers, strong headings.
function Styled({ text, level, reveal }: { text: string; level: number; reveal: boolean }) {
  if (level) {
    const m = text.match(/^(\s*#{1,6}\s)(.*)$/) ?? ["", "", text]
    return (
      <>
        <span className="text-muted-foreground/60">{m[1]}</span>
        <span className={cn("font-semibold", m[2].trim().endsWith("?") && "underline decoration-border underline-offset-4")}>
          <Decode text={m[2]} active={reveal} scope="notes" />
        </span>
      </>
    )
  }
  const li = text.match(/^(\s*(?:[-*]|\d+\.)\s(?:\[[ x]\]\s)?)(.*)$/)
  if (li)
    return (
      <>
        <span className="text-muted-foreground/60">{li[1]}</span>
        <Decode text={li[2]} active={reveal} scope="notes" />
      </>
    )
  return text.trim() ? <Decode text={text} active={reveal} scope="notes" /> : <span>&nbsp;</span>
}

function Gutter({ fold, open, onClick, label }: { fold: boolean; open?: boolean; onClick?: () => void; label?: string }) {
  if (!fold) return <span aria-hidden className="w-5 shrink-0" />
  return (
    <button type="button" aria-label={label} aria-expanded={open} onClick={onClick} className="w-5 shrink-0 rounded-sm text-center text-muted-foreground outline-none select-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring">
      {open ? "v" : ">"}
    </button>
  )
}

// The notes as one markdown file, the way an editor shows it: every heading
// folds from the gutter, and clicking a section's text edits it in place.
export function MarkdownDoc({ sessions, writable }: { sessions: DocSession[]; writable: boolean }) {
  const [shut, setShut] = useState<Record<string, boolean>>({})
  const [editing, setEditing] = useState<string | null>(null)
  const [draft, setDraft] = useState("")
  const [saving, setSaving] = useState(false)
  const [reveal, setReveal] = useState<string | null>(null)
  const flip = (key: string) => {
    if (shut[key]) setReveal(key)
    setShut((s) => ({ ...s, [key]: !s[key] }))
  }

  return (
    <div className="overflow-x-auto rounded-lg border bg-muted/30 py-2 font-mono text-[13px] leading-6">
      {sessions.map((s) => {
        const open = !shut[s.id]
        const body = lines(s.md)
        // A line is hidden when a heading above it, at its level or higher, is folded.
        const visible: Line[] = []
        let foldedAt = 0
        for (const l of body) {
          if (foldedAt && l.level && l.level <= foldedAt) foldedAt = 0
          if (foldedAt) continue
          visible.push(l)
          if (l.level && shut[`${s.id}:${l.at}`]) foldedAt = l.level
        }
        const isEditing = editing === s.id
        return (
          <section key={s.id} id={`note-${s.id}`} className={cn("group/doc scroll-mt-24", s.hidden && "opacity-60")}>
            <div className="flex items-center pr-3">
              <Gutter fold open={open} onClick={() => flip(s.id)} label={`Fold ${s.title}`} />
              <span className="min-w-0 flex-1 truncate">
                <span className="text-muted-foreground/60"># </span>
                <span className="font-semibold">{s.title}</span>
                {s.meta && <span className="text-muted-foreground"> · {s.meta}</span>}
                {s.hidden && <span className="text-muted-foreground"> · hidden</span>}
              </span>
              {writable && s.onShow && (
                <label className="flex shrink-0 items-center gap-1.5 font-sans text-xs text-muted-foreground opacity-0 group-hover/doc:opacity-100 focus-within:opacity-100">
                  <Checkbox checked={!s.hidden} onCheckedChange={(v) => s.onShow?.(v === true)} />
                  Show in notes
                </label>
              )}
            </div>
            {open &&
              (isEditing ? (
                <div className="flex flex-col gap-1 py-1 pr-3 pl-5">
                  <textarea
                    autoFocus
                    aria-label={`${s.title} notes`}
                    className="w-full resize-none rounded-md border bg-background px-2 py-1 font-mono text-[13px] leading-6 outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    rows={Math.max(3, draft.split("\n").length + 1)}
                    value={draft}
                    onChange={(e) => setDraft(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Escape") setEditing(null)
                      if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) void save(s)
                    }}
                  />
                  <div className="flex items-center gap-3 font-sans text-xs text-muted-foreground">
                    <button type="button" className="rounded-sm font-medium text-foreground hover:underline disabled:opacity-50" disabled={saving} onClick={() => void save(s)}>
                      Save
                    </button>
                    <button type="button" className="hover:text-foreground" onClick={() => setEditing(null)}>
                      Cancel
                    </button>
                    <span>⌘↵ saves · Esc cancels · a heading ending in ? is a question</span>
                  </div>
                </div>
              ) : (
                <div
                  role={writable ? "button" : undefined}
                  tabIndex={writable ? 0 : undefined}
                  aria-label={writable ? `Edit ${s.title}` : undefined}
                  onClick={(e) => {
                    if (!writable || (e.target as HTMLElement).closest("button")) return
                    setDraft(s.md)
                    setEditing(s.id)
                  }}
                  onKeyDown={(e) => {
                    if (writable && e.key === "Enter" && e.target === e.currentTarget) {
                      setDraft(s.md)
                      setEditing(s.id)
                    }
                  }}
                  className={cn("rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring", writable && "cursor-text hover:bg-muted/60")}
                >
                  {visible.length && s.md.trim() ? (
                    visible.map((l) => {
                      const key = `${s.id}:${l.at}`
                      return (
                        <div key={l.at} className="flex pr-3">
                          <span aria-hidden className="w-5 shrink-0" />
                          <Gutter fold={!!l.level} open={!shut[key]} onClick={() => flip(key)} label="Fold section" />
                          <span className="min-w-0 flex-1 whitespace-pre-wrap">
                            <Styled text={l.text} level={l.level} reveal={reveal === s.id || reveal === key} />
                          </span>
                        </div>
                      )
                    })
                  ) : (
                    <div className="flex pr-3 text-muted-foreground/60">
                      <span className="w-10 shrink-0" />
                      {writable ? "Click to write…" : "Nothing yet."}
                    </div>
                  )}
                </div>
              ))}
          </section>
        )
      })}
    </div>
  )

  async function save(s: DocSession) {
    setSaving(true)
    try {
      await s.onSave(draft)
      setEditing(null)
    } finally {
      setSaving(false)
    }
  }
}
