export type SearchQualifier = { key: string; op: "=" | ">" | ">=" | "<" | "<="; value: string }

export type ParsedSearch = { free: string; qualifiers: SearchQualifier[] }

const OPS = [">=", "<=", ">", "<"] as const

export function parseSearch(input: string): ParsedSearch {
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
      free.push(token)
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
