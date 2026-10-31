"use client"

import { useSyncExternalStore } from "react"
import { Monitor, Moon, Sun } from "lucide-react"
import { useTheme } from "next-themes"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

// Cycles system -> light -> dark. It writes the same next-themes setting as
// Settings > Appearance, so a choice made signed out is the one the app uses
// after sign-in, and a saved setting shows here after sign-out.
const NEXT = { system: "light", light: "dark", dark: "system" } as const
const ICON = { system: Monitor, light: Sun, dark: Moon }
type Mode = keyof typeof NEXT

const noop = () => () => {}

export function ThemeToggle({ className }: { className?: string }) {
  const { theme, setTheme } = useTheme()
  // The stored theme is only known in the browser; render "system" until hydrated.
  const mounted = useSyncExternalStore(noop, () => true, () => false)
  const current: Mode = mounted && theme && theme in NEXT ? (theme as Mode) : "system"
  const Icon = ICON[current]
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className={cn("shrink-0", className)}
      aria-label={`Theme: ${current}. Switch to ${NEXT[current]}`}
      title={`Theme: ${current}`}
      onClick={() => setTheme(NEXT[current])}
    >
      <Icon />
    </Button>
  )
}
