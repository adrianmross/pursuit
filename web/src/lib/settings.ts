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
