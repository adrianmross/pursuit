import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { api, ApiError } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import type { Application, Page, Question } from "@/lib/types"
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

function QuestionList({ rows }: { rows: Question[] }) {
  if (!rows.length) return <p className="text-sm text-muted-foreground">No prepared answers yet.</p>
  return (
    <div className="flex flex-col gap-2">
      {rows.map((row, idx) => (
        <div key={idx} className="rounded-md border p-2 text-sm">
          <div className="font-medium">{row.prompt || "Question"}</div>
          <div className="text-muted-foreground">{row.answer || "No answer"}</div>
        </div>
      ))}
    </div>
  )
}

function QueueCard({ row, onDecided }: { row: Application; onDecided: () => void }) {
  const { can } = useAuth()
  const [questions, setQuestions] = useState<Question[] | null>(null)
  const [decision, setDecision] = useState<"approve" | "reject" | null>(null)
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    api<Page<Question>>(`/v1/applications/${encodeURIComponent(row.id)}/questions`)
      .then((page) => setQuestions(page.data || []))
      .catch(() => setQuestions([]))
  }, [row.id])

  async function confirm() {
    if (!decision) return
    setBusy(true)
    try {
      await api(decision === "approve" ? "/v1/approvals" : "/v1/approvals/reject", {
        method: "POST",
        body: { ids: [row.id], note },
      })
      setDecision(null)
      setNote("")
      onDecided()
    } finally {
      setBusy(false)
    }
  }

  const posting = safeURL(row.posting_url)

  return (
    <Card>
      <CardHeader className="flex-row items-start justify-between">
        <div>
          <CardTitle>
            <Link to={`/app/${encodeURIComponent(row.id)}`} className="hover:underline">
              {row.title || "Untitled"}
            </Link>
          </CardTitle>
          <p className="text-sm text-muted-foreground">{row.company || "Unknown company"}</p>
        </div>
        {can("approve") ? (
          <div className="flex gap-2">
            <Button size="sm" onClick={() => setDecision("approve")}>
              Approve
            </Button>
            <Button size="sm" variant="destructive" onClick={() => setDecision("reject")}>
              Reject
            </Button>
          </div>
        ) : (
          <span className="text-sm text-muted-foreground">Approve needs the approve scope.</span>
        )}
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {posting && (
          <a href={posting} target="_blank" rel="noopener noreferrer" className="text-sm text-muted-foreground hover:text-foreground">
            Posting
          </a>
        )}
        {row.notes && <p className="text-sm">{row.notes}</p>}
        {questions == null ? (
          <p className="text-sm text-muted-foreground">Loading prepared answers…</p>
        ) : (
          <QuestionList rows={questions} />
        )}
      </CardContent>

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
            <Button variant={decision === "reject" ? "destructive" : "default"} onClick={confirm} disabled={busy}>
              {decision === "approve" ? "Approve" : "Reject"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}

export function QueuePage() {
  const { can } = useAuth()
  const [rows, setRows] = useState<Application[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function load() {
    try {
      const page = await api<Page<Application>>("/v1/applications?status=awaiting_review&sort=-updated_at&limit=50")
      setRows(page.data || [])
      setError(null)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load queue")
    }
  }

  useEffect(() => {
    void load()
  }, [])

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h1 className="text-xl font-semibold">Approval queue</h1>
        <p className="text-sm text-muted-foreground">Agent-prepared applications waiting for you. Approve only records approval.</p>
      </div>
      {!can("approve") && (
        <p className="text-sm text-muted-foreground">This token can view the queue. Approve and Reject need the approve scope.</p>
      )}
      {error && <p className="text-sm text-destructive">{error}</p>}
      {rows == null ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : rows.length === 0 ? (
        <Card>
          <CardContent className="py-6 text-center text-muted-foreground">Nothing is waiting for review.</CardContent>
        </Card>
      ) : (
        <div className="flex flex-col gap-3">
          {rows.map((row) => (
            <QueueCard key={row.id} row={row} onDecided={load} />
          ))}
        </div>
      )}
    </div>
  )
}
