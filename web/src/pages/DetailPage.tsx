import { useEffect, useState } from "react"
import { Link, useNavigate, useParams } from "react-router-dom"
import { Archive, Check, ChevronDown, ExternalLink, Pencil, RefreshCw, Trash2, X } from "lucide-react"
import { api, ApiError } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { label, type Application, type Artifact, type Event, type Page, type Question } from "@/lib/types"
import { CompanyAvatar } from "@/components/CompanyAvatar"
import { AttributesForm, EMPTY_ATTRIBUTES, type AttributesFormState } from "@/components/AttributesForm"
import { ConfirmDialog } from "@/components/ConfirmDialog"
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
import { Textarea } from "@/components/ui/textarea"

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

function Field({ label: name, value }: { label: string; value?: string | null }) {
  if (!value) return null
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-muted-foreground">{name}</span>
      <span className="text-sm">{value}</span>
    </div>
  )
}

function toFormState(app: Application): AttributesFormState {
  return {
    company: app.company || "",
    title: app.title || "",
    location: app.location || "",
    salary: app.salary || "",
    fit: app.fit || "",
    posting_url: app.posting_url || "",
    status: app.status,
  }
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
  const [archiveBusy, setArchiveBusy] = useState(false)
  const [deleting, setDeleting] = useState(false)

  const [editing, setEditing] = useState(isNew)
  const [attrs, setAttrs] = useState<AttributesFormState>(EMPTY_ATTRIBUTES)
  const [attrsBusy, setAttrsBusy] = useState(false)
  const [attrsError, setAttrsError] = useState<string | null>(null)

  const [notesDraft, setNotesDraft] = useState("")
  const [notesBusy, setNotesBusy] = useState(false)

  async function load() {
    if (isNew) return
    try {
      const [app, eventsPage, artifactsPage, questionsPage] = await Promise.all([
        api<Application>(`/v1/applications/${encodeURIComponent(id)}`),
        api<Page<Event>>(`/v1/applications/${encodeURIComponent(id)}/events?limit=100`),
        api<Page<Artifact>>(`/v1/applications/${encodeURIComponent(id)}/artifacts`),
        api<Page<Question>>(`/v1/applications/${encodeURIComponent(id)}/questions`),
      ])
      setApplication(app)
      setAttrs(toFormState(app))
      setNotesDraft(app.notes || "")
      setEvents(eventsPage.data || [])
      setArtifacts(artifactsPage.data || [])
      setQuestions(questionsPage.data || [])
      setError(null)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load application")
    }
  }

  useEffect(() => {
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

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
    setArchiveBusy(true)
    try {
      await api(`/v1/applications/${encodeURIComponent(application.id)}`, {
        method: "PATCH",
        body: { version: application.version, status: "withdrawn" },
      })
      await load()
    } finally {
      setArchiveBusy(false)
    }
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

  async function saveAttributes() {
    setAttrsBusy(true)
    setAttrsError(null)
    try {
      if (isNew) {
        const created = await api<Application>("/v1/applications", {
          method: "POST",
          body: { ...attrs, notes: notesDraft },
        })
        navigate(`/app/${encodeURIComponent(created.id)}`, { replace: true })
        return
      }
      if (!application) return
      await api<Application>(`/v1/applications/${encodeURIComponent(application.id)}`, {
        method: "PATCH",
        body: { version: application.version, ...attrs },
      })
      setEditing(false)
      await load()
    } catch (err) {
      setAttrsError(err instanceof ApiError ? err.message : "Failed to save")
    } finally {
      setAttrsBusy(false)
    }
  }

  function cancelEdit() {
    if (application) setAttrs(toFormState(application))
    setAttrsError(null)
    setEditing(false)
  }

  async function saveNotes() {
    if (!application) return
    setNotesBusy(true)
    try {
      await api<Application>(`/v1/applications/${encodeURIComponent(application.id)}`, {
        method: "PATCH",
        body: { version: application.version, notes: notesDraft },
      })
      await load()
    } finally {
      setNotesBusy(false)
    }
  }

  if (isNew) {
    return (
      <div className="mx-auto flex max-w-3xl flex-col gap-4">
        <Link to="/" className="text-sm text-muted-foreground hover:text-foreground">
          ← Pipeline
        </Link>
        <h1 className="text-xl font-semibold">Add application</h1>
        <Card>
          <CardContent className="pt-6">
            <AttributesForm value={attrs} onChange={setAttrs} showCompany />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Notes</CardTitle>
          </CardHeader>
          <CardContent>
            <Textarea
              className="min-h-40"
              placeholder="Anything worth remembering about this one…"
              value={notesDraft}
              onChange={(e) => setNotesDraft(e.target.value)}
            />
          </CardContent>
        </Card>
        {attrsError && <p className="text-sm text-destructive">{attrsError}</p>}
        <div className="flex gap-2">
          <Button onClick={saveAttributes} disabled={attrsBusy || !attrs.company.trim()}>
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

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-4">
      <Link to="/" className="text-sm text-muted-foreground hover:text-foreground">
        ← Pipeline
      </Link>

      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <CompanyAvatar company={application.company} className="mt-1 size-9 text-sm" />
          <div>
            <h1 className="text-xl font-semibold">{application.title || "Untitled"}</h1>
            <p className="text-sm text-muted-foreground">{application.company || "Unknown company"}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant="outline">{label(application.status)}</Badge>
          {application.status === "awaiting_review" &&
            (can("approve") ? (
              <>
                <Button size="sm" onClick={() => setDecision("approve")}>
                  Approve
                </Button>
                <Button size="sm" variant="destructive" onClick={() => setDecision("reject")}>
                  Reject
                </Button>
              </>
            ) : (
              <span className="text-sm text-muted-foreground">Approve needs the approve scope.</span>
            ))}
          {can("write") && !editing && (
            <>
              <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
                <Pencil /> Edit
              </Button>
              {application.status !== "withdrawn" && (
                <Button size="sm" variant="outline" onClick={archive} disabled={archiveBusy}>
                  <Archive /> Archive
                </Button>
              )}
              <Button size="sm" variant="outline" onClick={() => setDeleting(true)}>
                <Trash2 /> Delete
              </Button>
            </>
          )}
        </div>
      </div>

      <Card>
        <CardContent className="pt-6">
          {editing ? (
            <div className="flex flex-col gap-3">
              <AttributesForm value={attrs} onChange={setAttrs} />
              {attrsError && <p className="text-sm text-destructive">{attrsError}</p>}
              <div className="flex gap-2">
                <Button size="sm" onClick={saveAttributes} disabled={attrsBusy}>
                  <Check /> Save
                </Button>
                <Button size="sm" variant="outline" onClick={cancelEdit}>
                  <X /> Cancel
                </Button>
              </div>
            </div>
          ) : (
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
              <Field label="Location" value={application.location} />
              <Field label="Salary" value={application.salary} />
              <Field label="Fit" value={application.fit} />
              <Field label="Source" value={application.source} />
              <Field label="Applied" value={application.applied_date} />
              <div className="flex flex-col gap-0.5">
                <span className="text-xs text-muted-foreground">Posting</span>
                <span className="flex items-center gap-1.5">
                  {posting ? (
                    <a
                      href={posting}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-1 text-sm hover:underline"
                    >
                      <ExternalLink className="size-3.5" /> Open posting
                    </a>
                  ) : (
                    <span className="text-sm text-muted-foreground">No link</span>
                  )}
                  {can("write") && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-6"
                      aria-label="Re-check link"
                      onClick={recheckLink}
                      disabled={recheckBusy}
                    >
                      <RefreshCw className="size-3.5" />
                    </Button>
                  )}
                </span>
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
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Notes</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          <Textarea
            className="min-h-40"
            placeholder="Anything worth remembering about this one…"
            value={notesDraft}
            onChange={(e) => setNotesDraft(e.target.value)}
            disabled={!can("write")}
          />
          {notesDirty && can("write") && (
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
