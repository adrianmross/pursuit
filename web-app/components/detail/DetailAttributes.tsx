"use client"

import { useState } from "react"
import { ExternalLink, RefreshCw } from "lucide-react"
import { sourceLabel, type Application, type CustomField } from "@/lib/types"
import { FIT_OPTIONS } from "@/components/AttributesForm"
import { InlineSelect, InlineText } from "@/components/InlineEdit"
import { Card, CardContent } from "@/components/ui/card"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { Button } from "@/components/ui/button"

function Field({ label: name, value }: { label: string; value?: string | null }) {
  if (!value) return null
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-muted-foreground">{name}</span>
      <span className="text-sm">{value}</span>
    </div>
  )
}

function safeURL(raw?: string) {
  if (!raw) return ""
  try {
    const url = new URL(raw)
    return url.protocol === "http:" || url.protocol === "https:" ? url.href : ""
  } catch {
    return ""
  }
}

export function DetailAttributes({
  application,
  fields,
  writable,
  onSaveField,
  onSaveCustomFieldValue,
  onRecheckLink,
  fieldError,
}: {
  application: Application
  fields: CustomField[]
  writable: boolean
  onSaveField: (patch: Record<string, string>) => void
  onSaveCustomFieldValue: (fieldID: string, value: string) => void
  onRecheckLink: () => Promise<void>
  fieldError: string | null
}) {
  const [recheckBusy, setRecheckBusy] = useState(false)
  const posting = safeURL(application.posting_url)

  async function recheck() {
    setRecheckBusy(true)
    try {
      await onRecheckLink()
    } finally {
      setRecheckBusy(false)
    }
  }

  return (
    <Card>
      <CardContent className="grid grid-cols-2 gap-4 pt-6 sm:grid-cols-3">
        <div className="flex flex-col gap-0.5">
          <span className="text-xs text-muted-foreground">Location</span>
          <InlineText
            value={application.location || ""}
            disabled={!writable}
            className="text-sm"
            inputClassName="h-7 text-sm"
            onSave={(v) => onSaveField({ location: v })}
          />
        </div>
        <div className="flex flex-col gap-0.5">
          <span className="text-xs text-muted-foreground">Salary</span>
          <InlineText
            value={application.salary || ""}
            disabled={!writable}
            className="text-sm"
            inputClassName="h-7 text-sm"
            onSave={(v) => onSaveField({ salary: v })}
          />
        </div>
        <div className="flex flex-col gap-0.5">
          <span className="text-xs text-muted-foreground">Fit</span>
          <InlineSelect
            value={application.fit || ""}
            display={application.fit || "—"}
            options={FIT_OPTIONS.map((f) => [f, f] as [string, string])}
            disabled={!writable}
            className="text-sm"
            onSave={(v) => onSaveField({ fit: v })}
          />
        </div>
        <Field label="Source" value={sourceLabel(application.source) || application.source} />
        <Field label="Applied" value={application.applied_date} />
        <div className="col-span-2 flex flex-col gap-0.5 sm:col-span-3">
          <span className="text-xs text-muted-foreground">Posting URL</span>
          <div className="flex items-center gap-1.5">
            <InlineText
              value={application.posting_url || ""}
              placeholder="No link"
              disabled={!writable}
              className="text-sm"
              inputClassName="h-7 flex-1 text-sm"
              onSave={(v) => onSaveField({ posting_url: v })}
            />
            {posting && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <a href={posting} target="_blank" rel="noopener noreferrer" aria-label="Open posting">
                    <ExternalLink className="size-3.5 text-muted-foreground hover:text-foreground" />
                  </a>
                </TooltipTrigger>
                <TooltipContent>Open posting</TooltipContent>
              </Tooltip>
            )}
            {writable && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-6"
                    aria-label="Re-check link"
                    onClick={recheck}
                    disabled={recheckBusy}
                  >
                    <RefreshCw className={`size-3.5 ${recheckBusy ? "animate-spin" : ""}`} />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Re-check link</TooltipContent>
              </Tooltip>
            )}
          </div>
        </div>
        {application.next_step && (
          <div className="col-span-2 flex flex-col gap-0.5 sm:col-span-3">
            <span className="text-xs text-muted-foreground">Next step</span>
            <span className="text-sm">{application.next_step}</span>
          </div>
        )}
        {(application.tags || []).length > 0 && (
          <div className="col-span-2 flex items-center gap-1.5 sm:col-span-3">
            {application.tags!.map((tag) => (
              <span key={tag} className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                {tag}
              </span>
            ))}
          </div>
        )}
        {fields.map((f) => (
          <div key={f.id} className="flex flex-col gap-0.5">
            <span className="text-xs text-muted-foreground">{f.name}</span>
            <InlineText
              value={application.custom_fields?.[f.name] || ""}
              disabled={!writable}
              className="text-sm"
              inputClassName="h-7 text-sm"
              onSave={(v) => onSaveCustomFieldValue(f.id, v)}
            />
          </div>
        ))}
        {fieldError && <p className="col-span-2 text-sm text-destructive sm:col-span-3">{fieldError}</p>}
      </CardContent>
    </Card>
  )
}
