"use client"

import { useEffect, useState } from "react"
import { Check, Copy, Trash2 } from "lucide-react"
import { api } from "@/lib/api-client"
import type { Page, Token } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"

function Secret({ value }: { value: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <div className="flex flex-col gap-2 rounded-md border border-amber-500/50 bg-amber-500/10 p-3">
      <p className="text-xs">Copy this token now. It is shown once and cannot be recovered.</p>
      <div className="flex items-center gap-2">
        <code className="flex-1 overflow-x-auto rounded bg-background px-2 py-1.5 font-mono text-xs">{value}</code>
        <Button
          size="icon-sm"
          variant="outline"
          aria-label="Copy token"
          onClick={async () => {
            await navigator.clipboard.writeText(value)
            setCopied(true)
            setTimeout(() => setCopied(false), 1500)
          }}
        >
          {copied ? <Check /> : <Copy />}
        </Button>
      </div>
      <code className="text-[11px] text-muted-foreground">export PURSUIT_TOKEN={value}</code>
    </div>
  )
}

export function TokensPanel() {
  const [tokens, setTokens] = useState<Token[] | null>(null)
  const [name, setName] = useState("")
  const [scope, setScope] = useState("write")
  const [secret, setSecret] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  const load = () =>
    api<Page<Token>>("/v1/tokens?limit=100")
      .then((page) => setTokens((page.data || []).filter((t) => !t.revoked_at)))
      .catch((err) => setError(err.message))

  useEffect(() => {
    void load()
  }, [])

  async function create() {
    if (!name.trim()) return
    setBusy(true)
    setError("")
    try {
      const issued = await api<{ secret: string }>("/v1/tokens", {
        method: "POST",
        body: { name: name.trim(), scopes: scope === "write" ? ["read", "write"] : ["read"] },
      })
      setSecret(issued.secret)
      setName("")
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create the token")
    } finally {
      setBusy(false)
    }
  }

  async function revoke(id: string) {
    setError("")
    try {
      await api(`/v1/tokens/${encodeURIComponent(id)}`, { method: "DELETE" })
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not revoke the token")
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <p className="text-sm text-muted-foreground">
        Tokens let the <code>pursuit</code> CLI, agents, and the browser extension call the API as you. Approval stays a
        human action, so tokens cannot carry the approve scope.
      </p>
      <div className="flex flex-col gap-3 rounded-md border p-3">
        <div className="flex flex-col gap-2">
          <Label htmlFor="token-name">Name</Label>
          <Input
            id="token-name"
            placeholder="e.g. laptop CLI"
            value={name}
            maxLength={80}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && void create()}
          />
        </div>
        <div className="flex flex-col gap-2">
          <Label>Access</Label>
          <ToggleGroup type="single" variant="outline" value={scope} onValueChange={(v) => v && setScope(v)}>
            <ToggleGroupItem value="read">
              Read only
            </ToggleGroupItem>
            <ToggleGroupItem value="write">
              Read &amp; write
            </ToggleGroupItem>
          </ToggleGroup>
        </div>
        <Button onClick={create} disabled={busy || !name.trim()} className="self-start">
          Generate token
        </Button>
      </div>
      {secret && <Secret value={secret} />}
      {error && <p className="text-sm text-destructive">{error}</p>}
      <div className="flex flex-col divide-y rounded-md border text-sm">
        {tokens == null && <p className="p-3 text-muted-foreground">Loading…</p>}
        {tokens?.length === 0 && <p className="p-3 text-muted-foreground">No active tokens.</p>}
        {tokens?.map((t) => (
          <div key={t.id} className="flex items-center gap-3 p-3">
            <div className="flex min-w-0 flex-1 flex-col">
              <span className="truncate font-medium">{t.name}</span>
              <span className="text-xs text-muted-foreground">
                <code>{t.prefix}…</code> · {t.scopes.join(", ")} · created {new Date(t.created_at).toLocaleDateString()} ·{" "}
                {t.last_used_at ? "used " + new Date(t.last_used_at).toLocaleDateString() : "never used"}
              </span>
            </div>
            <Button size="icon-sm" variant="ghost" aria-label={`Revoke ${t.name}`} onClick={() => revoke(t.id)}>
              <Trash2 />
            </Button>
          </div>
        ))}
      </div>
    </div>
  )
}
