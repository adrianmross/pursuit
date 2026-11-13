import {
  compareText,
  matchesQualifiers as matches,
  parseSearch as parse,
  type Qualifier,
  type SearchQualifier,
} from "@adrianmross/appkit-ui"
import { FILTERS, MORE } from "@/lib/types"

// The search box's qualifiers: what `status:applied tag:remote` means for an
// application, and the values offered after each colon.
type Row = { status: string; tags?: string[]; applied_date?: string; company?: string; title?: string }

const STATUSES = FILTERS.concat(MORE).filter(([v]) => v)
const ago = (days: number) => new Date(Date.now() - days * 864e5).toISOString().slice(0, 10)
const uniq = (xs: (string | undefined)[]) => [...new Set(xs.filter((x): x is string => !!x && !/\s/.test(x)))]
const having = (xs: string[], partial: string) => xs.filter((x) => x.toLowerCase().includes(partial.toLowerCase())).map((value) => ({ value }))

export const QUALIFIERS: Qualifier<Row>[] = [
  {
    key: "status",
    hint: "Stage or outcome, like applied",
    match: (row, q) => row.status.toLowerCase() === q.value.toLowerCase(),
    values: (partial) => {
      const p = partial.toLowerCase()
      return STATUSES.filter(([v, l]) => v.includes(p) || l.toLowerCase().includes(p)).map(([value, hint]) => ({ value, hint }))
    },
  },
  {
    key: "tag",
    hint: "Applications with a tag",
    match: (row, q) => (row.tags || []).some((t) => t.toLowerCase() === q.value.toLowerCase()),
    values: (partial, rows) => having([...new Set(rows.flatMap((r) => r.tags || []))], partial),
  },
  {
    key: "company",
    hint: "Company name contains",
    match: (row, q) => (row.company || "").toLowerCase().includes(q.value.toLowerCase()),
    values: (partial, rows) => having(uniq(rows.map((r) => r.company)), partial),
  },
  {
    key: "title",
    hint: "Role title contains",
    match: (row, q) => (row.title || "").toLowerCase().includes(q.value.toLowerCase()),
    values: (partial, rows) => having(uniq(rows.flatMap((r) => (r.title || "").split(/\s+/))), partial),
  },
  {
    key: "applied",
    hint: "Applied date, like >2026-01-01",
    match: (row, q) => compareText(row.applied_date, q.op, q.value),
    values: (partial) =>
      [
        { value: ">" + ago(7), hint: "after 7 days ago" },
        { value: ">" + ago(30), hint: "after 30 days ago" },
        { value: ago(0).slice(0, 7), hint: "this month" },
        { value: ago(0).slice(0, 4), hint: "this year" },
      ].filter((v) => v.value.includes(partial.toLowerCase())),
  },
]

const KEYS = QUALIFIERS.map((q) => q.key)

export const parseSearch = (input: string) => parse(input, KEYS)
export const matchesQualifiers = (row: Row, qualifiers: SearchQualifier[]) => matches(row, qualifiers, QUALIFIERS)
