import { useState, type FormEvent } from "react"
import { useAuth } from "@/lib/auth"
import { authClient } from "@/lib/betterauth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

function AccountSignIn() {
  const { signIn, error } = useAuth()
  const [mode, setMode] = useState<"sign-in" | "sign-up">("sign-in")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [name, setName] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [localError, setLocalError] = useState<string | null>(null)

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    if (!authClient) return
    setSubmitting(true)
    setLocalError(null)
    try {
      const result =
        mode === "sign-up"
          ? await authClient.signUp.email({ email, password, name: name.trim() || email })
          : await authClient.signIn.email({ email, password })
      if (result.error) throw new Error(result.error.message || "Sign-in failed")
      const { data, error: tokenError } = await authClient.token()
      if (tokenError || !data?.token) throw new Error("Could not mint a session token")
      await signIn(data.token)
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "Sign-in failed")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{mode === "sign-up" ? "Create an account" : "Sign in"}</CardTitle>
      </CardHeader>
      <CardContent className="pt-0">
        {(localError || error) && <p className="mb-3 text-sm text-destructive">{localError || error}</p>}
        <form onSubmit={onSubmit} className="flex flex-col gap-4">
          {mode === "sign-up" && (
            <div className="flex flex-col gap-2">
              <Label htmlFor="name">Name</Label>
              <Input id="name" autoComplete="name" value={name} onChange={(event) => setName(event.target.value)} />
            </div>
          )}
          <div className="flex flex-col gap-2">
            <Label htmlFor="email">Email</Label>
            <Input
              id="email"
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              autoComplete={mode === "sign-up" ? "new-password" : "current-password"}
              required
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </div>
          <Button type="submit" disabled={submitting}>
            {mode === "sign-up" ? "Create account" : "Sign in"}
          </Button>
          <button
            type="button"
            className="text-sm text-muted-foreground underline-offset-4 hover:underline"
            onClick={() => setMode(mode === "sign-up" ? "sign-in" : "sign-up")}
          >
            {mode === "sign-up" ? "Already have an account? Sign in" : "New here? Create an account"}
          </button>
        </form>
      </CardContent>
    </Card>
  )
}

function TokenSignIn() {
  const { signIn, error } = useAuth()
  const [token, setTokenValue] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [localError, setLocalError] = useState<string | null>(null)

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setSubmitting(true)
    setLocalError(null)
    try {
      await signIn(token.trim())
    } catch {
      setLocalError("That token was rejected.")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Or paste an API token</CardTitle>
        <CardDescription>From `pursuit serve`'s bootstrap log, or issued with `pursuit tokens create`.</CardDescription>
      </CardHeader>
      <CardContent className="pt-0">
        {(localError || error) && <p className="mb-3 text-sm text-destructive">{localError || error}</p>}
        <form onSubmit={onSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="token">API token</Label>
            <Input
              id="token"
              type="password"
              autoComplete="off"
              required
              value={token}
              onChange={(event) => setTokenValue(event.target.value)}
            />
          </div>
          <Button type="submit" variant="secondary" disabled={submitting}>
            Continue
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

export function SignIn() {
  return (
    <div className="flex min-h-svh items-center justify-center bg-brand-forest p-4">
      <div className="flex w-full max-w-md flex-col gap-4">
        <div className="flex flex-col items-center gap-3 pb-2 text-center">
          <img src="/brand/pursuit-wordmark-reverse.svg" alt="Pursuit" className="h-8 w-auto" />
          <p className="text-sm text-brand-cream/80">A job application tracker you drive from the CLI or from here.</p>
        </div>
        {authClient && <AccountSignIn />}
        <TokenSignIn />
      </div>
    </div>
  )
}
