"use client"

import { useMemo, useState, type KeyboardEvent, type ReactNode } from "react"
import { Database, History, Info, Keyboard, KeyRound, Palette, Rows3, Search, SlidersHorizontal, User } from "lucide-react"
import { FILTERS, MORE, SORTS } from "@/lib/types"
import { getDefaultSort, getDefaultStatus, ACCENTS, getPref, setPref, setDefaultSort, setDefaultStatus, type Accent, type Density, type TextSize } from "@/lib/settings"
import { useShortcuts } from "@/lib/keybindings"
import { useTheme } from "next-themes"
import { api } from "@/lib/api-client"
import { cn } from "@/lib/utils"
import { CustomFieldsManager } from "@/components/CustomFieldsManager"
import { KeyboardPanel } from "@/components/settings/KeyboardPanel"
import { AuditPanel } from "@/components/settings/AuditPanel"
import { AboutPanel } from "@/components/settings/AboutPanel"
import { AccountPanel } from "@/components/settings/AccountPanel"
import { TokensPanel } from "@/components/settings/TokensPanel"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

const STATUSES = FILTERS.concat(MORE)
type Theme = "light" | "dark" | "system"

// Miniature app window: sidebar bar, header bar, two rows. Colors are fixed
// per theme so the preview shows that theme regardless of the current one.
const PALETTES = {
  light: { bg: "#ffffff", bar: "#e4e4e7", fg: "#18181b", accent: "#3b82f6" },
  dark: { bg: "#18181b", bar: "#3f3f46", fg: "#fafafa", accent: "#60a5fa" },
}

function Mini({ p, x, w }: { p: (typeof PALETTES)["light"]; x: number; w: number }) {
  return (
    <svg x={x} width={w} height={60} viewBox="0 0 60 60" preserveAspectRatio="none">
      <rect width="60" height="60" fill={p.bg} />
      <rect x="0" y="0" width="14" height="60" fill={p.bar} />
      <rect x="18" y="6" width="26" height="5" rx="2" fill={p.fg} opacity=".8" />
      <rect x="18" y="18" width="38" height="6" rx="2" fill={p.bar} />
      <rect x="18" y="28" width="38" height="6" rx="2" fill={p.bar} />
      <rect x="18" y="38" width="22" height="6" rx="2" fill={p.accent} />
    </svg>
  )
}

function ThemeCard({ value, current, onPick }: { value: Theme; current: Theme; onPick: (t: Theme) => void }) {
  return (
    <button
      type="button"
      onClick={() => onPick(value)}
      aria-pressed={current === value}
      className={cn(
        "flex w-28 shrink-0 flex-col gap-1.5 rounded-lg border p-1.5 text-left text-xs capitalize outline-none focus-visible:ring-2 focus-visible:ring-ring",
        current === value ? "border-primary ring-2 ring-primary/30" : "hover:bg-muted",
      )}
    >
      <svg viewBox="0 0 60 60" className="aspect-[3/2] w-full overflow-hidden rounded-md border" preserveAspectRatio="xMidYMid slice">
        {value === "system" ? (
          <>
            <Mini p={PALETTES.light} x={0} w={30} />
            <svg x={30} width={30} height={60} viewBox="30 0 30 60">
              <svg width={60} height={60} viewBox="0 0 60 60">
                <Mini p={PALETTES.dark} x={0} w={60} />
              </svg>
            </svg>
          </>
        ) : (
          <Mini p={PALETTES[value]} x={0} w={60} />
        )}
      </svg>
      <span className="px-0.5 font-medium">{value}</span>
    </button>
  )
}

