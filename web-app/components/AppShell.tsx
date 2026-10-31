"use client"

import { Suspense, useState, useSyncExternalStore } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { Archive, Bot, IdCard, LogOut, MessageSquareText, Search, Settings, X } from "lucide-react"
import { useAuth } from "@/lib/auth"
import { isMac, useShortcuts } from "@/lib/keybindings"
import { Kbd } from "@/components/ui/kbd"
import { CommandPalette } from "@/components/CommandPalette"
import { SettingsDialog } from "@/components/SettingsDialog"
import { SearchBox } from "@/components/SearchBox"
import { SiteFooter } from "@/components/SiteFooter"
import { UserAvatar, displayName } from "@/components/UserAvatar"
import { ProfileNudge } from "@/components/profile/ProfilePage"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

function useMedia(query: string) {
  return useSyncExternalStore(
    (cb) => {
      const mq = window.matchMedia(query)
      mq.addEventListener("change", cb)
      return () => mq.removeEventListener("change", cb)
    },
    () => window.matchMedia(query).matches,
    () => false,
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
  const narrow = useMedia("(max-width: 639px)")
  const medium = useMedia("(max-width: 719px)")
  const [searching, setSearching] = useState(false)
  const focus = (...names: string[]) => {
    // On a phone the global search is an icon until tapped.
    if (names[0] === "global" && narrow && !document.querySelector('[data-search="global"]')) {
      setSearching(true)
      return
    }
    const el = names.map((n) => document.querySelector<HTMLInputElement>(`[data-search="${n}"]`)).find(Boolean)
    el?.focus()
    el?.select()
    return el ? undefined : (false as const)
  }
  useShortcuts({
    settings: () => openSettings(),
    keys: () => openSettings("keys"),
    // Search whatever is in front: the open settings dialog, else the app.
    search: () => (document.querySelector("[data-settings-dialog]") ? focus("settings") : focus("global")),
    find: () => focus("audit"),
    // Only on an application page; anywhere else Esc is left alone.
    back: () => (window.location.pathname.startsWith("/app/") ? router.push("/applications") : false),
    home: () => router.push("/start"),
    pipeline: () => router.push("/applications"),
    archived: () => router.push("/applications?archived=1"),
    new: () => router.push("/app/new"),
  })
  const identity = displayName(me)

  return (
    <div className="flex min-h-svh flex-col">
      <header className="flex items-center gap-3 border-b px-4 py-2.5">
        {narrow && searching ? (
          <>
            <Suspense fallback={<div className="w-full" />}>
              <SearchBox placeholder="Search applications" onBlurEmpty={() => setSearching(false)} />
            </Suspense>
            <Button variant="ghost" size="icon" aria-label="Close search" onClick={() => setSearching(false)}>
              <X />
            </Button>
          </>
        ) : (
          <>
            <Link href="/start" aria-label="Hire Pursuit" className="shrink-0">
              {/* eslint-disable-next-line @next/next/no-img-element -- drawn SVG wordmark, not a photo next/image needs to optimize */}
              <img src="/brand/hire-pursuit-wordmark.svg" alt="" className="h-5 w-auto dark:hidden" />
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src="/brand/hire-pursuit-wordmark-reverse.svg" alt="" className="hidden h-5 w-auto dark:block" />
            </Link>
            {narrow ? (
              <Button variant="ghost" size="icon" className="ml-auto" aria-label="Search applications" onClick={() => setSearching(true)}>
                <Search />
              </Button>
            ) : (
              // Takes whatever room the logo and actions leave, up to a third of the screen (never under 22rem).
              <div className="mx-auto min-w-0 max-w-[max(22rem,33vw)] flex-1">
                <Suspense fallback={<div className="h-10 w-full" />}>
                  <SearchBox placeholder={medium ? "Search" : "Search applications"} />
                </Suspense>
              </div>
            )}
        <div className="flex shrink-0 items-center justify-end gap-2">
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
              <DropdownMenuItem onClick={() => router.push("/applications?archived=1")}>
                <Archive /> View archived
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => router.push("/profile")}>
                <IdCard /> Profile
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => router.push("/profile/questions")}>
                <MessageSquareText /> Questions
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => router.push("/agents")}>
                <Bot /> Connect an agent
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
          </>
        )}
      </header>
      <main className="flex-1 p-4">
        <ProfileNudge onOpen={(href) => router.push(href)} />
        {children}
      </main>
      <SiteFooter />
      <SettingsDialog key={settingsKey} initial={settingsSection} open={settingsOpen} onOpenChange={setSettingsOpen} />
    </div>
  )
}
