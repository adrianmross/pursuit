"use client"

// Consent screens for `pursuit auth login`: /authorize (browser + PKCE)
// and /device (device code). Both sign the human in inline, so the query
// string the CLI built survives sign-in, then ask the server to approve.
// The server re-checks everything; this page only shows and forwards it.

import { useState, type FormEvent, type ReactNode } from "react"
import { api, ApiError } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { SignInForm } from "@/components/SignInForm"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

const SCOPE_HELP: Record<string, string> = {
  read: "See your applications, companies, and settings.",
  write: "Create and change applications. This cannot approve anything.",
  approve: "Approve prepared applications for submission. Only grant this to a CLI you drive yourself, never to an agent.",
}

function parseScopes(raw: string): string[] {
  const set = new Set(raw.split(/[\s,]+/).filter(Boolean))
  if (set.size === 0) {
    set.add("read")
    set.add("write")
  }
  if (set.has("write") || set.has("approve")) set.add("read")
  return ["read", "write", "approve"].filter((s) => set.has(s)).concat([...set].filter((s) => !(s in SCOPE_HELP)))
}

function errorText(err: unknown) {
  if (err instanceof ApiError) return err.hint ? `${err.message}. ${err.hint}` : err.message
  return err instanceof Error ? err.message : "Request failed"
}

function Shell({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-svh items-center justify-center bg-brand-forest p-4">
      <div className="flex w-full max-w-md flex-col gap-4">
        <div className="flex flex-col items-center gap-3 pb-2 text-center">
          <img src="/brand/hire-pursuit-wordmark-reverse.svg" alt="Hire Pursuit" className="h-8 w-auto" />
        </div>
        {children}
      </div>
    </div>
  )
}

function ScopeList({ scopes }: { scopes: string[] }) {
  return (
    <ul className="flex flex-col gap-2">
      {scopes.map((scope) => (
        <li key={scope} className="flex items-start gap-2 text-sm">
          <Badge variant={scope === "approve" ? "destructive" : "secondary"}>{scope}</Badge>
          <span className="text-muted-foreground">{SCOPE_HELP[scope] || "Unknown scope; the server will refuse it."}</span>
        </li>
      ))}
    </ul>
  )
}

function Notice({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{title}</CardTitle>
        <CardDescription>{children}</CardDescription>
      </CardHeader>
    </Card>
  )
}

// Signs in inline when needed and refuses sessions that can't approve
// (agent tokens never carry approve), so a page renders only for a human.
function useApprover(allowTokenLogin: boolean): ReactNode | null {
  const { me, loading, can, isDemo, signOut } = useAuth()
  if (loading) return <Shell>{null}</Shell>
  if (!me) return <SignInForm allowTokenLogin={allowTokenLogin} demoAvailable={false} />
  if (!can("approve") || isDemo) {
    return (
      <Shell>
        <Notice title="This session can't approve a CLI login">
          You are signed in with a token that lacks the approve scope. Sign out and sign in as yourself.
        </Notice>
        <Button variant="outline" onClick={() => void signOut()}>
          Sign out
        </Button>
      </Shell>
    )
  }
  return null
}

