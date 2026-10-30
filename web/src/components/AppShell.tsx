import { useEffect, useState } from "react"
import { NavLink, Outlet, useLocation, useNavigate, useSearchParams } from "react-router-dom"
import { Archive, LogOut, MoreVertical, Search, Settings, User } from "lucide-react"
import { useAuth } from "@/lib/auth"
import { isMac, useShortcuts } from "@/lib/keybindings"
import { Kbd } from "@/components/ui/kbd"
import { CommandPalette } from "@/components/CommandPalette"
import { SettingsDialog } from "@/components/SettingsDialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

function GlobalSearch() {
  const location = useLocation()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const onPipeline = location.pathname === "/pipeline"
  const [value, setValue] = useState(onPipeline ? params.get("q") || "" : "")

  useEffect(() => {
    if (onPipeline) setValue(params.get("q") || "")
  }, [onPipeline, params])

  function onChange(next: string) {
    setValue(next)
    window.clearTimeout((window as unknown as { __search?: number }).__search)
    ;(window as unknown as { __search?: number }).__search = window.setTimeout(() => {
      const merged = new URLSearchParams(onPipeline ? params : undefined)
      if (next) merged.set("q", next)
      else merged.delete("q")
      navigate({ pathname: "/pipeline", search: merged.toString() }, { replace: onPipeline })
    }, 200)
  }

  return (
    <div className="relative mx-auto w-full max-w-2xl">
      <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        data-search="global"
        placeholder="Search — try status:applied, tag:remote, applied:>2026-01-01"
        className="h-10 pl-9 text-base"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  )
}

export function AppShell() {
  const { me, signOut } = useAuth()
  const navigate = useNavigate()
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [settingsSection, setSettingsSection] = useState("appearance")
  const [settingsKey, setSettingsKey] = useState(0)
  // Remount on each open so the dialog starts on the requested section.
  const openSettings = (section = "appearance") => {
    setSettingsSection(section)
    setSettingsKey((k) => k + 1)
    setSettingsOpen(true)
  }
  const focus = (...names: string[]) => {
    const el = names.map((n) => document.querySelector<HTMLInputElement>(`[data-search="${n}"]`)).find(Boolean)
    el?.focus()
    el?.select()
    return el ? undefined : (false as const)
  }
  useShortcuts({
    settings: () => openSettings(),
    keys: () => openSettings("keys"),
    // Search whatever is in front: the open settings dialog, else the app.
    search: () => (document.querySelector("[data-settings-dialog]") ? focus("audit", "settings") : focus("global")),
    find: () => focus("audit"),
    home: () => void navigate("/"),
    pipeline: () => void navigate("/pipeline"),
    archived: () => void navigate("/?archived=1"),
    new: () => void navigate("/app/new"),
  })
  const identity = me?.token.name || "token"
  const scopes = (me?.token.scopes || []).join(", ")

  return (
    <div className="flex min-h-svh flex-col">
      <header className="grid grid-cols-[1fr_auto_1fr] items-center gap-4 border-b px-4 py-2.5">
        <NavLink to="/" aria-label="Hire Pursuit">
          <img src="/brand/hire-pursuit-wordmark.svg" alt="" className="h-5 w-auto dark:hidden" />
          <img src="/brand/hire-pursuit-wordmark-reverse.svg" alt="" className="hidden h-5 w-auto dark:block" />
        </NavLink>
        <GlobalSearch />
        <div className="flex items-center justify-end gap-2">
          <CommandPalette onOpenSettings={openSettings} />
          <Kbd className="hidden sm:inline-flex">{isMac ? "⌘" : "Ctrl"} K</Kbd>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label="Account">
                <MoreVertical />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuLabel className="flex items-center gap-2 text-xs text-muted-foreground">
                <User className="size-3.5" />
                <span className="truncate">
                  {identity}
                  {scopes && <span className="block text-[10px] opacity-75">{scopes}</span>}
                </span>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => navigate("/?archived=1")}>
                <Archive /> View archived
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => openSettings()}>
                <Settings /> Settings
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={signOut} variant="destructive">
                <LogOut /> Sign out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </header>
      <main className="flex-1 p-4">
        <Outlet />
      </main>
      <SettingsDialog key={settingsKey} initial={settingsSection} open={settingsOpen} onOpenChange={setSettingsOpen} />
    </div>
  )
}
