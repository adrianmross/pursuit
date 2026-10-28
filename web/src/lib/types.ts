export type Application = {
  id: string
  company_id?: string
  company?: string
  title?: string
  status: string
  link_status?: string
  posting_url?: string
  applied_date?: string
  updated_at?: string
  created_at?: string
  tags?: string[]
  notes?: string
  location?: string
  salary?: string
  fit?: string
  source?: string
  next_step?: string
  archived_at?: string
  custom_fields?: Record<string, string>
  version?: number
}

export type CustomField = {
  id: string
  name: string
  type: "text" | "number" | "date"
  created_at?: string
}

export type Page<T> = {
  data: T[]
  page: { total: number; has_more: boolean; next_cursor?: string }
}

export type Me = {
  token: { name?: string; scopes?: string[] }
}

export type Question = {
  prompt?: string
  answer?: string
  required?: boolean
  unknown?: boolean
  source?: string
}

export type Artifact = {
  id: string
  filename?: string
  kind?: string
  content_type?: string
  body?: string
}

export type Event = {
  title?: string
  kind?: string
  occurred_at?: string
  actor?: { name?: string }
  from_status?: string
  to_status?: string
  body?: string
}

export type Audit = {
  id: string
  actor_name?: string
  action: string
  entity_type: string
  entity_id: string
  summary: string
  created_at: string
}

export const FILTERS: [string, string][] = [
  ["", "All"],
  ["to_apply", "To apply"],
  ["awaiting_review", "Prepared"],
  ["applied", "Applied"],
  ["interviewing", "Interviewing"],
  ["offer", "Offer"],
  ["rejected", "Rejected"],
]

export const MORE: [string, string][] = [
  ["saved", "Saved"],
  ["preparing", "Preparing"],
  ["needs_input", "Needs input"],
  ["approved", "Approved"],
  ["submitting", "Submitting"],
  ["submitted", "Submitted"],
  ["failed", "Failed"],
  ["in_progress", "In progress"],
  ["screening", "Screening"],
  ["withdrawn", "Withdrawn"],
  ["ghosted", "Ghosted"],
]

export const SORTS: [string, string][] = [
  ["-updated_at", "Recently updated"],
  ["updated_at", "Least recently updated"],
  ["company", "Company A–Z"],
  ["-company", "Company Z–A"],
  ["title", "Title A–Z"],
  ["status", "Status"],
  ["-created_at", "Newest"],
  ["created_at", "Oldest"],
  ["-applied_date", "Applied date"],
  ["applied_date", "Applied date, earliest"],
  // Category isn't a backend sort column — applied client-side over the
  // currently loaded page instead of being forwarded as a `sort` param.
  ["category", "Category"],
]

export const LABELS: Record<string, string> = {
  saved: "Saved",
  to_apply: "To apply",
  preparing: "Preparing",
  awaiting_review: "Prepared",
  approved: "Approved",
  submitting: "Submitting",
  submitted: "Submitted",
  needs_input: "Needs input",
  failed: "Failed",
  in_progress: "In progress",
  applied: "Applied",
  screening: "Screening",
  interviewing: "Interviewing",
  offer: "Offer",
  rejected: "Rejected",
  withdrawn: "Withdrawn",
  ghosted: "Ghosted",
}

export function label(status?: string) {
  return (status && LABELS[status]) || status || "—"
}

export const SOURCE_LABELS: Record<string, string> = {
  greenhouse: "Greenhouse",
  ashby: "Ashby",
  lever: "Lever",
  workday: "Workday",
  company_site: "Company site",
  other: "Other",
}

export function sourceLabel(source?: string) {
  return (source && SOURCE_LABELS[source]) || ""
}

// Pursuit's status chips are told apart by label, not hue — the only color
// is brand-gold, and only for the one status that's a win. Everything else
// is a plain neutral chip.
export function statusTone(status?: string): "win" | "default" {
  return status === "offer" ? "win" : "default"
}

export type StatusCategory = "action" | "waiting" | "closed" | "rejected"

export const CATEGORY_LABELS: Record<StatusCategory, string> = {
  action: "Needs attention",
  waiting: "Waiting",
  closed: "Closed",
  rejected: "Rejected",
}

export const CATEGORY_ORDER: StatusCategory[] = ["action", "waiting", "closed", "rejected"]

const STATUS_CATEGORY: Record<string, StatusCategory> = {
  to_apply: "action",
  awaiting_review: "action",
  needs_input: "action",
  failed: "action",
  offer: "action",
  saved: "waiting",
  preparing: "waiting",
  approved: "waiting",
  submitting: "waiting",
  submitted: "waiting",
  in_progress: "waiting",
  screening: "waiting",
  applied: "waiting",
  interviewing: "waiting",
  withdrawn: "closed",
  ghosted: "closed",
  rejected: "rejected",
}

export function statusCategory(status?: string): StatusCategory {
  return (status && STATUS_CATEGORY[status]) || "waiting"
}

export function statusesInCategory(category: StatusCategory): string[] {
  return Object.keys(STATUS_CATEGORY).filter((s) => STATUS_CATEGORY[s] === category)
}
