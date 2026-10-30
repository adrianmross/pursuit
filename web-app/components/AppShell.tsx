"use client"

import { Suspense, useEffect, useState } from "react"
import Link from "next/link"
import { usePathname, useRouter, useSearchParams } from "next/navigation"
import { Archive, LogOut, MoreVertical, Search, Settings, User } from "lucide-react"
import { useAuth } from "@/lib/auth"
import { CommandPalette } from "@/components/CommandPalette"
import { SettingsSheet } from "@/components/SettingsSheet"
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
    <div className="relative mx-auto w-full max-w-2xl">
      <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        placeholder="Search — try status:applied, tag:remote, applied:>2026-01-01"
        className="h-10 pl-9 text-base"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  )
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const { me, signOut } = useAuth()
  const router = useRouter()
  const [settingsOpen, setSettingsOpen] = useState(false)
  const identity = me?.token.name || "token"
  const scopes = (me?.token.scopes || []).join(", ")

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
          <CommandPalette />
          <kbd className="hidden rounded border bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground sm:inline">⌘K</kbd>
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
              <DropdownMenuItem onClick={() => router.push("/?archived=1")}>
                <Archive /> View archived
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => setSettingsOpen(true)}>
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
      <main className="flex-1 p-4">{children}</main>
      <SettingsSheet open={settingsOpen} onOpenChange={setSettingsOpen} />
    </div>
  )
}
