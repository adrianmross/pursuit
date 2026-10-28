import { useState, type FormEvent } from "react"
import { useAuth } from "@/lib/auth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

export function SignIn() {
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
    <div className="flex min-h-svh items-center justify-center bg-brand-forest p-4">
      <div className="flex w-full max-w-md flex-col gap-4">
        <div className="flex flex-col items-center gap-3 pb-2 text-center">
          <img src="/brand/pursuit-wordmark-reverse.svg" alt="Pursuit" className="h-8 w-auto" />
          <p className="text-sm text-brand-cream/80">A job application tracker you drive from the CLI or from here.</p>
        </div>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Get started</CardTitle>
            <CardDescription>Three steps if this is your first time.</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3 text-sm">
            <div>
              <p className="font-medium">1. Start the daemon</p>
              <pre className="mt-1 overflow-x-auto rounded bg-muted px-2 py-1.5 text-xs">pursuit serve</pre>
            </div>
            <div>
              <p className="font-medium">2. Copy the bootstrap token it prints</p>
              <p className="text-muted-foreground">Logged once, on an empty database: "bootstrap token created".</p>
            </div>
            <div>
              <p className="font-medium">3. Paste it below</p>
              <p className="text-muted-foreground">It stays in this browser and is sent as a bearer token to this server.</p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            {(localError || error) && <p className="mb-3 text-sm text-destructive">{localError || error}</p>}
            <form onSubmit={onSubmit} className="flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <Label htmlFor="token">API token</Label>
                <Input
                  id="token"
                  type="password"
                  autoComplete="off"
                  autoFocus
                  required
                  value={token}
                  onChange={(event) => setTokenValue(event.target.value)}
                />
              </div>
              <Button type="submit" disabled={submitting}>
                Continue
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
