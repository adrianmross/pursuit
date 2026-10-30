"use client"

import { useEffect } from "react"
import { ThemeProvider } from "next-themes"
import { AuthProvider } from "@/lib/auth"
import { applyAppearance } from "@/lib/settings"
import { TooltipProvider } from "@/components/ui/tooltip"
import type { Me } from "@/lib/types"

export function Providers({ initialMe, children }: { initialMe: Me | null; children: React.ReactNode }) {
  // Put the saved accent, text size, density and motion on <html> once.
  useEffect(() => {
    applyAppearance()
  }, [])
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
      <AuthProvider initialMe={initialMe}>
        <TooltipProvider delayDuration={300}>{children}</TooltipProvider>
      </AuthProvider>
    </ThemeProvider>
  )
}
