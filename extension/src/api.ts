// The Pursuit HTTP API, shared by the background worker and the popup.

export type Pairing = { server: string; token: string };
export type ApiRequest = { method: string; path: string; body?: unknown };

export type App = {
  id: string;
  status: string;
  outcome?: string;
  stage?: number;
  version: number;
  title: string;
  company: string;
  posting_url?: string;
};

export async function api(p: Pairing, method: string, path: string, body?: unknown): Promise<unknown> {
  if (!p.server || !p.token) throw Object.assign(new Error("Pair this browser with your Pursuit server first."), { code: "unpaired" });
  if (!path.startsWith("/v1/")) throw new Error("not an API path");
  const res = await fetch(p.server.replace(/\/$/, "") + path, {
    method,
    headers: {
      Authorization: "Bearer " + p.token,
      ...(body ? { "Content-Type": "application/json" } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let data: { error?: { message?: string; code?: string } } | null = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    // A proxy or sign-in page answered instead of the API.
  }
  if (!res.ok) {
    throw Object.assign(new Error(data?.error?.message || res.statusText || "Request failed"), {
      code: data?.error?.code || String(res.status),
    });
  }
  return data;
}

// Status labels match the dashboard (web-app/lib/types.ts).
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
  final: "Final round",
  waiting: "Waiting",
  offer: "Offer",
  rejected: "Rejected",
  withdrawn: "Withdrawn",
  ghosted: "Ghosted",
};

// What a person sets by hand. Submitting and submitted belong to the queue.
export const MANUAL_STATUSES = [
  "to_apply",
  "applied",
  "screening",
  "interviewing",
  "final",
  "offer",
  "waiting",
  "rejected",
  "withdrawn",
  "ghosted",
];

const PIECES: Record<string, number> = {
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
};

export function stageOf(app: App): number {
  return app.stage ?? PIECES[app.status] ?? 0;
}

export function outcomeKind(status?: string): "hold" | "stop" | "" {
  if (status === "waiting" || status === "needs_input") return "hold";
  if (status === "failed" || status === "rejected" || status === "withdrawn" || status === "ghosted") return "stop";
  return "";
}

// Chips are told apart by lightness and outline; gold is only the offer.
export function tone(status: string): string {
  if (status === "offer") return "win";
  if (status === "approved") return "strong";
  if (status === "needs_input" || status === "awaiting_review" || status === "waiting") return "attention";
  return outcomeKind(status) === "stop" ? "danger" : "";
}

// The progress mark from the design system: five ring pieces and the gold tail.
const RING = [
  "M34.73 64.55 A20.5 20.5 0 0 1 17.34 64.58",
  "M46.36 48.36 A20.5 20.5 0 0 1 37.23 63.15",
  "M38.73 29.93 A20.5 20.5 0 0 1 46.49 45.50",
  "M19.06 26.71 A20.5 20.5 0 0 1 36.37 28.32",
  "M5.95 41.74 A20.5 20.5 0 0 1 16.44 27.87",
  "M5.5 46 V84",
];

// ponytail: whole pieces only; the dashboard's per-round sub-arcs are left out.
export function progressMark(stage: number, outcome = "", size = 28): string {
  const n = Math.max(0, Math.min(6, stage));
  const kind = outcomeKind(outcome);
  const target = kind ? (n === 0 ? 1 : n) : 0;
  const paths = RING.map((d, i) => {
    const piece = i + 1;
    let color = "var(--line)";
    if (piece === target) color = kind === "stop" ? "var(--stop)" : "var(--hold)";
    else if (n >= piece) color = piece === 6 ? "var(--brand-gold)" : "var(--ink)";
    return `<path d="${d}" fill="none" stroke-width="11" stroke="${color}"/>`;
  }).join("");
  const label = `${n} of 6 pieces${outcome ? ", " + (LABELS[outcome] || outcome.replace(/[^a-z_]/g, "")) : ""}`;
  return `<svg viewBox="-11 9 74 80" width="${Math.round((size * 74) / 80)}" height="${size}" role="img" aria-label="${label}">${paths}</svg>`;
}

// resume_pdf artifacts are always base64; a document is base64 unless its type is text.
export function decodeFile(body: string, filename: string, contentType: string, base64: boolean): File {
  const bytes = base64 ? Uint8Array.from(atob(body), (c) => c.charCodeAt(0)) : new TextEncoder().encode(body);
  return new File([bytes], filename || "resume.pdf", { type: contentType || "application/pdf" });
}
