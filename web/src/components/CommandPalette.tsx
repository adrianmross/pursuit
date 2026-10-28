import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { LogOut, Plus } from "lucide-react"
import { api } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { label, type Application, type Page } from "@/lib/types"
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"

export function CommandPalette() {
  const navigate = useNavigate()
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

  return (
    <CommandDialog open={open} onOpenChange={setOpen} title="Fast actions" description="Jump to an application or run a command">
      <CommandInput placeholder="Jump to an application or run a command…" />
      <CommandList>
        <CommandEmpty>No matches.</CommandEmpty>
        <CommandGroup heading="Actions">
          <CommandItem onSelect={() => go(() => navigate("/?new=1"))}>
            <Plus /> Add application
          </CommandItem>
        </CommandGroup>
        <CommandGroup heading="Applications">
          {apps.map((app) => (
            <CommandItem
              key={app.id}
              value={`${app.company || ""} ${app.title || ""} ${label(app.status)}`}
              onSelect={() => go(() => navigate(`/app/${encodeURIComponent(app.id)}`))}
            >
              <span>
                {app.title || "Untitled"}
                <span className="text-muted-foreground"> — {app.company || "Unknown company"}</span>
              </span>
            </CommandItem>
          ))}
        </CommandGroup>
        <CommandGroup heading="Account">
          <CommandItem onSelect={() => go(signOut)}>
            <LogOut /> Sign out
          </CommandItem>
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  )
}
