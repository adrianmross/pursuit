"use client"

import { useLayoutEffect, useRef, useState } from "react"
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
  // Renaming the # heading renames the round; absent, the title is fixed.
  onRename?: (title: string) => Promise<unknown>
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

// The whole document as one file: each part under its own # heading.
function compose(sessions: DocSession[]) {
  return sessions.map((s) => `# ${s.title}\n${s.md.trim() ? `\n${s.md.trim()}\n` : ""}`).join("\n")
}

// Splits the file back at its # headings, one part per session, in order.
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

// The notes as one markdown file. Read, it folds from the gutter; click
// anywhere and the whole file is one editor, the cursor where you clicked.
// Clicking out (or Cmd+Enter) saves each part back where it belongs.
export function MarkdownDoc({ sessions, writable }: { sessions: DocSession[]; writable: boolean }) {
  const [shut, setShut] = useState<Record<string, boolean>>({})
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState("")
  const [error, setError] = useState("")
  const [reveal, setReveal] = useState<string | null>(null)
  const area = useRef<HTMLTextAreaElement>(null)
  const [caretAt, setCaretAt] = useState(0)
  const flip = (key: string) => {
    if (shut[key]) setReveal(key)
    setShut((s) => ({ ...s, [key]: !s[key] }))
  }

  // Each session's first line in the composed file, to put the cursor where
  // the click was.
  const starts: Record<string, number> = {}
  {
    let n = 0
    for (const s of sessions) {
      starts[s.id] = n
      n += 1 + (s.md.trim() ? s.md.trim().split("\n").length + 2 : 0) + 1
    }
  }
  const open = (sessionId: string, line: number) => {
    if (!writable) return
    const text = compose(sessions)
    const target = starts[sessionId] + line
    setCaretAt(text.split("\n").slice(0, target).join("\n").length + (target ? 1 : 0))
    setDraft(text)
    setError("")
    setEditing(true)
  }
  useLayoutEffect(() => {
    if (!editing || !area.current) return
    area.current.focus()
    area.current.setSelectionRange(caretAt, caretAt)
  }, [editing, caretAt])

  async function save() {
    const parts = split(draft)
    if (parts.length !== sessions.length) {
      setError(`Keep one # heading for each of the ${sessions.length} parts; the file has ${parts.length}.`)
      return
    }
    setEditing(false)
    await Promise.all(
      sessions.map((s, i) => {
        const p = parts[i]
        const jobs: Promise<unknown>[] = []
        if (p.body !== s.md.trim()) jobs.push(s.onSave(p.body))
        if (s.onRename && p.title !== s.title && p.title) jobs.push(s.onRename(p.title))
        return Promise.all(jobs)
      }),
    )
  }

  if (editing)
    return (
      <div className="flex flex-col gap-1">
        <textarea
          ref={area}
          aria-label="Notes"
          spellCheck
          className="w-full resize-none rounded-lg border bg-background px-3 py-2 font-mono text-[13px] leading-6 outline-none focus-visible:ring-2 focus-visible:ring-ring"
          rows={Math.max(8, draft.split("\n").length + 2)}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onBlur={() => void save()}
          onKeyDown={(e) => {
            if (e.key === "Escape") setEditing(false)
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
              e.preventDefault()
              void save()
            }
          }}
        />
        <p className={cn("text-xs", error ? "text-destructive" : "text-muted-foreground")}>
          {error || "Click out or ⌘↵ to save · Esc cancels · # starts a round · a heading ending in ? is a question"}
        </p>
      </div>
    )

  return (
    <div className="overflow-x-auto rounded-lg border bg-muted/30 py-2 font-mono text-[13px] leading-6">
      {sessions.map((s) => {
        const isOpen = !shut[s.id]
        const body = lines(s.md.trim())
        // A line is hidden when a heading above it, at its level or higher, is folded.
        const visible: Line[] = []
        let foldedAt = 0
        for (const l of body) {
          if (foldedAt && l.level && l.level <= foldedAt) foldedAt = 0
          if (foldedAt) continue
          visible.push(l)
          if (l.level && shut[`${s.id}:${l.at}`]) foldedAt = l.level
        }
        return (
          <section key={s.id} id={`note-${s.id}`} className={cn("group/doc scroll-mt-24", s.hidden && "opacity-60")}>
            <div className="flex items-center pr-3">
              <Gutter fold open={isOpen} onClick={() => flip(s.id)} label={`Fold ${s.title}`} />
              <span className={cn("min-w-0 flex-1 truncate", writable && "cursor-text")} onClick={() => open(s.id, 0)}>
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
            {isOpen &&
              (visible.length && s.md.trim() ? (
                visible.map((l) => {
                  const key = `${s.id}:${l.at}`
                  return (
                    <div key={l.at} className="flex pr-3">
                      <span aria-hidden className="w-5 shrink-0" />
                      <Gutter fold={!!l.level} open={!shut[key]} onClick={() => flip(key)} label="Fold section" />
                      {/* +2: the # heading and the blank line under it */}
                      <span className={cn("min-w-0 flex-1 whitespace-pre-wrap", writable && "cursor-text")} onClick={() => open(s.id, l.at + 2)}>
                        <Styled text={l.text} level={l.level} reveal={reveal === s.id || reveal === key} />
                      </span>
                    </div>
                  )
                })
              ) : (
                <div className={cn("flex pr-3 text-muted-foreground/60", writable && "cursor-text")} onClick={() => open(s.id, 1)}>
                  <span className="w-10 shrink-0" />
                  {writable ? "Click to write…" : "Nothing yet."}
                </div>
              ))}
          </section>
        )
      })}
    </div>
  )
}
