import { useEffect, useState } from "react"
import { Link, useParams } from "react-router-dom"
import { api, ApiError } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { label, type Application, type Artifact, type Event, type Page, type Question } from "@/lib/types"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
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

export function DetailPage() {
  const { id = "" } = useParams()
  const { can } = useAuth()
  const [application, setApplication] = useState<Application | null>(null)
  const [events, setEvents] = useState<Event[]>([])
  const [artifacts, setArtifacts] = useState<Artifact[]>([])
  const [questions, setQuestions] = useState<Question[]>([])
  const [error, setError] = useState<string | null>(null)
  const [decision, setDecision] = useState<"approve" | "reject" | null>(null)
  const [note, setNote] = useState("")
  const [recheckBusy, setRecheckBusy] = useState(false)

  async function load() {
    try {
      const [app, eventsPage, artifactsPage, questionsPage] = await Promise.all([
        api<Application>(`/v1/applications/${encodeURIComponent(id)}`),
        api<Page<Event>>(`/v1/applications/${encodeURIComponent(id)}/events?limit=100`),
        api<Page<Artifact>>(`/v1/applications/${encodeURIComponent(id)}/artifacts`),
        api<Page<Question>>(`/v1/applications/${encodeURIComponent(id)}/questions`),
      ])
      setApplication(app)
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
    setRecheckBusy(true)
    try {
      await api(`/v1/applications/${encodeURIComponent(id)}/check-link`, { method: "POST", body: {} })
      await load()
    } finally {
      setRecheckBusy(false)
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

  if (error) return <p className="text-sm text-destructive">{error}</p>
  if (!application) return <p className="text-sm text-muted-foreground">Loading…</p>

  const posting = safeURL(application.posting_url)

  return (
    <div className="flex flex-col gap-4">
      <Link to="/" className="text-sm text-muted-foreground hover:text-foreground">
        ← Pipeline
      </Link>

      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">{application.title || "Untitled"}</h1>
          <p className="text-sm text-muted-foreground">{application.company || "Unknown company"}</p>
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
        </div>
      </div>

      <Card>
        <CardContent className="flex flex-col gap-3 pt-6">
          <div className="flex flex-wrap gap-3 text-sm text-muted-foreground">
            {posting ? (
              <a href={posting} target="_blank" rel="noopener noreferrer" className="hover:text-foreground">
                Open posting
              </a>
            ) : (
              "No posting link"
            )}
            {application.location}
            {application.salary}
            {application.fit && `Fit ${application.fit}`}
            {application.source}
            {application.applied_date && `Applied ${application.applied_date}`}
          </div>
          {application.next_step && (
            <p className="text-sm">
              <strong>Next: </strong>
              {application.next_step}
            </p>
          )}
          {(application.tags || []).length > 0 && (
            <div className="flex gap-1.5">
              {application.tags!.map((tag) => (
                <Badge key={tag} variant="secondary">
                  {tag}
                </Badge>
              ))}
            </div>
          )}
          {can("write") ? (
            <Button size="sm" variant="outline" className="self-start" onClick={recheckLink} disabled={recheckBusy}>
              Re-check link
            </Button>
          ) : (
            <p className="text-sm text-muted-foreground">Re-checking this posting needs a token with write.</p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Notes</CardTitle>
        </CardHeader>
        <CardContent>
          {application.notes ? <p className="text-sm">{application.notes}</p> : <p className="text-sm text-muted-foreground">No notes.</p>}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Prepared answers</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          {questions.length === 0 ? (
            <p className="text-sm text-muted-foreground">No prepared answers yet.</p>
          ) : (
            questions.map((q, idx) => (
              <div key={idx} className="rounded-md border p-2 text-sm">
                <div className="font-medium">{q.prompt || "Question"}</div>
                <div className="text-muted-foreground">{q.answer || "No answer"}</div>
              </div>
            ))
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Artifacts</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          {artifacts.length === 0 ? (
            <p className="text-sm text-muted-foreground">No artifacts.</p>
          ) : (
            artifacts.map((a) => (
              <div key={a.id} className="flex items-center justify-between rounded-md border p-2 text-sm">
                <span>{a.filename || a.kind}</span>
                <span className="text-muted-foreground">{a.kind}</span>
              </div>
            ))
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">History</CardTitle>
        </CardHeader>
        <CardContent>
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
      </Card>

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
    </div>
  )
}
