"use client"

import { useEffect, useState, type FormEvent } from "react"
import { api, EXPORT_MODE } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { authClient } from "@/lib/betterauth"
import { CircleAlert } from "lucide-react"
import { Alert, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Field, FieldDescription, FieldGroup, FieldLabel, FieldSeparator } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { CommandBlock } from "@/components/CommandBlock"
import { InputOTP, InputOTPGroup, InputOTPSlot } from "@/components/ui/input-otp"

function ErrorAlert({ message }: { message?: string | null }) {
  if (!message) return null
  return (
    <Alert variant="destructive">
      <CircleAlert />
      <AlertTitle>{message}</AlertTitle>
    </Alert>
  )
}

function VerifyStep({
  email,
  password,
  onBack,
  onVerified,
}: {
  email: string
  password: string
  onBack: () => void
  onVerified: () => Promise<void>
}) {
  const [otp, setOtp] = useState("")
  const [verifying, setVerifying] = useState(false)
  const [resending, setResending] = useState(false)
  const [localError, setLocalError] = useState<string | null>(null)

  async function sendCode() {
    if (!authClient) return
    setLocalError(null)
    const { error: sendError } = await authClient.emailOtp.sendVerificationOtp({ email, type: "email-verification" })
    if (sendError) setLocalError(sendError.message || "Could not send the code")
  }

  useEffect(() => {
    // Sends the verification email once on mount; the state it sets comes
    // from that request's response, not synchronously in the effect body.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void sendCode()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function onResend() {
    setResending(true)
    await sendCode()
    setResending(false)
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    if (!authClient) return
    setVerifying(true)
    setLocalError(null)
    try {
      const { error: verifyError } = await authClient.emailOtp.verifyEmail({ email, otp })
      if (verifyError) throw new Error(verifyError.message || "That code didn't work")
      // Verifying the code doesn't itself start a session -- sign in again
      // now that the email is confirmed.
      const result = await authClient.signIn.email({ email, password })
      if (result.error) throw new Error(result.error.message || "Sign-in failed")
      await onVerified()
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "That code didn't work")
    } finally {
      setVerifying(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Enter verification code</CardTitle>
        <CardDescription>Sent to {email}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4 pt-0">
        <ErrorAlert message={localError} />
        <form onSubmit={onSubmit} className="flex flex-col items-center gap-4">
          <InputOTP maxLength={6} value={otp} onChange={setOtp} autoFocus>
            <InputOTPGroup>
              {Array.from({ length: 6 }, (_, i) => (
                <InputOTPSlot key={i} index={i} />
              ))}
            </InputOTPGroup>
          </InputOTP>
          <div className="flex w-full gap-2">
            <Button type="button" variant="outline" onClick={onBack} className="flex-1">
              Back
            </Button>
            <Button type="submit" disabled={verifying || otp.length < 6} className="flex-1">
              {verifying && <Spinner />}
              {verifying ? "Verifying…" : "Verify"}
            </Button>
          </div>
          <Button type="button" variant="link" size="sm" className="text-muted-foreground" disabled={resending} onClick={onResend}>
            {resending ? "Sending…" : "Resend code"}
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

function AccountSignIn() {
  const { signIn, error } = useAuth()
  const [step, setStep] = useState<"credentials" | "verify">("credentials")
  const [mode, setMode] = useState<"sign-in" | "sign-up">("sign-in")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [name, setName] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [localError, setLocalError] = useState<string | null>(null)

  async function finishSignIn() {
    if (!authClient) return
    const { data, error: tokenError } = await authClient.token()
    if (tokenError || !data?.token) throw new Error("Could not mint a session token")
    await signIn(data.token)
  }

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
      if (result.error) {
        const message = result.error.message || "Sign-in failed"
        if (/verif/i.test(message) || result.error.code === "EMAIL_NOT_VERIFIED") {
          setStep("verify")
          return
        }
        throw new Error(message)
      }
      await finishSignIn()
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "Sign-in failed")
    } finally {
      setSubmitting(false)
    }
  }

  if (step === "verify") {
    return (
      <VerifyStep
        email={email}
        password={password}
        onBack={() => setStep("credentials")}
        onVerified={finishSignIn}
      />
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{mode === "sign-up" ? "Create an account" : "Sign in"}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4 pt-0">
        <ErrorAlert message={localError || error} />
        <form onSubmit={onSubmit}>
          <FieldGroup className="gap-4">
            {mode === "sign-up" && (
              <Field>
                <FieldLabel htmlFor="name">Name</FieldLabel>
                <Input id="name" autoComplete="name" value={name} onChange={(event) => setName(event.target.value)} />
              </Field>
            )}
            <Field>
              <FieldLabel htmlFor="email">Email</FieldLabel>
              <Input
                id="email"
                type="email"
                autoComplete="email"
                required
                value={email}
                onChange={(event) => setEmail(event.target.value)}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="password">Password</FieldLabel>
              <Input
                id="password"
                type="password"
                autoComplete={mode === "sign-up" ? "new-password" : "current-password"}
                required
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </Field>
            <Field>
              <Button type="submit" disabled={submitting}>
                {submitting && <Spinner />}
                {mode === "sign-up" ? "Create account" : "Sign in"}
              </Button>
              <Button
                type="button"
                variant="link"
                size="sm"
                className="text-muted-foreground"
                onClick={() => setMode(mode === "sign-up" ? "sign-in" : "sign-up")}
              >
                {mode === "sign-up" ? "Already have an account? Sign in" : "New here? Create an account"}
              </Button>
            </Field>
          </FieldGroup>
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
        <CardDescription>From the daemon&apos;s bootstrap log, or issued on demand.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4 pt-0">
        <CommandBlock command={"pursuit serve\npursuit tokens create"} />
        <ErrorAlert message={localError || error} />
        <form onSubmit={onSubmit}>
          <FieldGroup className="gap-4">
            <Field>
              <FieldLabel htmlFor="token">API token</FieldLabel>
              <Input
                id="token"
                type="password"
                autoComplete="off"
                required
                value={token}
                onChange={(event) => setTokenValue(event.target.value)}
              />
            </Field>
            <Button type="submit" variant="secondary" disabled={submitting}>
              {submitting && <Spinner />}
              Continue
            </Button>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  )
}

function TryDemo() {
  const { startDemo, error } = useAuth()
  const [submitting, setSubmitting] = useState(false)
  const [localError, setLocalError] = useState<string | null>(null)

  async function onClick() {
    setSubmitting(true)
    setLocalError(null)
    try {
      await startDemo()
    } catch {
      setLocalError("Could not start the demo.")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Field className="text-center">
      <ErrorAlert message={localError || error} />
      <Button type="button" variant="outline" disabled={submitting} onClick={onClick}>
        {submitting && <Spinner />}
        {submitting ? "Starting…" : "Try it without an account"}
      </Button>
      <FieldDescription className="text-center text-xs">
        A throwaway sandbox pre-loaded with example data — nothing you do there touches a real account.
      </FieldDescription>
    </Field>
  )
}

export function SignInForm({
  allowTokenLogin: initialAllowTokenLogin,
  demoAvailable: initialDemoAvailable,
}: {
  allowTokenLogin: boolean
  demoAvailable: boolean
}) {
  const [allowTokenLogin, setAllowTokenLogin] = useState(initialAllowTokenLogin)
  const [demoAvailable, setDemoAvailable] = useState(initialDemoAvailable)

  // Export mode has no server to compute this ahead of time — check client-side.
  useEffect(() => {
    if (!EXPORT_MODE) return
    api<{ allow_token_login: boolean; demo_available: boolean }>("/v1/auth/providers")
      .then((result) => {
        setAllowTokenLogin(result.allow_token_login)
        setDemoAvailable(result.demo_available)
      })
      .catch(() => {})
  }, [])

  return (
    <div className="flex min-h-svh items-center justify-center bg-brand-forest p-4">
      <div className="flex w-full max-w-md flex-col gap-4">
        <div className="flex flex-col items-center gap-3 pb-2 text-center">
          <img src="/brand/hire-pursuit-wordmark-reverse.svg" alt="Hire Pursuit" className="h-8 w-auto" />
          <p className="text-sm text-brand-cream/80">A job application tracker you drive from the CLI or from here.</p>
        </div>
        {demoAvailable && <TryDemo />}
        {demoAvailable && (authClient || allowTokenLogin) && (
          <FieldSeparator className="my-0 text-brand-cream/70 *:data-[slot=separator]:bg-brand-cream/20 *:data-[slot=field-separator-content]:bg-brand-forest *:data-[slot=field-separator-content]:text-brand-cream/70">
            or
          </FieldSeparator>
        )}
        {authClient && <AccountSignIn />}
        {allowTokenLogin && <TokenSignIn />}
      </div>
    </div>
  )
}
