import { useEffect, useState, type FormEvent } from "react"
import { useAuth } from "@/lib/auth"
import { authClient } from "@/lib/betterauth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

function Msg({ ok, text }: { ok: boolean; text: string }) {
  return text ? <p className={"text-sm " + (ok ? "text-emerald-600 dark:text-emerald-400" : "text-destructive")}>{text}</p> : null
}

function ProfileForm() {
  const [email, setEmail] = useState("")
  const [name, setName] = useState("")
  const [msg, setMsg] = useState({ ok: true, text: "" })

  useEffect(() => {
    void authClient?.getSession().then(({ data }) => {
      setEmail(data?.user.email || "")
      setName(data?.user.name || "")
    })
  }, [])

  async function save(event: FormEvent) {
    event.preventDefault()
    const { error } = await authClient!.updateUser({ name })
    setMsg(error ? { ok: false, text: error.message || "Could not save" } : { ok: true, text: "Saved." })
  }

  return (
    <form onSubmit={save} className="flex flex-col gap-3">
      <div className="flex flex-col gap-2">
        <Label>Email</Label>
        <Input value={email} disabled />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="acct-name">Display name</Label>
        <Input id="acct-name" value={name} onChange={(e) => setName(e.target.value)} />
      </div>
      <Msg {...msg} />
      <Button type="submit" className="self-start">
        Save
      </Button>
    </form>
  )
}

function PasswordForm() {
  const [current, setCurrent] = useState("")
  const [next, setNext] = useState("")
  const [msg, setMsg] = useState({ ok: true, text: "" })

  async function save(event: FormEvent) {
    event.preventDefault()
    const { error } = await authClient!.changePassword({
      currentPassword: current,
      newPassword: next,
      revokeOtherSessions: true,
    })
    if (error) return setMsg({ ok: false, text: error.message || "Could not change the password" })
    setCurrent("")
    setNext("")
    setMsg({ ok: true, text: "Password changed. Other sessions were signed out." })
  }

  return (
    <form onSubmit={save} className="flex flex-col gap-3">
      <h3 className="text-sm font-medium">Change password</h3>
      <div className="flex flex-col gap-2">
        <Label htmlFor="pw-current">Current password</Label>
        <Input id="pw-current" type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="pw-new">New password</Label>
        <Input id="pw-new" type="password" autoComplete="new-password" minLength={8} value={next} onChange={(e) => setNext(e.target.value)} />
      </div>
      <Msg {...msg} />
      <Button type="submit" disabled={!current || next.length < 8} className="self-start">
        Update password
      </Button>
    </form>
  )
}

export function AccountPanel() {
  const { me, signOut } = useAuth()
  return (
    <div className="flex flex-col gap-6">
      {authClient ? (
        <>
          <ProfileForm />
          <PasswordForm />
        </>
      ) : (
        <p className="text-sm text-muted-foreground">
          This server signs in with an API token, so there is no email or password to manage. Revoke or rotate tokens
          under API tokens.
        </p>
      )}
      <div className="flex flex-col gap-2 border-t pt-4">
        <h3 className="text-sm font-medium">Session</h3>
        <p className="text-sm text-muted-foreground">
          Signed in as {me?.token.name || "token"}
          {me?.token.scopes?.length ? ` (${me.token.scopes.join(", ")})` : ""}.
        </p>
        <Button variant="outline" onClick={signOut} className="self-start">
          Sign out
        </Button>
      </div>
    </div>
  )
}
