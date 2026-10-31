"use client"

import { useState, type FormEvent } from "react"
import { useAuth } from "@/lib/auth"
import { authClient } from "@/lib/betterauth"
import { api } from "@/lib/api-client"
import { UserAvatar, avatarDataURL, displayName } from "@/components/UserAvatar"
import { ButtonGroup } from "@/components/ui/button-group"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Item, ItemActions, ItemContent, ItemDescription, ItemMedia, ItemTitle } from "@/components/ui/item"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

function Msg({ ok, text }: { ok: boolean; text: string }) {
  return text ? <p className={"text-sm " + (ok ? "text-emerald-600 dark:text-emerald-400" : "text-destructive")}>{text}</p> : null
}

function ProfileForm() {
  const { me, refresh } = useAuth()
  const [name, setName] = useState(me?.user?.name || "")
  const [msg, setMsg] = useState({ ok: true, text: "" })

  async function patch(body: { name?: string; avatar?: string }) {
    try {
      await api("/v1/me", { method: "PATCH", body })
      await refresh()
      setMsg({ ok: true, text: "Saved." })
    } catch (err) {
      setMsg({ ok: false, text: err instanceof Error ? err.message : "Could not save" })
    }
  }

  async function pick(file?: File) {
    if (file) await patch({ avatar: await avatarDataURL(file) })
  }

  return (
    <form
      onSubmit={(event: FormEvent) => {
        event.preventDefault()
        void patch({ name })
      }}
      className="flex flex-col gap-3"
    >
      <Item variant="outline">
        <ItemMedia>
          <UserAvatar me={me} size="lg" />
        </ItemMedia>
        <ItemContent>
          <ItemTitle>{displayName(me)}</ItemTitle>
          <ItemDescription>{me?.user?.email}</ItemDescription>
        </ItemContent>
        <ItemActions>
          <ButtonGroup>
            <Button type="button" variant="outline" size="sm" asChild>
              <label>
                {me?.user?.avatar ? "Change photo" : "Upload photo"}
                <input type="file" accept="image/*" className="sr-only" onChange={(e) => void pick(e.target.files?.[0])} />
              </label>
            </Button>
            {me?.user?.avatar && (
              <Button type="button" variant="outline" size="sm" onClick={() => void patch({ avatar: "" })}>
                Remove
              </Button>
            )}
          </ButtonGroup>
        </ItemActions>
      </Item>
      <Field>
        <FieldLabel htmlFor="acct-name">Display name</FieldLabel>
        <Input id="acct-name" value={name} onChange={(e) => setName(e.target.value)} />
        <FieldDescription>Shown in the header and recorded on events and the audit log. Your email is your sign-in and can&apos;t be changed here.</FieldDescription>
      </Field>
      <Msg {...msg} />
      <Button type="submit" disabled={!name.trim()} className="self-start">
        Save
      </Button>
    </form>
  )
}

function EmailForm() {
  const [email, setEmail] = useState("")
  const [msg, setMsg] = useState({ ok: true, text: "" })

  async function save(event: FormEvent) {
    event.preventDefault()
    const { error } = await authClient!.changeEmail({ newEmail: email })
    if (error) return setMsg({ ok: false, text: error.message || "Could not change the email" })
    setEmail("")
    setMsg({ ok: true, text: "Check your inbox to confirm the change." })
  }

  return (
    <form onSubmit={save} className="flex flex-col gap-3">
      <h3 className="text-sm font-medium">Change email</h3>
      <Input type="email" aria-label="New email" placeholder="new@example.com" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} />
      <Msg {...msg} />
      <Button type="submit" disabled={!email} className="self-start">
        Change email
      </Button>
    </form>
  )
}

function OtherSessions() {
  const [msg, setMsg] = useState({ ok: true, text: "" })
  async function revoke() {
    const { error } = await authClient!.revokeOtherSessions()
    setMsg(error ? { ok: false, text: error.message || "Could not sign out other sessions" } : { ok: true, text: "Other sessions were signed out." })
  }
  return (
    <div className="flex flex-col gap-2">
      <Button variant="outline" onClick={revoke} className="self-start">
        Sign out other sessions
      </Button>
      <Msg {...msg} />
    </div>
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
      <ProfileForm />
      {authClient ? (
        <>
          <EmailForm />
          <PasswordForm />
        </>
      ) : (
        <p className="text-sm text-muted-foreground">
          This server signs in with an API token or the platform in front of it, so there is no email or password to
          manage here. Revoke or rotate tokens under API tokens.
        </p>
      )}
      <div className="flex flex-col gap-2 border-t pt-4">
        <h3 className="text-sm font-medium">Session</h3>
        <p className="text-sm text-muted-foreground">
          Signed in as {displayName(me)} via {me?.token.name || "token"}
          {me?.token.scopes?.length ? ` (${me.token.scopes.join(", ")})` : ""}.
        </p>
        {authClient && <OtherSessions />}
        <Button variant="outline" onClick={signOut} className="self-start">
          Sign out
        </Button>
      </div>
    </div>
  )
}