function Row({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      <Label>{label}</Label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}

function Choice<T extends string>({
  pref,
  options,
}: {
  pref: Parameters<typeof getPref>[0]
  options: [T, string][]
}) {
  const [value, setValue] = useState(getPref<T>(pref))
  return (
    <ToggleGroup
      type="single"
      variant="outline"
      value={value}
      onValueChange={(v) => {
        if (!v) return
        setValue(v as T)
        setPref(pref, v)
      }}
    >
      {options.map(([v, label]) => (
        <ToggleGroupItem key={v} value={v}>
          {label}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}

function AccentPicker() {
  const [value, setValue] = useState(getPref<Accent>("accent"))
  return (
    <div className="flex flex-wrap gap-3">
      {(Object.keys(ACCENTS) as Accent[]).map((a) => (
        <button
          key={a}
          type="button"
          aria-label={a}
          aria-pressed={value === a}
          title={a}
          onClick={() => {
            setValue(a)
            setPref("accent", a)
          }}
          className={cn(
            "size-8 rounded-full border outline-none focus-visible:ring-2 focus-visible:ring-ring",
            value === a && "ring-2 ring-foreground ring-offset-2 ring-offset-background",
          )}
          style={{ background: `linear-gradient(135deg, ${ACCENTS[a][0]} 50%, ${ACCENTS[a][1]} 50%)` }}
        />
      ))}
    </div>
  )
}

function Appearance() {
  const { theme, setTheme } = useTheme()
  return (
    <div className="flex flex-col gap-6">
      <Row label="Theme" hint="System follows your operating system and switches automatically.">
        <div className="flex flex-wrap gap-3">
          {(["light", "dark", "system"] as const).map((t) => (
            <ThemeCard key={t} value={t} current={(theme as Theme) || "system"} onPick={setTheme} />
          ))}
        </div>
      </Row>
      <Row label="Accent color" hint="Buttons, focus rings and selected states. Each swatch shows its light and dark shade.">
        <AccentPicker />
      </Row>
      <Row label="Text size" hint="Scales the whole interface.">
        <Choice<TextSize> pref="textSize" options={[["small", "Small"], ["default", "Default"], ["large", "Large"]]} />
      </Row>
      <Row label="Motion" hint="Reduced turns off transitions and animations.">
        <Choice pref="motion" options={[["full", "Full"], ["reduced", "Reduced"]]} />
      </Row>
    </div>
  )
}

function TableDefaults() {
  const [status, setStatus] = useState(getDefaultStatus())
  const [sort, setSort] = useState(getDefaultSort())
  const [density, setDensityState] = useState(getPref<Density>("density"))
  return (
    <div className="flex flex-col gap-4">
      <Row label="Row density" hint="Compact fits more rows on screen.">
        <ToggleGroup
          type="single"
          variant="outline"
          value={density}
          onValueChange={(v) => {
            if (!v) return
            setDensityState(v as Density)
            setPref("density", v)
          }}
        >
          <ToggleGroupItem value="comfortable">
            Comfortable
          </ToggleGroupItem>
          <ToggleGroupItem value="compact">
            Compact
          </ToggleGroupItem>
        </ToggleGroup>
      </Row>
      <Row label="Striped rows" hint="Shade every other row to help scanning wide tables.">
        <Choice pref="stripes" options={[["off", "Off"], ["on", "On"]]} />
      </Row>
      <Row label="Default status filter" hint="Applied when you open the pipeline with no filter in the URL.">
        <Select
          value={status || "__all"}
          onValueChange={(value) => {
            const next = value === "__all" ? "" : value
            setStatus(next)
            setDefaultStatus(next)
          }}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all">All</SelectItem>
            {STATUSES.filter(([value]) => value).map(([value, text]) => (
              <SelectItem key={value} value={value}>
                {text}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Row>
      <Row label="Default sort">
        <Select
          value={sort}
          onValueChange={(value) => {
            setSort(value)
            setDefaultSort(value)
          }}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {SORTS.map(([value, text]) => (
              <SelectItem key={value} value={value}>
                {text}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Row>
    </div>
  )
}

function DataPanel() {
  const [busy, setBusy] = useState(false)
  async function exportJSON() {
    setBusy(true)
    try {
      const data = await api<unknown>("/v1/export/json")
      const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], { type: "application/json" }))
      const a = document.createElement("a")
      a.href = url
      a.download = `pursuit-${new Date().toISOString().slice(0, 10)}.json`
      a.click()
      URL.revokeObjectURL(url)
    } finally {
      setBusy(false)
    }
  }
  return (
    <Row label="Export" hint="Everything in your pipeline as one JSON file, re-importable with pursuit import.">
      <Button variant="outline" onClick={exportJSON} disabled={busy} className="self-start">
        {busy ? "Exporting…" : "Download JSON"}
      </Button>
    </Row>
  )
}

type Section = { id: string; label: string; icon: ReactNode; render: () => ReactNode }

// `items` feed the settings search: each is a setting label plus synonyms.
const SECTIONS: (Section & { items: [string, string][] })[] = [
  { id: "appearance", label: "Appearance", icon: <Palette />, render: () => <Appearance />, items: [
      ["Theme", "light dark system mode color"],
      ["Accent color", "primary highlight brand"],
      ["Text size", "font zoom larger smaller"],
      ["Motion", "animations reduce accessibility"],
    ] },
  {
    id: "table",
    label: "Table",
    icon: <Rows3 />,
    render: () => <TableDefaults />,
    items: [
      ["Default status filter", "pipeline stage"],
      ["Default sort", "order newest"],
      ["Row density", "compact comfortable spacing rows"],
      ["Striped rows", "zebra alternate shading"],
    ],
  },
  {
    id: "fields",
    label: "Custom fields",
    icon: <SlidersHorizontal />,
    render: () => <CustomFieldsManager active />,
    items: [["Custom fields", "columns attributes properties"]],
  },
  {
    id: "tokens",
    label: "API tokens",
    icon: <KeyRound />,
    render: () => <TokensPanel />,
    items: [
      ["Generate token", "api key cli agent extension"],
      ["Revoke token", "delete"],
    ],
  },
  {
    id: "account",
    label: "Account",
    icon: <User />,
    render: () => <AccountPanel />,
    items: [
      ["Password", "change security"],
      ["Email", "change address"],
      ["Sign out other sessions", "revoke devices security"],
      ["Display name", "profile email"],
      ["Sign out", "logout session"],
    ],
  },
  {
    id: "keys",
    label: "Keyboard",
    icon: <Keyboard />,
    render: () => <KeyboardPanel />,
    items: [["Keyboard shortcuts", "keybindings keys hotkeys vim enable disable"]],
  },
  {
    id: "audit",
    label: "Audit log",
    icon: <History />,
    render: () => <AuditPanel />,
    items: [["Audit log", "activity history changes who did what"]],
  },
  { id: "data", label: "Data", icon: <Database />, render: () => <DataPanel />, items: [["Export", "download backup json"]] },
  {
    id: "about",
    label: "About",
    icon: <Info />,
    render: () => <AboutPanel />,
    items: [["Version", "build server release docs api reference"]],
  },
]

// For the command palette: one entry per section, searchable by its settings.
export const SECTION_LINKS = SECTIONS.map((s) => ({
  id: s.id,
  label: s.label,
  icon: s.icon,
  keywords: s.items.map(([label, words]) => label + " " + words).join(" "),
}))

export function SettingsDialog({
  open,
  onOpenChange,
  initial = "appearance",
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  initial?: string
}) {
  const [active, setActive] = useState(initial)
  const [query, setQuery] = useState("")

  const hits = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return []
    return SECTIONS.flatMap((s) =>
      s.items
        .filter(([label, words]) => (s.label + " " + label + " " + words).toLowerCase().includes(q))
        .map(([label]) => ({ section: s, label })),
    )
  }, [query])

  const [sel, setSel] = useState(0)
  const pick = (id: string) => {
    setActive(id)
    setQuery("")
  }
  function onSearchKey(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Escape") {
      // Clear first, then leave the field. The dialog's own Esc is held back below.
      if (query) {
        setQuery("")
        setSel(0)
      } else e.currentTarget.blur()
      return
    }
    if (!hits.length) return
    const down = e.key === "ArrowDown" || (e.ctrlKey && e.key === "n")
    const up = e.key === "ArrowUp" || (e.ctrlKey && e.key === "p")
    if (down || up) {
      e.preventDefault()
      setSel((i) => (i + (down ? 1 : hits.length - 1)) % hits.length)
    } else if (e.key === "Enter") {
      e.preventDefault()
      pick(hits[Math.min(sel, hits.length - 1)].section.id)
    }
  }

  const current = SECTIONS.find((s) => s.id === active) || SECTIONS[0]
  const step = (d: number) => {
    setQuery("")
    setActive(SECTIONS[(SECTIONS.indexOf(current) + d + SECTIONS.length) % SECTIONS.length].id)
  }
  useShortcuts({ prevSection: () => step(-1), nextSection: () => step(1) })

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setQuery("")
        onOpenChange(next)
      }}
    >
      <DialogContent
        data-settings-dialog
        onEscapeKeyDown={(e) => {
          if ((e.target as HTMLElement).closest("[data-search]")) e.preventDefault()
        }}
        className="h-[90svh] w-[95vw] gap-0 overflow-hidden p-0 sm:max-w-6xl" showCloseButton>
        <DialogTitle className="sr-only">Settings</DialogTitle>
        <DialogDescription className="sr-only">Appearance, table defaults, API tokens, account, and about.</DialogDescription>
        <div className="grid h-full min-h-0 grid-cols-1 sm:grid-cols-[14rem_1fr]">
          <nav className="flex flex-col gap-2 border-b bg-muted/40 p-3 sm:border-r sm:border-b-0">
            <div className="relative">
              <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                data-search="settings"
                aria-label="Search settings"
                placeholder="Search settings"
                className="h-8 pl-8"
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value)
                  setSel(0)
                }}
                onKeyDown={onSearchKey}
              />
            </div>
            <div className="flex gap-1 overflow-x-auto sm:flex-col">
              {SECTIONS.map((s) => (
                <button
                  key={s.id}
                  type="button"
                  onClick={() => {
                    setActive(s.id)
                    setQuery("")
                  }}
                  aria-current={!query && s.id === current.id}
                  className={cn(
                    "flex shrink-0 items-center gap-2 rounded-md px-2.5 py-1.5 text-sm [&_svg]:size-4",
                    !query && s.id === current.id ? "bg-background font-medium shadow-sm ring-1 ring-foreground/10" : "text-muted-foreground hover:bg-muted",
                  )}
                >
                  {s.icon}
                  {s.label}
                </button>
              ))}
            </div>
          </nav>
          <section className="min-h-0 overflow-y-auto p-5 pr-12">
            {query ? (
              <div className="flex flex-col gap-1">
                <h2 className="mb-2 text-base font-medium">Results for “{query}” <span className="text-xs font-normal text-muted-foreground">↑↓ or Ctrl-N/P to move, Enter to open</span></h2>
                {hits.length === 0 && <p className="text-sm text-muted-foreground">No matching settings.</p>}
                {hits.map(({ section, label }, i) => (
                  <button
                    key={section.id + label}
                    type="button"
                    aria-selected={i === sel}
                    ref={(el) => {
                      if (i === sel) el?.scrollIntoView({ block: "nearest" })
                    }}
                    className={cn(
                      "flex items-center justify-between rounded-md px-2 py-2 text-left text-sm hover:bg-muted",
                      i === sel && "bg-muted ring-1 ring-foreground/10",
                    )}
                    onMouseEnter={() => setSel(i)}
                    onClick={() => pick(section.id)}
                  >
                    {label}
                    <span className="text-xs text-muted-foreground">{section.label}</span>
                  </button>
                ))}
              </div>
            ) : (
              <>
                <h2 className="mb-4 text-base font-medium">{current.label}</h2>
                {current.render()}
              </>
            )}
          </section>
        </div>
      </DialogContent>
    </Dialog>
  )
}
