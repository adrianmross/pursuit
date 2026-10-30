import { pursuitApi } from "@/lib/pursuit-api"
import type { Page, Question } from "@/lib/types"
import { QuestionsCard } from "@/components/detail/QuestionsCard"
import { SectionError } from "@/components/detail/SectionError"

export async function QuestionsSection({ id }: { id: string }) {
  let page: Page<Question> | null = null
  try {
    page = await pursuitApi<Page<Question>>(`/v1/applications/${encodeURIComponent(id)}/questions`)
  } catch {
    // fall through — page stays null
  }
  if (page == null) return <SectionError label="prepared answers" />
  return <QuestionsCard questions={page.data || []} />
}
