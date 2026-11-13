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
