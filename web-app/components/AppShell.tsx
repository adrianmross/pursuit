"use client"

import { Suspense, useEffect, useState } from "react"
import Link from "next/link"
import { usePathname, useRouter, useSearchParams } from "next/navigation"
import { Archive, IdCard, LogOut, Search, Settings } from "lucide-react"
import { useAuth } from "@/lib/auth"
import { isMac, useShortcuts } from "@/lib/keybindings"
import { Kbd } from "@/components/ui/kbd"
import { CommandPalette } from "@/components/CommandPalette"
import { SettingsDialog } from "@/components/SettingsDialog"
import { SiteFooter } from "@/components/SiteFooter"
import { UserAvatar, displayName } from "@/components/UserAvatar"
import { ProfileNudge } from "@/components/settings/ProfilePanel"
import { Button } from "@/components/ui/button"
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

function GlobalSearch() {
  const pathname = usePathname()
  const router = useRouter()
  const params = useSearchParams()
  const onPipeline = pathname === "/pipeline"
  const [value, setValue] = useState(onPipeline ? params.get("q") || "" : "")

  useEffect(() => {
    // Syncs the box with the URL's ?q= when the external router state
    // changes (e.g. clearing filters elsewhere), not on every render.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (onPipeline) setValue(params.get("q") || "")
  }, [onPipeline, params])

  function onChange(next: string) {
    setValue(next)
    window.clearTimeout((window as unknown as { __search?: number }).__search)
    ;(window as unknown as { __search?: number }).__search = window.setTimeout(() => {
      const merged = new URLSearchParams(onPipeline ? params.toString() : undefined)
      if (next) merged.set("q", next)
      else merged.delete("q")
      router.push(`/pipeline?${merged.toString()}`)
    }, 200)
  }

  return (
    <InputGroup className="mx-auto h-10 max-w-2xl">
      <InputGroupInput
        data-search="global"
        placeholder="Search — try status:applied, tag:remote, applied:>2026-01-01"
        className="text-base"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={(event) => event.key === "Escape" && event.currentTarget.blur()}
      />
      <InputGroupAddon>
        <Search />
      </InputGroupAddon>
    </InputGroup>
  )
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const { me, signOut } = useAuth()
  const router = useRouter()
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
    home: () => router.push("/"),
    pipeline: () => router.push("/pipeline"),
    archived: () => router.push("/?archived=1"),
    new: () => router.push("/app/new"),
  })
  const identity = displayName(me)

  return (
    <div className="flex min-h-svh flex-col">
      <header className="grid grid-cols-[1fr_auto_1fr] items-center gap-4 border-b px-4 py-2.5">
        <Link href="/" aria-label="Hire Pursuit">
          {/* eslint-disable-next-line @next/next/no-img-element -- drawn SVG wordmark, not a photo next/image needs to optimize */}
          <img src="/brand/hire-pursuit-wordmark.svg" alt="" className="h-5 w-auto dark:hidden" />
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src="/brand/hire-pursuit-wordmark-reverse.svg" alt="" className="hidden h-5 w-auto dark:block" />
        </Link>
        <Suspense fallback={<div className="mx-auto w-full max-w-2xl" />}>
          <GlobalSearch />
        </Suspense>
        <div className="flex items-center justify-end gap-2">
          <CommandPalette onOpenSettings={openSettings} />
          <Kbd className="hidden sm:inline-flex">{isMac ? "⌘" : "Ctrl"} K</Kbd>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="rounded-full" aria-label="Account">
                <UserAvatar me={me} size="sm" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuLabel className="flex items-center gap-2">
                <UserAvatar me={me} />
                <span className="min-w-0">
                  <span className="block truncate text-sm">{identity}</span>
                  {me?.user?.email && me.user.email !== identity && (
                    <span className="block truncate text-xs font-normal text-muted-foreground">{me.user.email}</span>
                  )}
                </span>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => router.push("/?archived=1")}>
                <Archive /> View archived
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => openSettings("profile")}>
                <IdCard /> Profile &amp; documents
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => openSettings()}>
                <Settings /> Settings
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => void signOut()} variant="destructive">
                <LogOut /> Sign out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </header>
      <main className="flex-1 p-4">
        <ProfileNudge onOpen={() => openSettings("profile")} />
        {children}
      </main>
      <SiteFooter />
      <SettingsDialog key={settingsKey} initial={settingsSection} open={settingsOpen} onOpenChange={setSettingsOpen} />
    </div>
  )
}
