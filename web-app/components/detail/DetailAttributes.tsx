"use client"

import { useState } from "react"
import { ExternalLink, RefreshCw } from "lucide-react"
import { sourceLabel, type Application, type CustomField } from "@/lib/types"
import { FIT_OPTIONS } from "@/components/AttributesForm"
import { InlineSelect, InlineText } from "@/components/InlineEdit"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Field, FieldError, FieldTitle } from "@/components/ui/field"
import { Spinner } from "@/components/ui/spinner"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

function Name({ children }: { children: React.ReactNode }) {
  return <FieldTitle className="text-xs font-normal text-muted-foreground">{children}</FieldTitle>
}

function ReadOnly({ label: name, value }: { label: string; value?: string | null }) {
  if (!value) return null
  return (
    <Field className="gap-0.5">
      <Name>{name}</Name>
      <span className="text-sm">{value}</span>
    </Field>
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
        <Field className="gap-0.5">
          <Name>Location</Name>
          <InlineText
            value={application.location || ""}
            disabled={!writable}
            className="text-sm"
            inputClassName="h-7 text-sm"
            onSave={(v) => onSaveField({ location: v })}
          />
        </Field>
        <Field className="gap-0.5">
          <Name>Salary</Name>
          <InlineText
            value={application.salary || ""}
            disabled={!writable}
            className="text-sm"
            inputClassName="h-7 text-sm"
            onSave={(v) => onSaveField({ salary: v })}
          />
        </Field>
        <Field className="gap-0.5">
          <Name>Fit</Name>
          <InlineSelect
            value={application.fit || ""}
            display={application.fit || "—"}
            options={FIT_OPTIONS.map((f) => [f, f] as [string, string])}
            disabled={!writable}
            className="text-sm"
            onSave={(v) => onSaveField({ fit: v })}
          />
        </Field>
        <ReadOnly label="Source" value={sourceLabel(application.source) || application.source} />
        <ReadOnly label="Applied" value={application.applied_date} />
        <Field className="col-span-2 gap-0.5 sm:col-span-3">
          <Name>Posting URL</Name>
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
                    size="icon-xs"
                    aria-label="Re-check link"
                    onClick={recheck}
                    disabled={recheckBusy}
                  >
                    {recheckBusy ? <Spinner className="size-3.5" /> : <RefreshCw className="size-3.5" />}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Re-check link</TooltipContent>
              </Tooltip>
            )}
          </div>
        </Field>
        {application.next_step && (
          <Field className="col-span-2 gap-0.5 sm:col-span-3">
            <Name>Next step</Name>
            <span className="text-sm">{application.next_step}</span>
          </Field>
        )}
        {(application.tags || []).length > 0 && (
          <div className="col-span-2 flex items-center gap-1.5 sm:col-span-3">
            {application.tags!.map((tag) => (
              <Badge key={tag} variant="secondary">
                {tag}
              </Badge>
            ))}
          </div>
        )}
        {fields.map((f) => (
          <Field key={f.id} className="gap-0.5">
            <Name>{f.name}</Name>
            <InlineText
              value={application.custom_fields?.[f.name] || ""}
              disabled={!writable}
              className="text-sm"
              inputClassName="h-7 text-sm"
              onSave={(v) => onSaveCustomFieldValue(f.id, v)}
            />
          </Field>
        ))}
        {fieldError && <FieldError className="col-span-2 sm:col-span-3">{fieldError}</FieldError>}
      </CardContent>
    </Card>
  )
}
