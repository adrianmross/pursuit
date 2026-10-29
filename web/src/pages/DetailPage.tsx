import { useEffect, useState } from "react"
import { Link, useNavigate, useParams } from "react-router-dom"
import { ArrowLeft, Check, ChevronDown, ExternalLink, MoreHorizontal, RefreshCw, X } from "lucide-react"
import { api, ApiError } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { label, statusTone, type Application, type Artifact, type CustomField, type Event, type Page, type Question } from "@/lib/types"
import { CompanyAvatar } from "@/components/CompanyAvatar"
import { AttributesForm, EMPTY_ATTRIBUTES, FIT_OPTIONS, STATUS_OPTIONS, type AttributesFormState } from "@/components/AttributesForm"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { InlineSelect, InlineText } from "@/components/InlineEdit"
import { PreparedAnswers } from "@/components/PreparedAnswers"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Textarea } from "@/components/ui/textarea"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

function safeURL(raw?: string) {
  if (!raw) return ""
  try {
    const url = new URL(raw)
    return url.protocol === "http:" || url.protocol === "https:" ? url.href : ""
  } catch {
    return ""
  }
}

function fmt(value?: string) {
  if (!value) return "—"
  const match = value.match(/^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})/)
  if (!match) return value
  const date = new Date(`${match[1]}T${match[2]}Z`)
  if (Number.isNaN(date.getTime())) return match[1]
  return date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" })
}

