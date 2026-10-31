import { useEffect, useRef, useSyncExternalStore } from "react"

// combo: "mod+k" (⌘ on Mac, Ctrl elsewhere), a bare key ("/"), or two steps
// separated by a space ("g h"). "|" lists alternatives ("j|ArrowDown"), and
// "ctrl+n" is Control only, where "mod+" is ⌘ on a Mac. when: "app" = no dialog open, "dialog" = a
// dialog (settings) is open, "any" = always.
export type Binding = { id: string; label: string; group: string; combo: string; when: "app" | "dialog" | "any" }

export const BINDINGS: Binding[] = [
  { id: "palette", label: "Open command palette", group: "General", combo: "mod+k", when: "any" },
  { id: "search", label: "Search what you are looking at", group: "General", combo: "/", when: "any" },
  { id: "settings", label: "Open settings", group: "General", combo: "mod+,", when: "any" },
  { id: "keys", label: "Show keyboard shortcuts", group: "General", combo: "?", when: "any" },
  { id: "home", label: "Go to start", group: "Navigation", combo: "g h", when: "app" },
  { id: "pipeline", label: "Go to applications", group: "Navigation", combo: "g p", when: "app" },
  { id: "archived", label: "Go to archived", group: "Navigation", combo: "g a", when: "app" },
  { id: "back", label: "Back to all applications", group: "Navigation", combo: "Escape", when: "app" },
  { id: "new", label: "Add application", group: "Navigation", combo: "n", when: "app" },
  { id: "rowNext", label: "Next application in the table", group: "Table", combo: "j|ArrowDown|ctrl+n", when: "app" },
  { id: "rowPrev", label: "Previous application in the table", group: "Table", combo: "k|ArrowUp|ctrl+p", when: "app" },
  { id: "rowOpen", label: "Open the highlighted application", group: "Table", combo: "Enter", when: "app" },
  { id: "rowMenu", label: "Quick actions for the highlighted application (then S status, A archive, D delete)", group: "Table", combo: ".|m", when: "app" },
  { id: "rowStatusNext", label: "Move the highlighted application to the next status", group: "Table", combo: "]", when: "app" },
  { id: "rowStatusPrev", label: "Move the highlighted application to the previous status", group: "Table", combo: "[", when: "app" },
  { id: "rowSelect", label: "Select the highlighted application", group: "Table", combo: "x|Space", when: "app" },
  { id: "rowClear", label: "Clear the selection and highlight", group: "Table", combo: "Escape", when: "app" },
  { id: "appStatus", label: "Change the status", group: "Application", combo: "s", when: "app" },
  { id: "appStageNext", label: "Move to the next stage", group: "Application", combo: "]", when: "app" },
  { id: "appStagePrev", label: "Move to the previous stage", group: "Application", combo: "[", when: "app" },
  { id: "appOutcome", label: "Set an outcome (then 1-6)", group: "Application", combo: "o", when: "app" },
  { id: "appRounds", label: "Edit every stage's rounds", group: "Application", combo: "r", when: "app" },
  { id: "appRoundAdd", label: "Add a round to the current stage", group: "Application", combo: "+|=", when: "app" },
  { id: "appRoundRemove", label: "Remove the current stage's last round", group: "Application", combo: "-", when: "app" },
  { id: "appPosting", label: "Open the posting", group: "Application", combo: "p", when: "app" },
  { id: "appApprove", label: "Approve, when prepared", group: "Application", combo: "y", when: "app" },
  { id: "appReject", label: "Reject, when prepared", group: "Application", combo: "x", when: "app" },
  { id: "appArchive", label: "Archive or unarchive", group: "Application", combo: "a", when: "app" },
  { id: "find", label: "Find in the audit log", group: "Settings", combo: "mod+f", when: "dialog" },
  { id: "prevSection", label: "Previous settings section", group: "Settings", combo: "[", when: "dialog" },
  { id: "nextSection", label: "Next settings section", group: "Settings", combo: "]", when: "dialog" },
]

const KEY = "pursuit.keys"
type Prefs = { master: boolean; off: string[] }

