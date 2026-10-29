import { useState } from "react"
import { Copy, Check } from "lucide-react"

// A copyable `pursuit` CLI command, for help text and empty states.
export function CommandBlock({ command }: { command: string }) {
  const [copied, setCopied] = useState(false)

  async function onCopy() {
    await navigator.clipboard.writeText(command)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  return (
    <div className="flex items-start gap-3 rounded-md bg-foreground px-4 py-3 font-mono text-xs text-background">
      <pre className="flex-1 overflow-x-auto whitespace-pre">{command}</pre>
      <button
        type="button"
        onClick={onCopy}
        aria-label="Copy command"
        className="shrink-0 rounded-sm border border-background/40 px-1.5 py-0.5 text-[11px] text-background hover:bg-background/10"
      >
        {copied ? <Check className="size-3" /> : <Copy className="size-3" />}
      </button>
    </div>
  )
}
