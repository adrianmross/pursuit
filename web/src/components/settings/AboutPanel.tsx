import { useEffect, useState } from "react"
import { api } from "@/lib/api"

export function AboutPanel() {
  const [version, setVersion] = useState("…")
  useEffect(() => {
    api<{ version: string }>("/version")
      .then((v) => setVersion(v.version))
      .catch(() => setVersion("unknown"))
  }, [])

  const rows: [string, string][] = [
    ["Version", version],
    ["Server", window.location.origin],
    ["Web build", import.meta.env.MODE],
  ]
  return (
    <div className="flex flex-col gap-4">
      <dl className="divide-y rounded-md border text-sm">
        {rows.map(([k, v]) => (
          <div key={k} className="flex justify-between gap-4 p-3">
            <dt className="text-muted-foreground">{k}</dt>
            <dd className="font-mono">{v}</dd>
          </div>
        ))}
      </dl>
      <div className="flex gap-4 text-sm">
        <a className="underline underline-offset-4" href="/reference">
          API reference
        </a>
        <a className="underline underline-offset-4" href="/docs">
          Docs
        </a>
        <a className="underline underline-offset-4" href="/llms.txt">
          llms.txt
        </a>
      </div>
    </div>
  )
}
