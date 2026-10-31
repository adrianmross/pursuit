"use client"

import { useState } from "react"
import { Check, MoreHorizontal, X } from "lucide-react"
import { markOf, shown, type Application } from "@/lib/types"
import { CompanyAvatar } from "@/components/CompanyAvatar"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { InlineText } from "@/components/InlineEdit"
import { ProgressMark } from "@/components/ProgressMark"
import { StatusBadge } from "@/components/StatusBadge"
import { StatusChooser } from "@/components/StatusChooser"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Textarea } from "@/components/ui/textarea"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

export function DetailHeader({
  application,
  writable,
  canApprove,
  onSaveField,
  onArchive,
  onUnarchive,
  onDecide,
  onDelete,
  onAddRound,
}: {
  application: Application
  writable: boolean
  canApprove: boolean
  onSaveField: (patch: Record<string, string>) => void
  onArchive: () => void
  onUnarchive: () => void
  onDecide: (kind: "approve" | "reject", note: string) => Promise<void>
  onDelete: () => Promise<void>
  onAddRound: (stage: string) => Promise<void>
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
          <CompanyAvatar company={application.company} companyId={application.company_id} className="mt-1 size-9 text-sm" />
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
          {writable ? (
            <StatusChooser app={application} align="end" onStatus={(v) => onSaveField({ status: v })} onAddRound={onAddRound}>
              <button type="button" className="rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-label="Change status">
                <StatusBadge status={shown(application)} />
              </button>
            </StatusChooser>
          ) : (
            <StatusBadge status={shown(application)} />
          )}
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
