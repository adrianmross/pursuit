#!/usr/bin/env node
// `output: "export"` can't contain: dynamic Route Handlers (api/auth/*,
// api/proxy/* — the cookie-BFF only exists for the hosted build, see
// lib/api-client.ts) or a dynamic page segment with no generateStaticParams
// (app/(app)/app/[id] — application ids are unbounded, can't be enumerated
// at build time; export mode uses the static /app/view?id=... route
// instead, see lib/links.ts). Move them all aside for this build, then
// restore them unconditionally, even on failure.
import { existsSync, renameSync, rmSync } from "node:fs"
import { spawnSync } from "node:child_process"
import { join } from "node:path"

const root = join(import.meta.dirname, "..")
const PARK = [
  { real: join(root, "app", "api"), tmp: join(root, ".app-api.export-build-tmp") },
  { real: join(root, "app", "(app)", "app", "[id]"), tmp: join(root, ".app-id.export-build-tmp") },
]

for (const { real, tmp } of PARK) {
  if (existsSync(tmp)) renameSync(tmp, real) // leftover from an interrupted run
  renameSync(real, tmp)
}
// A previous `next dev`/`next build` leaves route-manifest types for the
// parked routes in .next — stale once they're moved, and next build then
// fails type-checking against files that no longer exist.
rmSync(join(root, ".next"), { recursive: true, force: true })

let status = 1
try {
  const result = spawnSync("next", ["build"], {
    stdio: "inherit",
    env: { ...process.env, NEXT_PUBLIC_EXPORT_MODE: "1" },
  })
  status = result.status ?? 1
} finally {
  for (const { real, tmp } of PARK) renameSync(tmp, real)
}

process.exit(status)
