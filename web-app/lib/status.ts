// Status words, tones, categories and progress-mark pieces. No app types, so the
// registry ships this file with the progress mark and status badge.

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

// The seven stops on the progress mark, piece 0 (To apply) to 6 (Offer).
export function label(status?: string) {
  return (status && LABELS[status]) || status || "—"
}

// The status chips are told apart by lightness and outline, not
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
