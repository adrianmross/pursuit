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
    <div className="flex min-h-svh items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-xl">Pursuit</CardTitle>
          <CardDescription>
            Paste a token once. It stays in this browser and is sent as a bearer token to this server.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {(localError || error) && (
            <p className="mb-3 text-sm text-destructive">{localError || error}</p>
          )}
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
  )
}
