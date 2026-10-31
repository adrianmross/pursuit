"use client"

import { useState, type FormEvent } from "react"
import { Info, KeyRound, LogOut, MonitorSmartphone } from "lucide-react"
import { useAuth } from "@/lib/auth"
import { authClient } from "@/lib/betterauth"
import { api } from "@/lib/api-client"
import { UserAvatar, avatarDataURL, displayName } from "@/components/UserAvatar"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { ButtonGroup } from "@/components/ui/button-group"
import { Field, FieldDescription, FieldGroup, FieldLabel, FieldLegend, FieldSeparator, FieldSet } from "@/components/ui/field"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemMedia, ItemTitle } from "@/components/ui/item"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

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
    <form onSubmit={save}>
      <FieldSet>
        <FieldLegend variant="label">Change email</FieldLegend>
        <Field>
          <Input type="email" aria-label="New email" placeholder="new@example.com" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} />
          <FieldDescription>A confirmation link goes to the new address.</FieldDescription>
        </Field>
        <Field orientation="horizontal">
          <Button type="submit" disabled={!email}>
            Change email
          </Button>
          <Msg {...msg} />
        </Field>
      </FieldSet>
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
    <Item variant="outline" size="sm">
      <ItemMedia variant="icon">
        <MonitorSmartphone />
      </ItemMedia>
      <ItemContent>
        <ItemTitle>Other sessions</ItemTitle>
        <ItemDescription>Every other browser and device signed in to this account.</ItemDescription>
        <Msg {...msg} />
      </ItemContent>
      <ItemActions>
        <Button variant="outline" size="sm" onClick={revoke}>
          Sign out other sessions
        </Button>
      </ItemActions>
    </Item>
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
    <form onSubmit={save}>
      <FieldSet>
        <FieldLegend variant="label">Change password</FieldLegend>
        <FieldGroup className="grid gap-4 sm:grid-cols-2">
          <Field>
            <FieldLabel htmlFor="pw-current">Current password</FieldLabel>
            <Input id="pw-current" type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} />
          </Field>
          <Field>
            <FieldLabel htmlFor="pw-new">New password</FieldLabel>
            <Input id="pw-new" type="password" autoComplete="new-password" minLength={8} value={next} onChange={(e) => setNext(e.target.value)} />
          </Field>
        </FieldGroup>
        <FieldDescription>At least 8 characters. Changing it signs out your other sessions.</FieldDescription>
        <Field orientation="horizontal">
          <Button type="submit" disabled={!current || next.length < 8}>
            Update password
          </Button>
          <Msg {...msg} />
        </Field>
      </FieldSet>
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
        <Alert>
          <Info />
          <AlertDescription>
            This server signs in with an API token or the platform in front of it, so there is no email or password to
            manage here. Revoke or rotate tokens under API tokens.
          </AlertDescription>
        </Alert>
      )}
      <FieldSeparator />
      <FieldSet>
        <FieldLegend variant="label">Session</FieldLegend>
        <ItemGroup className="gap-2">
          <Item variant="outline" size="sm">
            <ItemMedia variant="icon">
              <KeyRound />
            </ItemMedia>
            <ItemContent>
              <ItemTitle>Signed in as {displayName(me)}</ItemTitle>
              <ItemDescription>
                via {me?.token.name || "token"}
                {me?.token.scopes?.length ? ` (${me.token.scopes.join(", ")})` : ""}
              </ItemDescription>
            </ItemContent>
            <ItemActions>
              <Button variant="outline" size="sm" onClick={signOut}>
                <LogOut />
                Sign out
              </Button>
            </ItemActions>
          </Item>
          {authClient && <OtherSessions />}
        </ItemGroup>
      </FieldSet>
    </div>
  )
}
