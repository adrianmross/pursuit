const DEFAULT_STATUS_KEY = "pursuit.defaultStatus"
const DEFAULT_SORT_KEY = "pursuit.defaultSort"

export function getDefaultStatus() {
  return localStorage.getItem(DEFAULT_STATUS_KEY) || ""
}

export function setDefaultStatus(value: string) {
  if (value) localStorage.setItem(DEFAULT_STATUS_KEY, value)
  else localStorage.removeItem(DEFAULT_STATUS_KEY)
}

export function getDefaultSort() {
  return localStorage.getItem(DEFAULT_SORT_KEY) || "-updated_at"
}

export function setDefaultSort(value: string) {
  localStorage.setItem(DEFAULT_SORT_KEY, value)
}

const DENSITY_KEY = "pursuit.density"
export type Density = "comfortable" | "compact"

export function getDensity(): Density {
  return localStorage.getItem(DENSITY_KEY) === "compact" ? "compact" : "comfortable"
}

export function setDensity(value: Density) {
  localStorage.setItem(DENSITY_KEY, value)
  applyDensity()
}

// Read by the table rules in index.css.
export function applyDensity() {
  document.documentElement.dataset.density = getDensity()
}
