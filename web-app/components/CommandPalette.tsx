"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { Archive, Bot, IdCard, ListChecks, LogOut, Plus } from "lucide-react"
import { SECTION_LINKS } from "@/components/SettingsDialog"
import { api } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { useShortcuts } from "@/lib/keybindings"
import { label, statusesInCategory, type Application, type Page } from "@/lib/types"
import { applicationHref } from "@/lib/links"
import { CommandDialog, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command"

export function CommandPalette({ onOpenSettings }: { onOpenSettings: (section: string) => void }) {
  const router = useRouter()
  const { signOut } = useAuth()
  const [open, setOpen] = useState(false)
  const [apps, setApps] = useState<Application[]>([])

  useShortcuts({ palette: () => setOpen((prev) => !prev) })

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
          <CommandItem onSelect={() => go(() => router.push("/profile"))}>
            <IdCard /> Profile
          </CommandItem>
          <CommandItem onSelect={() => go(() => router.push("/agents"))}>
            <Bot /> Connect an agent
          </CommandItem>
          <CommandItem onSelect={() => go(() => router.push("/app/new"))}>
            <Plus /> Add application
          </CommandItem>
          <CommandItem onSelect={() => go(() => router.push("/applications?archived=1"))}>
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
        <CommandGroup heading="Settings">
          {SECTION_LINKS.map((link) => (
            <CommandItem
              key={link.id}
              value={`settings ${link.label} ${link.keywords}`}
              onSelect={() => go(() => onOpenSettings(link.id))}
            >
              {link.icon} Settings: {link.label}
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
