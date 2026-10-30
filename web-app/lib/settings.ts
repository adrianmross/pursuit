const DEFAULT_STATUS_KEY = "pursuit.defaultStatus"
const DEFAULT_SORT_KEY = "pursuit.defaultSort"

// Next prerenders a client component's initial render server-side too (for
// the static HTML shell), where there's no localStorage — these guard that,
// same pattern as lib/api-client.ts.
export function getDefaultStatus() {
  if (typeof window === "undefined") return ""
  return localStorage.getItem(DEFAULT_STATUS_KEY) || ""
}

export function setDefaultStatus(value: string) {
  if (value) localStorage.setItem(DEFAULT_STATUS_KEY, value)
  else localStorage.removeItem(DEFAULT_STATUS_KEY)
}

export function getDefaultSort() {
  if (typeof window === "undefined") return "-updated_at"
  return localStorage.getItem(DEFAULT_SORT_KEY) || "-updated_at"
}

export function setDefaultSort(value: string) {
  localStorage.setItem(DEFAULT_SORT_KEY, value)
}

const PREFIX = "pursuit."

export const ACCENTS = {
  neutral: ["#141615", "#F1F1EE"],
  forest: ["#173A2E", "#7CCBA8"],
  blue: ["#2563EB", "#60A5FA"],
  violet: ["#7C3AED", "#A78BFA"],
  rose: ["#E11D48", "#FB7185"],
  amber: ["#B45309", "#F5B84A"],
} as const
export type Accent = keyof typeof ACCENTS
export type Density = "comfortable" | "compact"
export type TextSize = "small" | "default" | "large"

const SIZES: Record<TextSize, string> = { small: "14px", default: "16px", large: "18px" }

const DEFAULTS = { density: "comfortable", accent: "neutral", textSize: "default", motion: "full", stripes: "off", searchSyntax: "on" }
type Pref = keyof typeof DEFAULTS

function read(key: Pref) {
  // Server rendering has no localStorage: fall back to the default.
  if (typeof localStorage === "undefined") return DEFAULTS[key]
  return localStorage.getItem(PREFIX + key) || DEFAULTS[key]
}

export function getPref<T extends string>(key: Pref) {
  return read(key) as T
}

export function setPref(key: Pref, value: string) {
  if (value === DEFAULTS[key]) localStorage.removeItem(PREFIX + key)
  else localStorage.setItem(PREFIX + key, value)
  applyAppearance()
}

// Reflect every appearance preference onto <html>; index.css reads these.
export function applyAppearance() {
  const root = document.documentElement
  root.dataset.density = read("density")
  root.dataset.accent = read("accent")
  root.dataset.motion = read("motion")
  root.dataset.stripes = read("stripes")
  root.style.fontSize = SIZES[read("textSize") as TextSize] || SIZES.default
}
