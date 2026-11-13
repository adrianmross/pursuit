import { AppGate } from "@adrianmross/appkit-ui"
import { AppShell } from "@/components/AppShell"

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <AppGate>
      <AppShell>{children}</AppShell>
    </AppGate>
  )
}
