"use client"

import { useState } from "react"
import { ExternalLink, RefreshCw } from "lucide-react"
import { SOURCE_LABELS, type Application, type CustomField } from "@/lib/types"
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
  onSaveField: (patch: Record<string, string>) => Promise<unknown>
  onSaveCustomFieldValue: (fieldID: string, value: string) => Promise<unknown>
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

  // One editable cell per field the PATCH accepts.
  const text = (name: string, key: keyof Application & string, type?: "date") => (
    <Field className="gap-0.5">
      <Name>{name}</Name>
      <InlineText
        value={String(application[key] ?? "")}
        label={name}
        disabled={!writable}
        className="text-sm"
        type={type}
        onSave={(v) => onSaveField({ [key]: v })}
      />
    </Field>
  )

  return (
    <Card>
      <CardContent className="grid grid-cols-2 gap-4 pt-6 sm:grid-cols-3">
        {text("Location", "location")}
        {text("Salary", "salary")}
        <Field className="gap-0.5">
          <Name>Fit</Name>
          <InlineSelect
            value={application.fit || ""}
            label="Fit"
            options={FIT_OPTIONS.map((f) => [f, f] as [string, string])}
            disabled={!writable}
            className="text-sm"
            onSave={(v) => onSaveField({ fit: v })}
          />
        </Field>
        <Field className="gap-0.5">
          <Name>Source</Name>
          <InlineSelect
            value={application.source || ""}
            label="Source"
            options={Object.entries(SOURCE_LABELS)}
            disabled={!writable}
            className="text-sm"
            onSave={(v) => onSaveField({ source: v })}
          />
        </Field>
        {text("Applied", "applied_date", "date")}
        {text("Job type", "job_type")}
        {text("Contact email", "contact_email")}
        {text("Résumé version", "resume_version")}
        <Field className="col-span-2 gap-0.5 sm:col-span-3">
          <Name>Posting URL</Name>
          <div className="flex items-center gap-1.5">
            <InlineText
              value={application.posting_url || ""}
              placeholder="No link"
              label="Posting URL"
              disabled={!writable}
              className="flex-1 text-sm"
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
        <Field className="col-span-2 gap-0.5 sm:col-span-3">
          <Name>Next step</Name>
          <InlineText
            value={application.next_step || ""}
            label="Next step"
            disabled={!writable}
            className="text-sm"
            onSave={(v) => onSaveField({ next_step: v })}
          />
        </Field>
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
              label={f.name}
              disabled={!writable}
              className="text-sm"
              onSave={(v) => onSaveCustomFieldValue(f.id, v)}
            />
          </Field>
        ))}
        {fieldError && <FieldError className="col-span-2 sm:col-span-3">{fieldError}</FieldError>}
      </CardContent>
    </Card>
  )
}
