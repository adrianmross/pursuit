"use client"

import { useState, type FormEvent } from "react"
import { Eye, MoreHorizontal, PenLine, Star } from "lucide-react"
import { api } from "@adrianmross/appkit-ui"
import type { Document } from "@/lib/types"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { FileDrop } from "@/components/FileDrop"
import { FileIcon, FilePreview, isTextType } from "@/components/FilePreview"
import { ProfileHeader, Section, useList } from "@/components/profile/ProfilePage"
import { Badge } from "@adrianmross/appkit-ui/ui/badge"
import { Button } from "@adrianmross/appkit-ui/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@adrianmross/appkit-ui/ui/dialog"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@adrianmross/appkit-ui/ui/dropdown-menu"
import { Field, FieldGroup, FieldLabel } from "@adrianmross/appkit-ui/ui/field"
import { InputGroup, InputGroupInput, InputGroupTextarea } from "@adrianmross/appkit-ui/ui/input-group"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemMedia, ItemTitle } from "@adrianmross/appkit-ui/ui/item"
import { Spinner } from "@adrianmross/appkit-ui/ui/spinner"

export const KINDS: Record<string, string> = { base_resume: "Résumé", cover_letter: "Cover letter", other: "Other" }
const GROUPS: [kind: string, title: string, description: string][] = [
  ["base_resume", "Résumés", "Prepare attaches the main one. Keep other versions here, named for what they're for."],
  ["cover_letter", "Cover letters", "The main one is attached when a form asks for a cover letter and answers a cover-letter question."],
  ["other", "Other", "Anything else: a LinkedIn export, a portfolio PDF, references."],
]

export const docName = (d: Document) => d.label || d.filename || d.id

export function size(n?: number) {
  if (!n) return ""
  return n < 1024 ? `${n} B` : n < 1 << 20 ? `${Math.round(n / 1024)} KB` : `${(n / (1 << 20)).toFixed(1)} MB`
}

// What a dialog edits: a document's name, a text document's content, or a new cover letter.
type Editing = { mode: "rename" | "edit"; doc: Document; label: string; body: string } | { mode: "write"; label: string; body: string }

