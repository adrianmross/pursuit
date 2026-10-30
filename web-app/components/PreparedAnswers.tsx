"use client"

import { useState } from "react"
import type { Question } from "@/lib/types"
import { api } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

// An unanswered question saves as a standing answer keyed by its prompt, so
// the next prepare of this or any other application fills it.
function Unanswered({ prompt }: { prompt: string }) {
  const [answer, setAnswer] = useState("")
  const [state, setState] = useState<"" | "saved" | string>("")
  async function save() {
    try {
      await api("/v1/standing", { method: "POST", body: { key: prompt, answer } })
      setState("saved")
    } catch (err) {
      setState(err instanceof Error ? err.message : "Could not save")
    }
  }
  if (state === "saved") return <div className="text-muted-foreground">{answer} · saved to your profile, prepare again to apply it</div>
  return (
    <form
      className="mt-1 flex gap-2"
      onSubmit={(e) => {
        e.preventDefault()
        void save()
      }}
    >
      <Input aria-label={prompt} placeholder="Answer once, reuse everywhere" value={answer} onChange={(e) => setAnswer(e.target.value)} />
      <Button type="submit" size="sm" disabled={!answer.trim()}>
        Save
      </Button>
      {state && <span className="text-destructive">{state}</span>}
    </form>
  )
}

export function PreparedAnswers({ questions }: { questions: Question[] }) {
  const { can } = useAuth()
  if (!questions.length) return <p className="text-sm text-muted-foreground">No prepared answers yet.</p>
  return (
    <div className="flex flex-col gap-2">
      {questions.map((q, idx) => (
        <div key={idx} className="rounded-md border p-2 text-sm">
          <div className="font-medium">{q.prompt || "Question"}</div>
          {q.unknown && !q.answer && q.prompt && can("write") ? (
            <Unanswered prompt={q.prompt} />
          ) : (
            <div className="text-muted-foreground">{q.answer || "No answer"}</div>
          )}
        </div>
      ))}
    </div>
  )
}
