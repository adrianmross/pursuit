import type { Question } from "@/lib/types"

export function PreparedAnswers({ questions }: { questions: Question[] }) {
  if (!questions.length) return <p className="text-sm text-muted-foreground">No prepared answers yet.</p>
  return (
    <div className="flex flex-col gap-2">
      {questions.map((q, idx) => (
        <div key={idx} className="rounded-md border p-2 text-sm">
          <div className="font-medium">{q.prompt || "Question"}</div>
          <div className="text-muted-foreground">{q.answer || "No answer"}</div>
        </div>
      ))}
    </div>
  )
}
