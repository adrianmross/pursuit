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

export type TabPlace = "tab" | "card" | "off"
export const OUTLINE_TABS = [
  ["rounds", "Rounds", "tabRounds"],
  ["tasks", "Tasks", "tabTasks"],
  ["questions", "Questions", "tabQuestions"],
  ["notes", "Notes", "tabNotes"],
] as const
export const PAGE_CARDS = [
  ["cardListing", "Listing", "The posting's fields under the header: link, dates and custom fields."],
  ["cardRelated", "Related postings", "Other postings at the company, and what this one is tracked with."],
  ["cardArtifacts", "Documents", "Résumés, cover letters and files."],
  ["cardHistory", "History", "Every change to the application."],
] as const
