import { EXPORT_MODE } from "@/lib/api-client"

// Hosted: a real dynamic route, /app/[id]/page.tsx.
// Export mode: static export can't enumerate every application id via
// generateStaticParams (unbounded), so there's no /app/[id] route at all
// in that build — a static /app/view page reads the id from a query
// string instead, which needs nothing known at build time.
export function applicationHref(id: string): string {
  return EXPORT_MODE ? `/app/view?id=${encodeURIComponent(id)}` : `/app/${encodeURIComponent(id)}`
}