export function AuthorizeConsent({ params, allowTokenLogin }: { params: URLSearchParams; allowTokenLogin: boolean }) {
  const { me } = useAuth()
  const gate = useApprover(allowTokenLogin)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState<"approved" | "denied" | null>(null)
  if (gate) return gate

  const redirectURI = params.get("redirect_uri") || ""
  const scope = params.get("scope") || ""
  let target = redirectURI
  try {
    target = new URL(redirectURI).host
  } catch {
    // shown as-is; the server rejects it
  }

  async function decide(approve: boolean) {
    setBusy(true)
    setError(null)
    try {
      const { redirect_to } = await api<{ redirect_to: string }>("/v1/oauth/authorize", {
        method: "POST",
        body: {
          client_id: params.get("client_id") || "",
          redirect_uri: redirectURI,
          code_challenge: params.get("code_challenge") || "",
          code_challenge_method: params.get("code_challenge_method") || "",
          state: params.get("state") || "",
          scope,
          approve,
        },
      })
      setDone(approve ? "approved" : "denied")
      window.location.assign(redirect_to)
    } catch (err) {
      setError(errorText(err))
      setBusy(false)
    }
  }

  if (done) {
    return (
      <Shell>
        <Notice title={done === "approved" ? "CLI signed in" : "Login denied"}>
          Return to your terminal. You can close this tab.
        </Notice>
      </Shell>
    )
  }

  return (
    <Shell>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Sign in the Pursuit CLI?</CardTitle>
          <CardDescription>
            A <code>pursuit auth login</code> on this computer ({target}) is asking for a token for {me?.user?.email || "your account"}.
            Only approve if you just ran it.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4 pt-0">
          <ScopeList scopes={parseScopes(scope)} />
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <div className="flex gap-2">
            <Button variant="outline" className="flex-1" disabled={busy} onClick={() => void decide(false)}>
              Deny
            </Button>
            <Button className="flex-1" disabled={busy} onClick={() => void decide(true)}>
              Approve
            </Button>
          </div>
        </CardContent>
      </Card>
    </Shell>
  )
}

type DeviceRequest = { user_code: string; client_id: string; scopes: string[]; status: string; expires_at: string }

export function DeviceConsent({ initialCode, allowTokenLogin }: { initialCode: string; allowTokenLogin: boolean }) {
  const gate = useApprover(allowTokenLogin)
  const [code, setCode] = useState(initialCode)
  const [request, setRequest] = useState<DeviceRequest | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState<string | null>(null)
  if (gate) return gate

  async function lookup(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError(null)
    try {
      setRequest(await api<DeviceRequest>("/v1/oauth/device?user_code=" + encodeURIComponent(code.trim())))
    } catch (err) {
      setError(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  async function decide(approve: boolean) {
    if (!request) return
    setBusy(true)
    setError(null)
    try {
      const { status } = await api<{ status: string }>("/v1/oauth/device/approve", {
        method: "POST",
        body: { user_code: request.user_code, approve },
      })
      setDone(status)
    } catch (err) {
      setError(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  if (done) {
    return (
      <Shell>
        <Notice title={done === "approved" ? "Device approved" : "Login denied"}>
          Return to your terminal. The CLI picks this up on its next poll.
        </Notice>
      </Shell>
    )
  }

  return (
    <Shell>
      {!request ? (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Connect a device</CardTitle>
            <CardDescription>
              Enter the code <code>pursuit auth login --device</code> printed.
            </CardDescription>
          </CardHeader>
          <CardContent className="pt-0">
            <form onSubmit={lookup} className="flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <Label htmlFor="user-code">Code</Label>
                <Input
                  id="user-code"
                  autoComplete="off"
                  autoCapitalize="characters"
                  placeholder="BCDF-GHJK"
                  className="font-mono uppercase tracking-widest"
                  required
                  value={code}
                  onChange={(event) => setCode(event.target.value)}
                />
              </div>
              {error && (
                <p role="alert" className="text-sm text-destructive">
                  {error}
                </p>
              )}
              <Button type="submit" disabled={busy || code.trim() === ""}>
                Continue
              </Button>
            </form>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Approve device {request.user_code}?</CardTitle>
            <CardDescription>
              Only approve if the code matches the one in your terminal. The CLI gets a token with:
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-4 pt-0">
            <ScopeList scopes={request.scopes} />
            {error && (
              <p role="alert" className="text-sm text-destructive">
                {error}
              </p>
            )}
            <div className="flex gap-2">
              <Button variant="outline" className="flex-1" disabled={busy} onClick={() => void decide(false)}>
                Deny
              </Button>
              <Button className="flex-1" disabled={busy} onClick={() => void decide(true)}>
                Approve
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </Shell>
  )
}
