import { getPref } from "@/lib/settings"

export type SearchQualifier = { key: string; op: "=" | ">" | ">=" | "<" | "<="; value: string }

export type ParsedSearch = { free: string; qualifiers: SearchQualifier[] }

const OPS = [">=", "<=", ">", "<"] as const

export function parseSearch(input: string): ParsedSearch {
  // With the syntax turned off in Settings, everything is plain text.
  if (getPref("searchSyntax") === "off") return { free: input.trim(), qualifiers: [] }
  const qualifiers: SearchQualifier[] = []
  const free: string[] = []
  for (const token of input.trim().split(/\s+/).filter(Boolean)) {
    const m = token.match(/^([a-zA-Z_]+):(.*)$/)
    if (!m) {
      free.push(token)
      continue
    }
    const [, key, rest] = m
    let op: SearchQualifier["op"] = "="
    let value = rest
    for (const candidate of OPS) {
      if (rest.startsWith(candidate)) {
        op = candidate
        value = rest.slice(candidate.length)
        break
      }
    }
    if (!value) {
      // A known key with nothing after it is still being typed: ignore it
      // rather than searching for the literal text "status:".
      if (!QUALIFIER_KEYS.some(([k]) => k === key.toLowerCase())) free.push(token)
      continue
    }
    qualifiers.push({ key: key.toLowerCase(), op, value })
  }
  return { free: free.join(" "), qualifiers }
}

function compareDates(rowValue: string | undefined, op: SearchQualifier["op"], value: string): boolean {
  if (!rowValue) return false
  if (op === "=") return rowValue.startsWith(value)
  return op === ">" ? rowValue > value : op === ">=" ? rowValue >= value : op === "<" ? rowValue < value : rowValue <= value
}

export function matchesQualifiers(
  row: { status: string; tags?: string[]; applied_date?: string; company?: string; title?: string },
  qualifiers: SearchQualifier[],
): boolean {
  return qualifiers.every((q) => {
    switch (q.key) {
      case "status":
        return row.status.toLowerCase() === q.value.toLowerCase()
      case "tag":
        return (row.tags || []).some((t) => t.toLowerCase() === q.value.toLowerCase())
      case "applied":
        return compareDates(row.applied_date, q.op, q.value)
      case "company":
        return (row.company || "").toLowerCase().includes(q.value.toLowerCase())
      case "title":
        return (row.title || "").toLowerCase().includes(q.value.toLowerCase())
      default:
        return true
    }
  })
}

export const QUALIFIER_KEYS: [string, string][] = [
  ["status", "Stage or outcome, like applied"],
  ["tag", "Applications with a tag"],
  ["company", "Company name contains"],
  ["title", "Role title contains"],
  ["applied", "Applied date, like >2026-01-01"],
]

export type Token = { text: string; kind: "space" | "free" | "key" | "op" | "value" | "unknown" }

// Splits input the way parseSearch reads it, for syntax highlighting. Unknown
// keys still filter nothing, so they are marked rather than hidden.
export function tokenize(input: string): Token[] {
  const syntax = getPref("searchSyntax") !== "off"
  const out: Token[] = []
  for (const part of input.split(/(\s+)/)) {
    if (!part) continue
    if (/^\s+$/.test(part)) {
      out.push({ text: part, kind: "space" })
      continue
    }
    const m = syntax ? part.match(/^([a-zA-Z_]+):(.*)$/) : null
    if (!m || !m[2]) {
      out.push({ text: part, kind: "free" })
      continue
    }
    const known = QUALIFIER_KEYS.some(([k]) => k === m[1].toLowerCase())
    out.push({ text: m[1] + ":", kind: known ? "key" : "unknown" })
    const op = OPS.find((o) => m[2].startsWith(o))
    if (op) out.push({ text: op, kind: "op" })
    out.push({ text: m[2].slice(op?.length || 0), kind: known ? "value" : "unknown" })
  }
  return out
}

// The whitespace-delimited word around the caret.
export function tokenAt(input: string, caret: number) {
  let start = caret
  while (start > 0 && !/\s/.test(input[start - 1])) start--
  let end = caret
  while (end < input.length && !/\s/.test(input[end])) end++
  return { start, end, text: input.slice(start, end) }
}
