"use client"

import { ThemeProvider } from "next-themes"
import { AuthProvider } from "@/lib/auth"
import { TooltipProvider } from "@/components/ui/tooltip"
import type { Me } from "@/lib/types"

export function Providers({ initialMe, children }: { initialMe: Me | null; children: React.ReactNode }) {
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
      <AuthProvider initialMe={initialMe}>
        <TooltipProvider delayDuration={300}>{children}</TooltipProvider>
      </AuthProvider>
    </ThemeProvider>
  )
}
