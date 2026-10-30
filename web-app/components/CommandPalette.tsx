"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { Archive, ListChecks, LogOut, Plus } from "lucide-react"
import { api } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { label, statusesInCategory, type Application, type Page } from "@/lib/types"
import { applicationHref } from "@/lib/links"
import { CommandDialog, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command"

export function CommandPalette() {
  const router = useRouter()
  const { signOut } = useAuth()
  const [open, setOpen] = useState(false)
  const [apps, setApps] = useState<Application[]>([])

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.key === "k" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault()
        setOpen((prev) => !prev)
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [])

  useEffect(() => {
    if (!open) return
    api<Page<Application>>("/v1/applications?limit=50&sort=-updated_at")
      .then((page) => setApps(page.data || []))
      .catch(() => setApps([]))
  }, [open])

  function go(fn: () => void) {
    fn()
    setOpen(false)
  }

  async function goThroughPending() {
    const qs = statusesInCategory("action")
      .map((st) => "status=" + encodeURIComponent(st))
      .join("&")
    const page = await api<Page<Application>>(`/v1/applications?limit=1&sort=updated_at&${qs}`)
    const next = page.data[0]
    if (next) router.push(applicationHref(next.id))
  }

  return (
    <CommandDialog open={open} onOpenChange={setOpen} title="Fast actions" description="Jump to an application or run a command">
      <CommandInput placeholder="Jump to an application or run a command…" />
      <CommandList>
        <CommandEmpty>No matches.</CommandEmpty>
        <CommandGroup heading="Actions">
          <CommandItem onSelect={() => go(() => router.push("/app/new"))}>
            <Plus /> Add application
          </CommandItem>
          <CommandItem onSelect={() => go(() => router.push("/?archived=1"))}>
            <Archive /> View archived
          </CommandItem>
          <CommandItem onSelect={() => go(() => void goThroughPending())}>
            <ListChecks /> Go through pending
          </CommandItem>
        </CommandGroup>
        <CommandGroup heading="Applications">
          {apps.map((app) => (
            <CommandItem
              key={app.id}
              value={`${app.company || ""} ${app.title || ""} ${label(app.status)}`}
              onSelect={() => go(() => router.push(applicationHref(app.id)))}
            >
              <span>
                {app.title || "Untitled"}
                <span className="text-muted-foreground"> — {app.company || "Unknown company"}</span>
              </span>
            </CommandItem>
          ))}
        </CommandGroup>
        <CommandGroup heading="Account">
          <CommandItem onSelect={() => go(() => void signOut())}>
            <LogOut /> Sign out
          </CommandItem>
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  )
}
