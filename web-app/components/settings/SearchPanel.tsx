"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { getPref, setPref } from "@/lib/settings"
import { parseSearch } from "@/lib/search"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"

const QUALIFIERS: [string, string, string][] = [
  ["status:", "status:applied", "Exact status: to_apply, applied, screening, interviewing, offer, rejected, and the rest"],
  ["tag:", "tag:remote", "Applications with this tag"],
  ["company:", "company:stripe", "Company name contains the text"],
  ["title:", "title:engineer", "Role title contains the text"],
  ["applied:", "applied:>2026-01-01", "Applied date. Use > >= < <= with a date, or a date prefix like applied:2026-03"],
]

export function SearchPanel({ close }: { close: () => void }) {
  const [on, setOn] = useState(getPref("searchSyntax") !== "off")
  const [sample, setSample] = useState("status:applied tag:remote acme")
  const router = useRouter()
  const parsed = parseSearch(sample)

  function toggle(next: boolean) {
    setOn(next)
    setPref("searchSyntax", next ? "on" : "off")
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between gap-4 rounded-md border p-3">
        <div className="flex flex-col">
          <span className="text-sm font-medium">Search syntax</span>
          <span className="text-xs text-muted-foreground">
            Lets the search bar read qualifiers like status:applied. Off treats everything you type as plain text.
          </span>
        </div>
        <Switch checked={on} onCheckedChange={toggle} aria-label="Enable search syntax" />
      </div>
      <div className="flex flex-col gap-2">
        <h3 className="text-sm font-medium">Free text</h3>
        <p className="text-sm text-muted-foreground">
          Words that are not qualifiers are matched loosely against company, title, status and tags, best match first.
          Qualifiers narrow the list first, and several can be combined.
        </p>
      </div>
      <div className={"flex flex-col gap-2 " + (on ? "" : "opacity-50")}>
        <h3 className="text-sm font-medium">Qualifiers</h3>
        <div className="divide-y rounded-md border text-sm">
          {QUALIFIERS.map(([key, example, help]) => (
            <div key={key} className="flex items-center gap-3 p-3">
              <div className="flex min-w-0 flex-1 flex-col">
                <code className="font-mono text-xs">{example}</code>
                <span className="text-xs text-muted-foreground">{help}</span>
              </div>
              <Button
                size="sm"
                variant="outline"
                disabled={!on}
                onClick={() => {
                  close()
                  router.push("/applications?q=" + encodeURIComponent(example))
                }}
              >
                Try it
              </Button>
            </div>
          ))}
        </div>
      </div>
      <div className="flex flex-col gap-2">
        <h3 className="text-sm font-medium">Test a search</h3>
        <Input aria-label="Sample search" value={sample} onChange={(e) => setSample(e.target.value)} className="font-mono" />
        <p className="text-xs text-muted-foreground">
          {parsed.qualifiers.length === 0 && !parsed.free && "Nothing to search."}
          {parsed.qualifiers.map((q) => `${q.key} ${q.op} ${q.value}`).join(" · ")}
          {parsed.qualifiers.length > 0 && parsed.free && " · "}
          {parsed.free && `text: ${parsed.free}`}
        </p>
      </div>
    </div>
  )
}
