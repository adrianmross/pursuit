"use client"

import { useEffect, useState } from "react"
import { Check, CircleAlert, Copy, KeyRound, Trash2 } from "lucide-react"
import { api } from "@/lib/api-client"
import type { Page, Token } from "@/lib/types"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Field, FieldDescription, FieldGroup, FieldLabel, FieldLegend, FieldSeparator, FieldSet } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemMedia, ItemTitle } from "@/components/ui/item"
import { Spinner } from "@/components/ui/spinner"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"

function Secret({ value }: { value: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <Alert className="border-amber-500/50 bg-amber-500/10">
      <KeyRound />
      <AlertTitle>Copy this token now</AlertTitle>
      <AlertDescription className="flex flex-col gap-2">
        It is shown once and cannot be recovered.
        <InputGroup className="bg-background">
          <InputGroupInput readOnly value={value} aria-label="New token" className="font-mono text-xs" onFocus={(e) => e.currentTarget.select()} />
          <InputGroupAddon align="inline-end">
            <InputGroupButton
              size="icon-xs"
              aria-label="Copy token"
              onClick={async () => {
                await navigator.clipboard.writeText(value)
                setCopied(true)
                setTimeout(() => setCopied(false), 1500)
              }}
            >
              {copied ? <Check /> : <Copy />}
            </InputGroupButton>
          </InputGroupAddon>
        </InputGroup>
        <code className="text-[11px] break-all">export PURSUIT_TOKEN={value}</code>
      </AlertDescription>
    </Alert>
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
    <FieldGroup>
      <FieldSet>
        <FieldLegend>New token</FieldLegend>
        <FieldDescription>
          Tokens let the <code>pursuit</code> CLI, agents, and the browser extension call the API as you. Approval stays a
          human action, so tokens cannot carry the approve scope.
        </FieldDescription>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="token-name">Name</FieldLabel>
            <Input
              id="token-name"
              placeholder="e.g. laptop CLI"
              value={name}
              maxLength={80}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && void create()}
            />
          </Field>
          <Field>
            <FieldLabel>Access</FieldLabel>
            <div>
              <ToggleGroup type="single" variant="outline" value={scope} onValueChange={(v) => v && setScope(v)}>
                <ToggleGroupItem value="read">
                  Read only
                </ToggleGroupItem>
                <ToggleGroupItem value="write">
                  Read &amp; write
                </ToggleGroupItem>
              </ToggleGroup>
            </div>
          </Field>
          <Button onClick={create} disabled={busy || !name.trim()} className="self-start">
            {busy && <Spinner />}
            Generate token
          </Button>
        </FieldGroup>
      </FieldSet>
      {secret && <Secret value={secret} />}
      {error && (
        <Alert variant="destructive">
          <CircleAlert />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <FieldSeparator />
      <FieldSet>
        <FieldLegend variant="label">Active tokens</FieldLegend>
        {tokens == null ? (
          <Spinner className="text-muted-foreground" />
        ) : tokens.length === 0 ? (
          <Empty className="border p-4">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <KeyRound />
              </EmptyMedia>
              <EmptyTitle>No active tokens</EmptyTitle>
              <EmptyDescription>Generate one above to use the CLI, an agent, or the browser extension.</EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <ItemGroup className="gap-2">
            {tokens.map((t) => (
              <Item key={t.id} variant="outline" size="sm">
                <ItemMedia variant="icon">
                  <KeyRound />
                </ItemMedia>
                <ItemContent className="min-w-0">
                  <ItemTitle className="max-w-full">
                    <span className="truncate">{t.name}</span>
                    {t.scopes.map((s) => (
                      <Badge key={s} variant="secondary">
                        {s}
                      </Badge>
                    ))}
                  </ItemTitle>
                  <ItemDescription className="text-xs">
                    <code>{t.prefix}…</code> · created {new Date(t.created_at).toLocaleDateString()} ·{" "}
                    {t.last_used_at ? "used " + new Date(t.last_used_at).toLocaleDateString() : "never used"}
                  </ItemDescription>
                </ItemContent>
                <ItemActions>
                  <Button size="icon-sm" variant="ghost" aria-label={`Revoke ${t.name}`} onClick={() => revoke(t.id)}>
                    <Trash2 />
                  </Button>
                </ItemActions>
              </Item>
            ))}
          </ItemGroup>
        )}
      </FieldSet>
    </FieldGroup>
  )
}