function EditDialog({ editing, onClose, onSaved }: { editing: Editing; onClose: () => void; onSaved: () => Promise<void> }) {
  const [label, setLabel] = useState(editing.label)
  const [body, setBody] = useState(editing.body)
  const [error, setError] = useState("")
  const [saving, setSaving] = useState(false)
  const rename = editing.mode === "rename"

  async function submit(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    setError("")
    try {
      if (editing.mode === "write")
        await api("/v1/documents", { method: "POST", body: { kind: "cover_letter", filename: "cover-letter.md", content_type: "text/markdown", label, body } })
      else await api("/v1/documents/" + editing.doc.id, { method: "PATCH", body: rename ? { label } : { label, body } })
      await onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save")
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className={rename ? undefined : "sm:max-w-2xl"}>
        <form onSubmit={submit} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>{editing.mode === "write" ? "Write a cover letter" : rename ? "Rename" : "Edit " + docName(editing.doc)}</DialogTitle>
            <DialogDescription>
              {rename ? "A name for this version, like Backend or Short." : "Plain text or Markdown. Prepare attaches it as is."}
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="doc-label">Name</FieldLabel>
              <InputGroup>
                <InputGroupInput id="doc-label" autoFocus={rename} placeholder="e.g. Default" value={label} onChange={(e) => setLabel(e.target.value)} />
              </InputGroup>
            </Field>
            {!rename && (
              <Field>
                <FieldLabel htmlFor="doc-body">Letter</FieldLabel>
                <InputGroup>
                  <InputGroupTextarea id="doc-body" autoFocus rows={14} value={body} onChange={(e) => setBody(e.target.value)} />
                </InputGroup>
              </Field>
            )}
          </FieldGroup>
          {error && <p className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={saving || (!rename && !body.trim())}>
              {saving && <Spinner />} Save
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function DocumentRow({ d, onChanged, onEdit, onDelete }: { d: Document; onChanged: () => Promise<void>; onEdit: (e: Editing) => void; onDelete: () => void }) {
  const text = isTextType(d.content_type)
  const main = d.kind !== "other"
  async function makeMain() {
    await api("/v1/documents/" + d.id, { method: "PATCH", body: { primary: true } })
    await onChanged()
  }
  async function edit() {
    const full = await api<{ body: string }>("/v1/documents/" + d.id)
    onEdit({ mode: "edit", doc: d, label: d.label || "", body: full.body })
  }
  return (
    <Item variant="outline" size="sm">
      <ItemMedia variant="icon">
        <FileIcon type={d.content_type} name={d.filename} />
      </ItemMedia>
      <ItemContent>
        <ItemTitle>
          {docName(d)}
          {d.primary && <Badge variant="secondary">Main</Badge>}
        </ItemTitle>
        <ItemDescription>
          {[d.label && d.filename, size(d.size), d.created_at && new Date(d.created_at).toLocaleDateString()].filter(Boolean).join(" · ")}
        </ItemDescription>
      </ItemContent>
      <ItemActions>
        <FilePreview path={"/v1/documents/" + d.id} base64={(f) => !isTextType(f.content_type)} title={docName(d)} description={KINDS[d.kind]}>
          <Button size="icon-sm" variant="ghost" aria-label={`Preview ${docName(d)}`}>
            <Eye />
          </Button>
        </FilePreview>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button size="icon-sm" variant="ghost" aria-label={`More for ${docName(d)}`}>
              <MoreHorizontal />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {main && !d.primary && (
              <DropdownMenuItem onClick={() => void makeMain()}>
                <Star /> Make main
              </DropdownMenuItem>
            )}
            <DropdownMenuItem onClick={() => onEdit({ mode: "rename", doc: d, label: d.label || "", body: "" })}>Rename</DropdownMenuItem>
            {text && (
              <DropdownMenuItem onClick={() => void edit()}>
                <PenLine /> Edit text
              </DropdownMenuItem>
            )}
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onClick={onDelete}>
              Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </ItemActions>
    </Item>
  )
}

function Documents() {
  const { rows: docs, load } = useList<Document>("/v1/documents")
  const [editing, setEditing] = useState<Editing | null>(null)
  const [deleting, setDeleting] = useState<Document | null>(null)
  const [busy, setBusy] = useState(false)

  async function remove() {
    if (!deleting) return
    setBusy(true)
    try {
      await api("/v1/documents/" + deleting.id, { method: "DELETE" })
      await load()
    } finally {
      setBusy(false)
      setDeleting(null)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <FileDrop
        path="/v1/documents"
        kinds={KINDS}
        description={'PDF, Word, Markdown, or text, up to 5MB. A LinkedIn "Save to PDF" export works too.'}
        onUploaded={() => void load()}
      />
      {GROUPS.map(([kind, title, description]) => {
        const rows = docs?.filter((d) => d.kind === kind) ?? []
        if (!rows.length && kind !== "cover_letter") return null
        return (
          <Section key={kind} id={kind} title={title} description={description}>
            <ItemGroup className="gap-2">
              {rows.map((d) => (
                <DocumentRow key={d.id} d={d} onChanged={load} onEdit={setEditing} onDelete={() => setDeleting(d)} />
              ))}
              {kind === "cover_letter" && (
                <Button variant="outline" size="sm" className="self-start" onClick={() => setEditing({ mode: "write", label: rows.length ? "" : "Default", body: "" })}>
                  <PenLine /> Write a cover letter
                </Button>
              )}
            </ItemGroup>
          </Section>
        )
      })}
      {editing && <EditDialog editing={editing} onClose={() => setEditing(null)} onSaved={load} />}
      <ConfirmDialog
        open={!!deleting}
        onOpenChange={(o) => !o && setDeleting(null)}
        title={`Delete ${deleting ? docName(deleting) : ""}?`}
        description={
          deleting?.primary
            ? "This is your main one. The newest other version becomes the main one; applications already prepared keep their copy."
            : "Applications already prepared keep their copy."
        }
        confirmLabel="Delete"
        danger
        busy={busy}
        onConfirm={() => void remove()}
      />
    </div>
  )
}

export function DocumentsPage() {
  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-4">
      <ProfileHeader description="Every file you keep: your main résumé and cover letter, their other versions, and anything else." />
      <Documents />
    </div>
  )
}