export function DetailPage() {
  const { id = "" } = useParams()
  const isNew = id === "new"
  const navigate = useNavigate()
  const { can } = useAuth()
  const [application, setApplication] = useState<Application | null>(null)
  const [events, setEvents] = useState<Event[]>([])
  const [artifacts, setArtifacts] = useState<Artifact[]>([])
  const [questions, setQuestions] = useState<Question[]>([])
  const [error, setError] = useState<string | null>(null)
  const [decision, setDecision] = useState<"approve" | "reject" | null>(null)
  const [note, setNote] = useState("")
  const [recheckBusy, setRecheckBusy] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [fieldError, setFieldError] = useState<string | null>(null)

  const [createAttrs, setCreateAttrs] = useState<AttributesFormState>(EMPTY_ATTRIBUTES)
  const [createNotes, setCreateNotes] = useState("")
  const [createBusy, setCreateBusy] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

  const [notesDraft, setNotesDraft] = useState("")
  const [notesBusy, setNotesBusy] = useState(false)
  const [fields, setFields] = useState<CustomField[]>([])

  async function load() {
    if (isNew) return
    try {
      const [app, eventsPage, artifactsPage, questionsPage, fieldsPage] = await Promise.all([
        api<Application>(`/v1/applications/${encodeURIComponent(id)}`),
        api<Page<Event>>(`/v1/applications/${encodeURIComponent(id)}/events?limit=100`),
        api<Page<Artifact>>(`/v1/applications/${encodeURIComponent(id)}/artifacts`),
        api<Page<Question>>(`/v1/applications/${encodeURIComponent(id)}/questions`),
        api<Page<CustomField>>(`/v1/fields?limit=100`),
      ])
      setApplication(app)
      setNotesDraft(app.notes || "")
      setEvents(eventsPage.data || [])
      setArtifacts(artifactsPage.data || [])
      setQuestions(questionsPage.data || [])
      setFields(fieldsPage.data || [])
      setError(null)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load application")
    }
  }

  useEffect(() => {
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  async function saveField(patch: Record<string, string>) {
    if (!application) return
    setFieldError(null)
    try {
      const updated = await api<Application>(`/v1/applications/${encodeURIComponent(application.id)}`, {
        method: "PATCH",
        body: { version: application.version, ...patch },
      })
      setApplication(updated)
    } catch (err) {
      setFieldError(err instanceof ApiError ? err.message : "Failed to save")
    }
  }

  async function saveCustomFieldValue(fieldID: string, value: string) {
    if (!application) return
    try {
      const updated = await api<Application>(
        `/v1/applications/${encodeURIComponent(application.id)}/fields/${encodeURIComponent(fieldID)}`,
        { method: "POST", body: { value } },
      )
      setApplication(updated)
    } catch (err) {
      setFieldError(err instanceof ApiError ? err.message : "Failed to save")
    }
  }

  async function recheckLink() {
    if (!application) return
    setRecheckBusy(true)
    try {
      await api(`/v1/applications/${encodeURIComponent(application.id)}/check-link`, { method: "POST", body: {} })
      await load()
    } finally {
      setRecheckBusy(false)
    }
  }

  async function archive() {
    if (!application) return
    await api(`/v1/applications/${encodeURIComponent(application.id)}/archive`, {
      method: "POST",
      body: { version: application.version },
    })
    await load()
  }

  async function unarchive() {
    if (!application) return
    await api(`/v1/applications/${encodeURIComponent(application.id)}/unarchive`, {
      method: "POST",
      body: { version: application.version },
    })
    await load()
  }

  async function confirmDecision() {
    if (!decision || !application) return
    await api(decision === "approve" ? "/v1/approvals" : "/v1/approvals/reject", {
      method: "POST",
      body: { ids: [application.id], note },
    })
    setDecision(null)
    setNote("")
    await load()
  }

  async function saveNotes() {
    if (!application) return
    setNotesBusy(true)
    try {
      const updated = await api<Application>(`/v1/applications/${encodeURIComponent(application.id)}`, {
        method: "PATCH",
        body: { version: application.version, notes: notesDraft },
      })
      setApplication(updated)
    } finally {
      setNotesBusy(false)
    }
  }

  async function createApplication() {
    setCreateBusy(true)
    setCreateError(null)
    try {
      const created = await api<Application>("/v1/applications", {
        method: "POST",
        body: { ...createAttrs, notes: createNotes },
      })
      navigate(`/app/${encodeURIComponent(created.id)}`, { replace: true })
    } catch (err) {
      setCreateError(err instanceof ApiError ? err.message : "Failed to save")
    } finally {
      setCreateBusy(false)
    }
  }

  if (isNew) {
    return (
      <div className="mx-auto flex max-w-3xl flex-col gap-4">
        <Link to="/" aria-label="Back to pipeline" className="text-muted-foreground hover:text-foreground">
          <ArrowLeft className="size-4" />
        </Link>
        <h1 className="text-xl font-semibold">Add application</h1>
        <Card>
          <CardContent className="pt-6">
            <AttributesForm value={createAttrs} onChange={setCreateAttrs} showCompany />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Notes</CardTitle>
          </CardHeader>
          <CardContent>
            <Textarea
              className="min-h-40 border-transparent bg-transparent px-0 shadow-none focus-visible:border-transparent focus-visible:ring-1 focus-visible:ring-border"
              placeholder="Anything worth remembering about this one…"
              value={createNotes}
              onChange={(e) => setCreateNotes(e.target.value)}
            />
          </CardContent>
        </Card>
        {createError && <p className="text-sm text-destructive">{createError}</p>}
        <div className="flex gap-2">
          <Button onClick={createApplication} disabled={createBusy || !createAttrs.company.trim()}>
            Add application
          </Button>
          <Button variant="outline" onClick={() => navigate("/")}>
            Cancel
          </Button>
        </div>
      </div>
    )
  }

  if (error) return <p className="text-sm text-destructive">{error}</p>
  if (!application) return <p className="text-sm text-muted-foreground">Loading…</p>

  const posting = safeURL(application.posting_url)
  const notesDirty = notesDraft !== (application.notes || "")
  const writable = can("write")

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-4">
      <Link to="/" aria-label="Back to pipeline" className="text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" />
      </Link>

      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <CompanyAvatar company={application.company} className="mt-1 size-9 text-sm" />
          <div>
            <InlineText
              value={application.title || ""}
              placeholder="Untitled"
              disabled={!writable}
              className="text-xl font-semibold"
              inputClassName="h-9 text-xl font-semibold"
              onSave={(v) => saveField({ title: v })}
            />
            <p className="text-sm text-muted-foreground">{application.company || "Unknown company"}</p>
          </div>
        </div>
        <div className="flex items-center gap-1">
          <Badge
            variant="secondary"
            className={statusTone(application.status) === "win" ? "bg-brand-gold text-[#141615]" : undefined}
          >
            {label(application.status)}
          </Badge>
          {application.archived_at && <Badge variant="outline">Archived</Badge>}
          {application.status === "awaiting_review" &&
            (can("approve") ? (
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
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label="Reject"
                      className="text-destructive hover:text-destructive"
                      onClick={() => setDecision("reject")}
                    >
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
                  <DropdownMenuItem onClick={unarchive}>Unarchive</DropdownMenuItem>
                ) : (
                  <DropdownMenuItem onClick={archive}>Archive</DropdownMenuItem>
                )}
                <DropdownMenuItem onClick={() => setDeleting(true)} variant="destructive">
                  Delete
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      </div>

      <Card>
        <CardContent className="grid grid-cols-2 gap-4 pt-6 sm:grid-cols-3">
          <div className="flex flex-col gap-0.5">
            <span className="text-xs text-muted-foreground">Location</span>
            <InlineText
              value={application.location || ""}
              disabled={!writable}
              className="text-sm"
              inputClassName="h-7 text-sm"
              onSave={(v) => saveField({ location: v })}
            />
          </div>
          <div className="flex flex-col gap-0.5">
            <span className="text-xs text-muted-foreground">Salary</span>
            <InlineText
              value={application.salary || ""}
              disabled={!writable}
              className="text-sm"
              inputClassName="h-7 text-sm"
              onSave={(v) => saveField({ salary: v })}
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
              onSave={(v) => saveField({ fit: v })}
            />
          </div>
          <Field label="Source" value={application.source} />
          <Field label="Applied" value={application.applied_date} />
          <div className="flex flex-col gap-0.5">
            <span className="text-xs text-muted-foreground">Status</span>
            <InlineSelect
              value={application.status}
              display={label(application.status)}
              options={STATUS_OPTIONS}
              disabled={!writable}
              className="text-sm"
              onSave={(v) => saveField({ status: v })}
            />
          </div>
          <div className="col-span-2 flex flex-col gap-0.5 sm:col-span-3">
            <span className="text-xs text-muted-foreground">Posting URL</span>
            <div className="flex items-center gap-1.5">
              <InlineText
                value={application.posting_url || ""}
                placeholder="No link"
                disabled={!writable}
                className="text-sm"
                inputClassName="h-7 flex-1 text-sm"
                onSave={(v) => saveField({ posting_url: v })}
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
                      onClick={recheckLink}
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
                onSave={(v) => saveCustomFieldValue(f.id, v)}
              />
            </div>
          ))}
          {fieldError && (
            <p className="col-span-2 text-sm text-destructive sm:col-span-3">{fieldError}</p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Notes</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          <Textarea
            className="min-h-40 border-transparent bg-transparent px-0 shadow-none focus-visible:border-transparent focus-visible:ring-1 focus-visible:ring-border"
            placeholder="Anything worth remembering about this one…"
            value={notesDraft}
            onChange={(e) => setNotesDraft(e.target.value)}
            disabled={!writable}
          />
          {notesDirty && writable && (
            <div className="flex gap-2">
              <Button size="sm" onClick={saveNotes} disabled={notesBusy}>
                Save notes
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setNotesDraft(application.notes || "")}>
                Discard
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {questions.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Prepared answers</CardTitle>
          </CardHeader>
          <CardContent>
            <PreparedAnswers questions={questions} />
          </CardContent>
        </Card>
      )}

      {artifacts.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Artifacts</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            {artifacts.map((a) => (
              <div key={a.id} className="flex items-center justify-between rounded-md border p-2 text-sm">
                <span>{a.filename || a.kind}</span>
                <span className="text-muted-foreground">{a.kind}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      <Collapsible>
        <Card>
          <CollapsibleTrigger className="flex w-full items-center justify-between p-6 text-left">
            <CardTitle className="text-base">History ({events.length})</CardTitle>
            <ChevronDown className="size-4 text-muted-foreground" />
          </CollapsibleTrigger>
          <CollapsibleContent>
            <CardContent className="pt-0">
              {events.length === 0 ? (
                <p className="text-sm text-muted-foreground">No history yet.</p>
              ) : (
                <ol className="flex flex-col gap-3">
                  {events.map((e, idx) => (
                    <li key={idx} className="text-sm">
                      <div className="font-medium">{e.title || e.kind}</div>
                      <div className="text-muted-foreground">
                        {fmt(e.occurred_at)}
                        {e.actor?.name ? ` · ${e.actor.name}` : ""}
                      </div>
                      {(e.from_status || e.to_status) && (
                        <div>
                          {label(e.from_status)} → {label(e.to_status)}
                        </div>
                      )}
                      {e.body && <div className="text-muted-foreground">{e.body}</div>}
                    </li>
                  ))}
                </ol>
              )}
            </CardContent>
          </CollapsibleContent>
        </Card>
      </Collapsible>

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
            <Button variant={decision === "reject" ? "destructive" : "default"} onClick={confirmDecision}>
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
        onConfirm={async () => {
          await api(`/v1/applications/${encodeURIComponent(application.id)}`, { method: "DELETE" })
          navigate("/")
        }}
      />
    </div>
  )
}

function Field({ label: name, value }: { label: string; value?: string | null }) {
  if (!value) return null
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-muted-foreground">{name}</span>
      <span className="text-sm">{value}</span>
    </div>
  )
}
