"use client"

import { useEffect, useState } from "react"
import { CreditCard, Database, Download, FlaskConical, Info, LayoutPanelTop, Rows3, ScanSearch, SlidersHorizontal } from "lucide-react"
import { api, AppearanceSettings, platformSections, SettingsChoice as Choice, SettingsRow as Row, type SettingsSection } from "@adrianmross/appkit-ui"
import { FILTERS, MORE, SORTS } from "@/lib/types"
import { getDefaultSort, getDefaultStatus, OUTLINE_TABS, PAGE_CARDS, setDefaultSort, setDefaultStatus, type TabPlace } from "@/lib/settings"
import { CustomFieldsManager } from "@/components/CustomFieldsManager"
import { SearchPanel } from "@/components/settings/SearchPanel"
import { AboutPanel } from "@/components/settings/AboutPanel"
import { ExperimentalPanel, PlanPanel } from "@/components/settings/ExperimentalPanel"
import { Button } from "@adrianmross/appkit-ui/ui/button"
import { FieldGroup } from "@adrianmross/appkit-ui/ui/field"
import { Spinner } from "@adrianmross/appkit-ui/ui/spinner"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@adrianmross/appkit-ui/ui/select"

const STATUSES = FILTERS.concat(MORE)

function FaviconService() {
  const [serverDefault, setServerDefault] = useState("")
  useEffect(() => {
    api<{ favicon_service?: string }>("/version").then((v) => setServerDefault(v.favicon_service || "")).catch(() => {})
  }, [])
  return (
    <Row
      label="Company icons"
      hint={`When a site has no usable icon of its own, fall back to Google's favicon service. That tells Google which company domains you track. Off shows the company's initial instead. Default follows this server${serverDefault ? ` (${serverDefault})` : ""}. Changes apply after a reload.`}
    >
      <Choice pref="faviconService" options={[["default", "Default"], ["on", "On"], ["off", "Off"]]} />
    </Row>
  )
}

// Pursuit's appearance settings, after the ones every app has.
function Appearance() {
  return (
    <AppearanceSettings>
      <FaviconService />
      <Row label="Round counter" hint="How a stage's number of rounds is set: a wheel you scroll or click, or a die you roll and turn.">
        <Choice pref="roundCounter" options={[["wheel", "Wheel"], ["dice", "Dice"]]} />
      </Row>
    </AppearanceSettings>
  )
}

// What an application's page shows: each card on or off, and each tab of
// the Details card kept as a tab, given a card of its own, or hidden.
function DetailsPanel() {
  return (
    <FieldGroup>
      <Row label="Details tabs" hint="A tab stays inside the card. Own card moves it out to a card of its own below, and All becomes an overview of the rest.">
        <div className="flex flex-col gap-2">
          {OUTLINE_TABS.map(([, label, pref]) => (
            <div key={pref} className="flex items-center justify-between gap-4 text-sm">
              {label}
              <Choice<TabPlace> pref={pref} options={[["tab", "Tab"], ["card", "Own card"], ["off", "Off"]]} />
            </div>
          ))}
        </div>
      </Row>
      {PAGE_CARDS.map(([pref, label, hint]) => (
        <Row key={pref} label={label} hint={hint}>
          <Choice pref={pref} options={[["on", "Show"], ["off", "Hide"]]} />
        </Row>
      ))}
    </FieldGroup>
  )
}

function TableDefaults() {
  const [status, setStatus] = useState(getDefaultStatus())
  const [sort, setSort] = useState(getDefaultSort())
  return (
    <FieldGroup>
      <Row label="Stage funnel" hint="A bar of every stage above the table, sized by how many roles sit on each. Click one to filter.">
        <Choice pref="funnel" options={[["off", "Off"], ["on", "On"]]} />
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
    </FieldGroup>
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
        {busy ? <Spinner /> : <Download />}
        {busy ? "Exporting…" : "Download JSON"}
      </Button>
    </Row>
  )
}

// Every settings section, in the order the dialog lists them: the platform's
// (appearance, tokens, connected apps, account, keyboard, audit log) with
// Pursuit's own between them. `items` feed the settings search.
export const SECTIONS: SettingsSection[] = [
  {
    ...platformSections.appearance,
    render: () => <Appearance />,
    items: [...platformSections.appearance.items, ["Company icons", "favicon logo google service privacy fallback"]],
  },
  {
    id: "details",
    label: "Details",
    icon: <LayoutPanelTop />,
    sub: true,
    render: () => <DetailsPanel />,
    items: [
      ["Application page cards", "listing related documents history hide show"],
      ["Details tabs", "rounds tasks questions notes own card separate overview"],
    ],
  },
  {
    id: "table",
    label: "Table",
    icon: <Rows3 />,
    render: () => <TableDefaults />,
    items: [
      ["Default status filter", "pipeline stage"],
      ["Default sort", "order newest"],
      ["Stage funnel", "pipeline bar counts stages overview"],
    ],
  },
  {
    id: "fields",
    label: "Custom fields",
    icon: <SlidersHorizontal />,
    render: () => <CustomFieldsManager active />,
    items: [["Custom fields", "columns attributes properties"]],
  },
  platformSections.tokens,
  platformSections.apps,
  platformSections.account,
  {
    id: "search",
    label: "Search",
    icon: <ScanSearch />,
    render: (close) => <SearchPanel close={close} />,
    items: [
      ["Search syntax", "qualifiers status tag company title applied operators filter query enable disable"],
      ["Test a search", "try example parse"],
    ],
  },
  platformSections.keys,
  platformSections.audit,
  {
    id: "experimental",
    label: "Experimental",
    icon: <FlaskConical />,
    render: () => <ExperimentalPanel />,
    items: [
      ["Experimental features", "labs beta preview early try enable disable toggle"],
      ["Smart search", "vector semantic meaning answers notes documents ai"],
      ["Tailored documents", "resume cover letter tailored per application compare diff default"],
    ],
  },
  {
    id: "plan",
    label: "Plan",
    icon: <CreditCard />,
    render: () => <PlanPanel />,
    items: [["Plan", "billing subscription upgrade plus premium pay stripe cancel invoice"]],
  },
  { id: "data", label: "Data", icon: <Database />, render: () => <DataPanel />, items: [["Export", "download backup json"]] },
  {
    id: "about",
    label: "About",
    icon: <Info />,
    render: (close) => <AboutPanel onClose={close} />,
    items: [
      ["Version", "build server release docs api reference"],
      ["Tutorial", "demo sandbox try replay example onboarding tour"],
    ],
  },
]
