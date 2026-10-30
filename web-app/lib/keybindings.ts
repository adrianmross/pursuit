import { useEffect, useRef, useSyncExternalStore } from "react"

// combo: "mod+k" (⌘ on Mac, Ctrl elsewhere), a bare key ("/"), or two steps
// separated by a space ("g h"). when: "app" = no dialog open, "dialog" = a
// dialog (settings) is open, "any" = always.
export type Binding = { id: string; label: string; group: string; combo: string; when: "app" | "dialog" | "any" }

export const BINDINGS: Binding[] = [
  { id: "palette", label: "Open command palette", group: "General", combo: "mod+k", when: "any" },
  { id: "search", label: "Search what you are looking at", group: "General", combo: "/", when: "any" },
  { id: "settings", label: "Open settings", group: "General", combo: "mod+,", when: "any" },
  { id: "keys", label: "Show keyboard shortcuts", group: "General", combo: "?", when: "any" },
  { id: "home", label: "Go to home", group: "Navigation", combo: "g h", when: "app" },
  { id: "pipeline", label: "Go to pipeline", group: "Navigation", combo: "g p", when: "app" },
  { id: "archived", label: "Go to archived", group: "Navigation", combo: "g a", when: "app" },
  { id: "new", label: "Add application", group: "Navigation", combo: "n", when: "app" },
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
export function keyLabels(step: string) {
  return step.split("+").map((k) => (k === "mod" ? (isMac ? "⌘" : "Ctrl") : k.length === 1 ? k.toUpperCase() : k))
}

function matches(e: KeyboardEvent, step: string) {
  if (step.startsWith("mod+")) return (e.metaKey || e.ctrlKey) && e.key.toLowerCase() === step.slice(4)
  return !e.metaKey && !e.ctrlKey && !e.altKey && e.key === step
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

      const active = BINDINGS.filter((b) => {
        if (!ref.current[b.id] || prefs.off.includes(b.id)) return false
        if (b.when === "app" && dialog) return false
        if (b.when === "dialog" && !dialog) return false
        return !typing || b.combo.startsWith("mod+")
      })

      function fire(b: Binding) {
        pending = ""
        window.clearTimeout(timer)
        if (ref.current[b.id]?.() !== false) e.preventDefault()
      }

      if (pending) {
        const first = pending
        pending = ""
        const hit = active.find((b) => b.combo.split(" ")[0] === first && b.combo.split(" ")[1] && matches(e, b.combo.split(" ")[1]))
        if (hit) return fire(hit)
      }
      for (const b of active) {
        const [a, second] = b.combo.split(" ")
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
