import { LABELS, statusPieces } from "./status"

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
  // Tasks per stage number (0-6): done and total.
  tasks?: Record<string, { done: number; total: number }>
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
  job_type?: string
  contact_email?: string
  // The application at the same company whose process covers this posting.
  tracked_by?: string
  // This application's own names for its round stages and the ones it skips.
  stage_plan?: { labels?: Record<string, string>; off?: string[] }
  resume_version?: string
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
  // One markdown document: prep, transcript and questions.
  notes?: string
  // Parsed from notes: a heading ending in ? and the text under it.
  questions?: { prompt: string; answer: string }[]
  // false folds the round to one line in the notes outline.
  in_notes?: boolean
  version: number
}

// A notes file of its own on an application, beside its notes and rounds.
export type Note = {
  id: string
  application_id: string
  name: string
  body: string
  questions?: { prompt: string; answer: string }[]
  version: number
  created_at?: string
  updated_at?: string
}

// A question from a round's notes, with where it was asked.
export type BankQuestion = {
  prompt: string
  answer: string
  round_id: string
  application_id: string
  company: string
  title: string
  stage: string
  kind: string
  updated_at: string
}

// A to-do on an application, things waiting on you, optionally on a stage.
export type Task = {
  id: string
  application_id: string
  stage: string
  // The round this task belongs to, if any; empty means its whole stage.
  round_id?: string
  title: string
  notes?: string
  due_at?: string
  done_at?: string
  version: number
}

// Open tasks across every stage.
export function openTasks(app: Pick<Application, "tasks">) {
  return Object.values(app.tasks ?? {}).reduce((n, c) => n + c.total - c.done, 0)
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
  user?: { id?: string; name?: string; email?: string; avatar?: string; created_at?: string }
  token: { id?: string; name?: string; scopes?: string[] }
  // The PURSUIT_TRUST_PROXY_AUTH owner, who may change their email.
  proxy?: boolean
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
  id?: string
  application_id?: string
  title?: string
  kind?: string
  occurred_at?: string
  actor?: { name?: string; token_id?: string }
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

// Index = piece, so STAGE_STOPS[app.stage] is the stage an app sits on.
export const STAGE_STOPS: [string, string][] = ["to_apply", "awaiting_review", "applied", "screening", "interviewing", "final", "offer"].map(
  (s) => [s, LABELS[s]],
)

// Rounds attach to pieces 1-5, keyed by the stage's status name. Offer has none.
export const ROUND_STAGES = STAGE_STOPS.slice(1, 6)

// Mirrors domain.IsOutcome: these overlay a stage instead of replacing it.
export const OUTCOMES = ["waiting", "needs_input", "failed", "rejected", "ghosted", "withdrawn"]

// Statuses between the stops (agent pipeline steps), offered under "more".
export const SUB_STEPS = ["saved", "preparing", "approved", "submitting", "submitted", "in_progress"]

// The status an application displays: its outcome when set, else its stage.
export function shown(app: Pick<Application, "status" | "outcome">) {
  return app.outcome || app.status
}

// What the badge reads: a tracked posting with no outcome of its own reads
// "Tracked elsewhere" instead of its stage. Transitions still compare shown().
export function displayed(app: Pick<Application, "status" | "outcome" | "tracked_by">) {
  return app.tracked_by && !app.outcome ? "tracked" : shown(app)
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

// What the progress mark draws for an application.
export function markOf(app: Application) {
  return {
    stage: app.stage ?? statusPieces(app.status),
    outcome: app.outcome,
    interviews: app.interviews,
  }
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

export * from "./status"
