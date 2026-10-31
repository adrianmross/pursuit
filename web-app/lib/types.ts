export type Application = {
  id: string
  company_id?: string
  company?: string
  title?: string
  status: string
  // An outcome overlays the stage instead of replacing it: waiting and
  // needs_input hold the application, the rest end the pursuit.
  outcome?: string
  stage?: number
  // Rounds per stage number (1-6): done and total.
  interviews?: Record<string, { done: number; total: number }>
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

export type Interview = {
  id: string
  application_id: string
  stage: string
  kind?: string
  interviewer?: string
  scheduled_at?: string
  done_at?: string
  prep?: string
  notes?: string
  version: number
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
  user?: { id?: string; name?: string; email?: string; avatar?: string }
  token: { name?: string; scopes?: string[] }
}

export type StandingAnswer = { key: string; answer: string; updated_at?: string }

export type Document = { id: string; kind: string; filename?: string; content_type?: string; created_at?: string }

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
  created_at?: string
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

export type ImportResult = {
  rows: number
  companies_created: number
  companies_existing: number
  roles_created: number
  roles_updated: number
  applications_created: number
  applications_updated: number
  skipped: number
  warnings?: string[]
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
  ["to_apply", "Apply"],
  ["awaiting_review", "Prepared"],
  ["applied", "Applied"],
  ["screening", "Screening"],
  ["interviewing", "Interviewing"],
  ["final", "Final"],
  ["offer", "Offer"],
  ["rejected", "Rejected"],
]

export const MORE: [string, string][] = [
  ["saved", "Saved"],
  ["preparing", "Preparing"],
  ["needs_input", "Needs input"],
  ["waiting", "Waiting"],
  ["approved", "Approved"],
  ["submitting", "Submitting"],
  ["submitted", "Submitted"],
  ["failed", "Failed"],
  ["in_progress", "In progress"],
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
  to_apply: "Apply",
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
  final: "Final",
  waiting: "Waiting",
  offer: "Offer",
  rejected: "Rejected",
  withdrawn: "Withdrawn",
  ghosted: "Ghosted",
}

// The status an application displays: its outcome when set, else its stage.
export function shown(app: Pick<Application, "status" | "outcome">) {
  return app.outcome || app.status
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

// Hire Pursuit's status chips are told apart by lightness and outline, not
// hue — gold is reserved for the one status that's a win. The two outcome
// families are the exception: hold (amber) and stop (red) sit on top of a stage.
export type StatusTone = "default" | "attention" | "strong" | "danger" | "muted" | "win"

export type OutcomeKind = "hold" | "stop" | ""

export function outcomeKind(status?: string): OutcomeKind {
  switch (status) {
    case "waiting":
    case "needs_input":
      return "hold"
    case "failed":
    case "rejected":
    case "withdrawn":
    case "ghosted":
      return "stop"
    default:
      return ""
  }
}

const STATUS_TONE: Record<string, StatusTone> = {
  to_apply: "default",
  preparing: "default",
  needs_input: "attention",
  failed: "danger",
  awaiting_review: "attention",
  approved: "strong",
  submitting: "default",
  submitted: "default",
  in_progress: "default",
  applied: "default",
  screening: "default",
  interviewing: "default",
  offer: "win",
  withdrawn: "danger",
  rejected: "danger",
  ghosted: "danger",
  waiting: "attention",
  final: "default",
  saved: "default",
}

export function statusTone(status?: string): StatusTone {
  return (status && STATUS_TONE[status]) || "default"
}

// Stage of the progress mark (0-6; 6 is the gold tail) a status has earned.
// Outcomes have no stage of their own; the server keeps the stage beside them.
const STATUS_PIECES: Record<string, number> = {
  awaiting_review: 1,
  approved: 1,
  submitting: 1,
  submitted: 2,
  applied: 2,
  in_progress: 2,
  screening: 3,
  interviewing: 4,
  final: 5,
  offer: 6,
}

export function statusPieces(status?: string): number {
  return status ? STATUS_PIECES[status] ?? 0 : 0
}

// What the progress mark draws for an application.
export function markOf(app: Application) {
  return {
    stage: app.stage ?? statusPieces(app.status),
    outcome: app.outcome,
    interviews: app.interviews,
  }
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
  final: "waiting",
  waiting: "waiting",
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

export type Token = {
  id: string
  name: string
  prefix: string
  scopes: string[]
  created_at: string
  last_used_at: string | null
  revoked_at: string | null
}
