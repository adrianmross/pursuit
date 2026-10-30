import { AppGate } from "@/components/AppGate"
import { AppShell } from "@/components/AppShell"

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <AppGate>
      <AppShell>{children}</AppShell>
    </AppGate>
  )
}
