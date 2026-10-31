"use client"

import { useState } from "react"
import { Check } from "lucide-react"
import type { Question } from "@/lib/types"
import { api } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { Badge } from "@/components/ui/badge"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group"
import { Item, ItemContent, ItemDescription, ItemFooter, ItemGroup, ItemTitle } from "@/components/ui/item"

// An unanswered question saves as a standing answer keyed by its prompt, so
// the next prepare of this or any other application fills it.
function Unanswered({ prompt }: { prompt: string }) {
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
      <ItemDescription className="flex items-center gap-1.5">
        <Check className="size-3.5 text-emerald-600" /> {answer} · saved to your profile, prepare again to apply it
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
      <InputGroup>
        <InputGroupInput aria-label={prompt} placeholder="Answer once, reuse everywhere" value={answer} onChange={(e) => setAnswer(e.target.value)} />
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
                <Unanswered prompt={q.prompt} />
              </ItemFooter>
            )}
          </Item>
        )
      })}
    </ItemGroup>
  )
}
