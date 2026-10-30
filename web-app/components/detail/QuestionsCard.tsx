import type { Question } from "@/lib/types"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { PreparedAnswers } from "@/components/PreparedAnswers"

export function QuestionsCard({ questions }: { questions: Question[] }) {
  if (questions.length === 0) return null
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Prepared answers</CardTitle>
      </CardHeader>
      <CardContent>
        <PreparedAnswers questions={questions} />
      </CardContent>
    </Card>
  )
}
