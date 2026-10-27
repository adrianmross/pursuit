import { useEffect, useState } from "react"
import { NavLink, Outlet, useLocation, useNavigate, useSearchParams } from "react-router-dom"
import { Moon, Sun, Monitor, LogOut, MoreVertical, Search } from "lucide-react"
import { useAuth } from "@/lib/auth"
import { useTheme } from "@/lib/theme"
import { CommandPalette } from "@/components/CommandPalette"
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

const NAV = [
  { to: "/", label: "Pipeline" },
  { to: "/queue", label: "Queue" },
]

function GlobalSearch() {
  const location = useLocation()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const onPipeline = location.pathname === "/"
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
      navigate({ pathname: "/", search: merged.toString() }, { replace: onPipeline })
    }, 200)
  }

  return (
    <div className="relative w-64">
      <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
      <Input
        placeholder="Search company, title, status, tags"
        className="pl-8"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  )
}

export function AppShell() {
  const { me, signOut } = useAuth()
  const { theme, setTheme } = useTheme()
  const identity = me ? `${me.token.name || "token"} · ${(me.token.scopes || []).join(", ")}` : ""

  return (
    <div className="flex min-h-svh flex-col">
      <header className="flex items-center gap-4 border-b px-4 py-2.5">
        <NavLink to="/" className="font-semibold">
          Pursuit
        </NavLink>
        <nav className="flex gap-1">
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === "/"}
              className={({ isActive }) =>
                `rounded-md px-3 py-1.5 text-sm ${isActive ? "bg-accent text-accent-foreground" : "text-muted-foreground hover:text-foreground"}`
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
        <GlobalSearch />
        <div className="ml-auto flex items-center gap-2">
          <CommandPalette />
          <kbd className="hidden rounded border bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground sm:inline">⌘K</kbd>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label="Account">
                <MoreVertical />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuLabel className="text-xs text-muted-foreground">{identity}</DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => setTheme("light")}>
                <Sun /> Light {theme === "light" && "✓"}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => setTheme("dark")}>
                <Moon /> Dark {theme === "dark" && "✓"}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => setTheme("system")}>
                <Monitor /> System {theme === "system" && "✓"}
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
    </div>
  )
}
