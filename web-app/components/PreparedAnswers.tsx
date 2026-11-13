"use client"

import { useState } from "react"
import Link from "next/link"
import { Check, Sparkles } from "lucide-react"
import type { Question } from "@/lib/types"
import { api } from "@adrianmross/appkit-ui"
import { useAuth } from "@adrianmross/appkit-ui"
import { Badge } from "@adrianmross/appkit-ui/ui/badge"
import { Button } from "@adrianmross/appkit-ui/ui/button"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupTextarea } from "@adrianmross/appkit-ui/ui/input-group"
import { Item, ItemContent, ItemDescription, ItemFooter, ItemGroup, ItemTitle } from "@adrianmross/appkit-ui/ui/item"

const humanize = (key: string) => key.replaceAll("_", " ").replace(/^./, (c) => c.toUpperCase())

// An unanswered question saves as a standing answer keyed by its prompt, so
// the next prepare of this or any other application fills it. Smart
// search's closest saved answer is offered, never saved on its own.
export function Unanswered({ prompt, q }: { prompt: string; q?: Question }) {
  const [answer, setAnswer] = useState("")
  const [state, setState] = useState<"" | "saved" | string>("")
  async function save() {
    if (!answer.trim()) return
    try {
      await api("/v1/standing", { method: "POST", body: { key: prompt, answer } })
      setState("saved")
    } catch (err) {
      setState(err instanceof Error ? err.message : "Could not save")
    }
  }
  if (state === "saved")
    return (
      <ItemDescription className="flex flex-wrap items-center gap-1.5">
        <Check className="size-3.5 text-emerald-600" /> Saved to your{" "}
        <Link href="/profile/questions" className="underline underline-offset-2">
          questions
        </Link>
        . Prepare again to fill it in here and on every other form that asks.
      </ItemDescription>
    )
  return (
    <form
      className="flex w-full flex-col gap-1"
      onSubmit={(e) => {
        e.preventDefault()
        void save()
      }}
    >
      {q?.suggested_answer && answer !== q?.suggested_answer && (
        <div className="flex items-start gap-2 rounded-md bg-muted/60 px-2.5 py-2 text-sm">
          <Sparkles className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
          <div className="min-w-0 flex-1">
            <p className="text-xs text-muted-foreground">Like your answer to “{humanize(q?.suggested_source || "")}”</p>
            <p className="line-clamp-3">{q?.suggested_answer}</p>
          </div>
          <Button type="button" size="xs" variant="outline" onClick={() => setAnswer(q?.suggested_answer || "")}>
            Use this
          </Button>
        </div>
      )}
      <InputGroup>
        <InputGroupTextarea
          aria-label={prompt}
          rows={answer.length > 80 ? 4 : 1}
          className="min-h-0"
          placeholder="Answer once, reuse everywhere"
          value={answer}
          onChange={(e) => setAnswer(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault()
              void save()
            }
          }}
        />
        <InputGroupAddon align="inline-end">
          {/* Not disabled when empty: a disabled control greys out the whole InputGroup. */}
          <InputGroupButton type="submit" variant="secondary">
            Save
          </InputGroupButton>
        </InputGroupAddon>
      </InputGroup>
      {state && <p className="text-xs text-destructive">{state}</p>}
    </form>
  )
}

const SOURCE: Record<string, string> = { standing: "From profile", form: "From form" }

export function PreparedAnswers({ questions }: { questions: Question[] }) {
  const { can } = useAuth()
  if (!questions.length) return <p className="text-sm text-muted-foreground">No prepared answers yet.</p>
  // Unanswered first: that is what needs attention.
  const sorted = [...questions].sort((a, b) => Number(!!b.unknown && !b.answer) - Number(!!a.unknown && !a.answer))
  return (
    <ItemGroup className="gap-2">
      {sorted.map((q, idx) => {
        const open = q.unknown && !q.answer
        return (
          <Item key={idx} variant={open ? "outline" : "muted"} size="sm" className="flex-wrap">
            <ItemContent>
              <ItemTitle className="flex-wrap">
                {q.prompt || "Question"}
                {open ? (
                  <Badge variant="outline">{q.required ? "Required" : "Optional"}</Badge>
                ) : q.answer ? (
                  q.source && SOURCE[q.source] && <Badge variant="secondary">{SOURCE[q.source]}</Badge>
                ) : (
                  <Badge variant="outline">Optional</Badge>
                )}
              </ItemTitle>
              {!open && <ItemDescription>{q.answer || "No answer"}</ItemDescription>}
            </ItemContent>
            {open && q.prompt && can("write") && (
              <ItemFooter className="basis-full">
                <Unanswered prompt={q.prompt || ""} q={q} />
              </ItemFooter>
            )}
          </Item>
        )
      })}
    </ItemGroup>
  )
}
