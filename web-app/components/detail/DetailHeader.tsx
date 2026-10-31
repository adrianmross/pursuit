"use client"

import { useState } from "react"
import { Check, MoreHorizontal, X } from "lucide-react"
import { markOf, shown, type Application } from "@/lib/types"
import { STATUS_OPTIONS } from "@/components/AttributesForm"
import { api } from "@/lib/api-client"
import { CompanyAvatar, forgetFavicon } from "@/components/CompanyAvatar"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { InlineText } from "@/components/InlineEdit"
import { ProgressMark } from "@/components/ProgressMark"
import { StatusBadge } from "@/components/StatusBadge"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

function StatusEditor({ value, disabled, onSave }: { value: string; disabled?: boolean; onSave: (next: string) => void }) {
  const [editing, setEditing] = useState(false)

  if (disabled) return <StatusBadge status={value} />

  if (editing) {
    return (
      <Select
        defaultOpen
        value={value}
        onValueChange={(next) => {
          setEditing(false)
          if (next !== value) onSave(next)
        }}
        onOpenChange={(open) => !open && setEditing(false)}
      >
        <SelectTrigger className="h-7 w-auto gap-1 border-none bg-transparent px-0 shadow-none focus-visible:ring-0" aria-label="Status">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {STATUS_OPTIONS.map(([v, text]) => (
            <SelectItem key={v} value={v}>
              {text}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    )
  }

  return (
    <button type="button" className="rounded-full" onClick={() => setEditing(true)}>
      <StatusBadge status={value} />
    </button>
  )
}

function IconEditor({ application }: { application: Application }) {
  const [open, setOpen] = useState(false)
  const [value, setValue] = useState("")
  const [error, setError] = useState("")
  const [shownKey, setShownKey] = useState(0)
  const companyId = application.company_id

  async function openDialog() {
    if (!companyId) return
    setError("")
    setValue("")
    setOpen(true)
    try {
      const company = await api<{ icon_url?: string }>(`/v1/companies/${encodeURIComponent(companyId)}`)
      setValue(company.icon_url || "")
    } catch {}
  }

  async function save() {
    if (!companyId) return
    try {
      const company = await api<{ version: number }>(`/v1/companies/${encodeURIComponent(companyId)}`)
      await api(`/v1/companies/${encodeURIComponent(companyId)}`, { method: "PATCH", body: { version: company.version, icon_url: value.trim() } })
      forgetFavicon(companyId)
      setShownKey((k) => k + 1)
      setOpen(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not save")
    }
  }

  return (
    <>
      <button type="button" className="mt-1 rounded-full" aria-label="Change company icon" onClick={openDialog} disabled={!companyId}>
        <CompanyAvatar key={shownKey} company={application.company} companyId={companyId} className="size-9 text-sm" />
      </button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Company icon</DialogTitle>
            <DialogDescription>
              Paste an image URL to use for {application.company || "this company"}. Leave it empty to use the site&apos;s own favicon. It applies to every listing at this company.
            </DialogDescription>
          </DialogHeader>
          <Input placeholder="https://example.com/logo.png" value={value} onChange={(event) => setValue(event.target.value)} />
          {error && <p className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button onClick={save}>Save</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

export function DetailHeader({
  application,
  writable,
  canApprove,
  onSaveField,
  onArchive,
  onUnarchive,
  onDecide,
  onDelete,
}: {
  application: Application
  writable: boolean
  canApprove: boolean
  onSaveField: (patch: Record<string, string>) => void
  onArchive: () => void
  onUnarchive: () => void
  onDecide: (kind: "approve" | "reject", note: string) => Promise<void>
  onDelete: () => Promise<void>
}) {
  const [decision, setDecision] = useState<"approve" | "reject" | null>(null)
  const [note, setNote] = useState("")
  const [deleting, setDeleting] = useState(false)

  async function confirmDecision() {
    if (!decision) return
    await onDecide(decision, note)
    setDecision(null)
    setNote("")
  }

  return (
    <>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <ProgressMark {...markOf(application)} size={40} />
          {writable ? (
            <IconEditor application={application} />
          ) : (
            <CompanyAvatar company={application.company} companyId={application.company_id} className="mt-1 size-9 text-sm" />
          )}
          <div>
            <InlineText
              value={application.title || ""}
              placeholder="Untitled"
              disabled={!writable}
              className="text-xl font-semibold"
              inputClassName="h-9 text-xl font-semibold"
              onSave={(v) => onSaveField({ title: v })}
            />
            <p className="text-sm text-muted-foreground">{application.company || "Unknown company"}</p>
          </div>
        </div>
        <div className="flex items-center gap-1">
          <StatusEditor value={shown(application)} disabled={!writable} onSave={(v) => onSaveField({ status: v })} />
          {application.archived_at && <Badge variant="outline">Archived</Badge>}
          {shown(application) === "awaiting_review" &&
            (canApprove ? (
              <>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant="ghost" size="icon" aria-label="Approve" onClick={() => setDecision("approve")}>
                      <Check className="size-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Approve</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant="ghost" size="icon" aria-label="Reject" onClick={() => setDecision("reject")}>
                      <X className="size-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Reject</TooltipContent>
                </Tooltip>
              </>
            ) : (
              <span className="text-sm text-muted-foreground">Approve needs the approve scope.</span>
            ))}
          {writable && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" aria-label="More actions">
                  <MoreHorizontal className="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {application.archived_at ? (
                  <DropdownMenuItem onClick={onUnarchive}>Unarchive</DropdownMenuItem>
                ) : (
                  <DropdownMenuItem onClick={onArchive}>Archive</DropdownMenuItem>
                )}
                <DropdownMenuItem onClick={() => setDeleting(true)} variant="destructive">
                  Delete
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      </div>

      <Dialog open={decision != null} onOpenChange={(open) => !open && setDecision(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{decision === "approve" ? "Approve application" : "Reject application"}</DialogTitle>
            <DialogDescription>
              {decision === "approve"
                ? "Approve marks this application approved. It does not submit the form."
                : "Reject declines this prepared application and marks it withdrawn. Nothing is submitted."}
            </DialogDescription>
          </DialogHeader>
          <Textarea placeholder="Note" value={note} onChange={(event) => setNote(event.target.value)} />
          <DialogFooter>
            <Button variant="outline" onClick={() => setDecision(null)}>
              Cancel
            </Button>
            <Button variant={decision === "reject" ? "outline" : "default"} onClick={confirmDecision}>
              {decision === "approve" ? "Approve" : "Reject"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title="Delete application"
        description={`This permanently deletes ${application.title || "this application"} at ${application.company || "this company"}. This can't be undone.`}
        confirmLabel="Delete"
        danger
        onConfirm={onDelete}
      />
    </>
  )
}
