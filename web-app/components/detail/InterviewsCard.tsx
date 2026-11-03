"use client"

import { useEffect, useState } from "react"
import { NotebookPen } from "lucide-react"
import { api } from "@/lib/api-client"
import type { Interview, Page, Question } from "@/lib/types"
import { RoundsList, TaskList, useRounds, useTasks } from "@/components/Rounds"
import { PreparedAnswers } from "@/components/PreparedAnswers"
import { RoundItem } from "@/components/detail/RoundNotes"
import { SectionCard } from "@/components/detail/SectionCard"
import { Button } from "@/components/ui/button"
import { ItemGroup } from "@/components/ui/item"

function Heading({ children }: { children: React.ReactNode }) {
  return <h4 className="px-1 text-xs font-medium text-muted-foreground">{children}</h4>
}

// One card for everything that happens per stage. Each stage row opens a panel
// with that stage's rounds (each a markdown document of prep, transcript and
// questions), its tasks, and on Apply the form questions prepare found.
export function InterviewsCard({
  appId,
  current,
  writable,
  onChange,
}: {
  appId: string
  current: number
  writable: boolean
  onChange: () => void
}) {
  const [sel, setSel] = useState(current)
  const [expanded, setExpanded] = useState<number | null>(current)
  const [form, setForm] = useState<Question[]>([])
  const base = `/v1/applications/${encodeURIComponent(appId)}/rounds`
  const rounds = useRounds(appId, current, onChange)
  const tasks = useTasks(appId, current, onChange)

  useEffect(() => {
    rounds.fetchRounds().catch(() => {})
    tasks.fetchTasks().catch(() => {})
    api<Page<Question>>(`/v1/applications/${encodeURIComponent(appId)}/questions`)
      .then((page) => setForm(page.data || []))
      .catch(() => {})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [appId])

  // Matches the server: an unanswered required form question is an open Apply task.
  const openForm = form.filter((q) => q.required && !q.answer).length
  const patch = (item: Interview, body: Record<string, unknown>) =>
    rounds.run(() => api(`/v1/rounds/${encodeURIComponent(item.id)}`, { method: "PATCH", body: { version: item.version, ...body } }))
  const remove = (item: Interview) =>
    rounds.run(() => api(`/v1/rounds/${encodeURIComponent(item.id)}?version=${item.version}`, { method: "DELETE" }))
  // A stage with no stored round still has its implied one; writing notes on
  // it stores that round, done if the stage is reached.
  const startNotes = (key: string, piece: number) =>
    rounds.run(() => api(base, { method: "POST", body: { stage: key, done: piece <= current } }))

  function panel(i: number, key: string, name: string) {
    const stored = rounds.storedFor(rounds.list, key)
    return (
      <div className="flex flex-col gap-4">
        {i === 0 && form.length > 0 && (
          <section className="flex flex-col gap-2">
            <Heading>Form questions</Heading>
            <PreparedAnswers questions={form} />
          </section>
        )}
        {i > 0 && (
          <section className="flex flex-col gap-2">
            <Heading>Rounds</Heading>
            {stored.length > 0 ? (
              <ItemGroup className="gap-2">
                {stored.map((item, n) => (
                  <RoundItem key={item.id} item={item} index={n + 1} writable={writable} onPatch={patch} onDelete={remove} />
                ))}
              </ItemGroup>
            ) : writable ? (
              <Button variant="outline" size="sm" className="self-start" disabled={rounds.busy} onClick={() => startNotes(key, i)}>
                <NotebookPen />
                Take notes on {name}
              </Button>
            ) : (
              <p className="px-1 text-[13px] text-muted-foreground">No notes at {name}.</p>
            )}
          </section>
        )}
        <section className="flex flex-col gap-2">
          <Heading>Tasks</Heading>
          <TaskList stageKey={key} name={name} tasks={tasks.forStage(key)} api={tasks} autoFocus={false} />
        </section>
      </div>
    )
  }

  return (
    <SectionCard id="stages" title="Rounds and tasks" summary={rounds.summary()}>
      <RoundsList
        rounds={rounds}
        tasks={tasks}
        stage={current}
        sel={sel}
        onSelect={setSel}
        expanded={expanded}
        onExpand={setExpanded}
        extraOpen={(i) => (i === 0 ? openForm : 0)}
        panel={panel}
      />
    </SectionCard>
  )
}
