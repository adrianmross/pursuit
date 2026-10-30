"use client"

import { Fragment } from "react"
import { BINDINGS, keyLabels, useKeyPrefs } from "@/lib/keybindings"
import { Kbd, KbdGroup } from "@/components/ui/kbd"
import { Switch } from "@/components/ui/switch"

export function Keys({ combo }: { combo: string }) {
  return (
    <span className="flex items-center gap-1.5">
      {combo.split(" ").map((step, i) => (
        <Fragment key={i}>
          {i > 0 && <span className="text-xs text-muted-foreground">then</span>}
          <KbdGroup>
            {keyLabels(step).map((k) => (
              <Kbd key={k}>{k}</Kbd>
            ))}
          </KbdGroup>
        </Fragment>
      ))}
    </span>
  )
}

export function KeyboardPanel() {
  const { master, isOn, setMaster, setOn } = useKeyPrefs()
  const groups = [...new Set(BINDINGS.map((b) => b.group))]
  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between gap-4 rounded-md border p-3">
        <div className="flex flex-col">
          <span className="text-sm font-medium">Keyboard shortcuts</span>
          <span className="text-xs text-muted-foreground">
            Turn everything off at once. Single-key shortcuts never fire while you are typing in a field.
          </span>
        </div>
        <Switch checked={master} onCheckedChange={setMaster} aria-label="Enable keyboard shortcuts" />
      </div>
      {groups.map((group) => (
        <div key={group} className="flex flex-col gap-2">
          <h3 className="text-sm font-medium">{group}</h3>
          <div className="divide-y rounded-md border text-sm">
            {BINDINGS.filter((b) => b.group === group).map((b) => (
              <div key={b.id} className={"flex items-center gap-4 p-3 " + (master ? "" : "opacity-50")}>
                <span className="flex-1">{b.label}</span>
                <Keys combo={b.combo} />
                <Switch checked={isOn(b.id)} disabled={!master} onCheckedChange={(on) => setOn(b.id, on)} aria-label={`Enable: ${b.label}`} />
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