function load(): Prefs {
  try {
    const raw = JSON.parse(localStorage.getItem(KEY) || "{}")
    return { master: raw.master !== false, off: Array.isArray(raw.off) ? raw.off : [] }
  } catch {
    return { master: true, off: [] }
  }
}

let prefs = load()
const listeners = new Set<() => void>()

function save(next: Prefs) {
  prefs = next
  localStorage.setItem(KEY, JSON.stringify(next))
  listeners.forEach((l) => l())
}

export function useKeyPrefs() {
  const current = useSyncExternalStore(
    (cb) => {
      listeners.add(cb)
      return () => void listeners.delete(cb)
    },
    () => prefs,
  )
  return {
    master: current.master,
    isOn: (id: string) => !current.off.includes(id),
    setMaster: (on: boolean) => save({ ...prefs, master: on }),
    setOn: (id: string, on: boolean) =>
      save({ ...prefs, off: on ? prefs.off.filter((x) => x !== id) : [...prefs.off, id] }),
  }
}

export const isMac = typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform)

// "mod+k" -> ["⌘", "K"]; "g h" -> [["G"], ["H"]] is rendered as steps by the caller.
const NAMES: Record<string, string> = { Escape: "Esc", ArrowDown: "↓", ArrowUp: "↑", ctrl: "Ctrl", Space: "Space" }
export function keyLabels(step: string) {
  if (step === "+") return ["+"]
  return step.split("+").map((k) => (k === "mod" ? (isMac ? "⌘" : "Ctrl") : NAMES[k] || (k.length === 1 ? k.toUpperCase() : k)))
}

function matches(e: KeyboardEvent, step: string) {
  if (step.startsWith("ctrl+")) return e.ctrlKey && !e.metaKey && e.key.toLowerCase() === step.slice(5)
  if (step.startsWith("mod+")) return (e.metaKey || e.ctrlKey) && e.key.toLowerCase() === step.slice(4)
  return !e.metaKey && !e.ctrlKey && !e.altKey && e.key === (step === "Space" ? " " : step)
}

// A handler may return false to say it did nothing, leaving the key to the browser.
export function useShortcuts(handlers: Partial<Record<string, () => void | false>>) {
  const ref = useRef(handlers)
  useEffect(() => {
    ref.current = handlers
  })

  useEffect(() => {
    let pending = ""
    let timer = 0

    function onKey(e: KeyboardEvent) {
      if (e.isComposing || e.defaultPrevented || !prefs.master) return
      const target = e.target as HTMLElement
      const typing = !!target.closest?.("input, textarea, select, [contenteditable='true']")
      const dialog = !!document.querySelector('[role="dialog"]')

      // One entry per alternative key, so "j|ArrowDown" is two shortcuts.
      const active = BINDINGS.filter((b) => {
        if (!ref.current[b.id] || prefs.off.includes(b.id)) return false
        if (b.when === "app" && dialog) return false
        if (b.when === "dialog" && !dialog) return false
        // Esc belongs to an open menu or select first.
        if (b.combo === "Escape" && document.querySelector('[role="menu"], [role="listbox"]')) return false
        return true
      }).flatMap((b) =>
        b.combo
          .split("|")
          .filter((alt) => !typing || alt.startsWith("mod+"))
          .map((alt) => ({ b, alt })),
      )

      function fire(b: Binding) {
        pending = ""
        window.clearTimeout(timer)
        if (ref.current[b.id]?.() !== false) e.preventDefault()
      }

      if (pending) {
        const first = pending
        pending = ""
        const hit = active.find(({ alt }) => alt.split(" ")[0] === first && alt.split(" ")[1] && matches(e, alt.split(" ")[1]))
        if (hit) return fire(hit.b)
      }
      for (const { b, alt } of active) {
        const [a, second] = alt.split(" ")
        if (!matches(e, a)) continue
        if (second) {
          pending = a
          window.clearTimeout(timer)
          timer = window.setTimeout(() => (pending = ""), 1000)
          return
        }
        return fire(b)
      }
    }

    window.addEventListener("keydown", onKey)
    return () => {
      window.removeEventListener("keydown", onKey)
      window.clearTimeout(timer)
    }
  }, [])
}
