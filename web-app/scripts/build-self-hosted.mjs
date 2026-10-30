#!/usr/bin/env node
// `output: "export"` can't contain dynamic Route Handlers (api/auth/*,
// api/proxy/*) — they only exist for the hosted/cookie-BFF build and have
// no meaning for self-hosted (the client talks to the Go API directly, see
// lib/api-client.ts). Move them aside for this build, then restore them
// unconditionally, even on failure.
import { existsSync, renameSync, rmSync } from "node:fs"
import { spawnSync } from "node:child_process"
import { join } from "node:path"

const root = join(import.meta.dirname, "..")
const appApi = join(root, "app", "api")
const parked = join(root, ".app-api.export-build-tmp")

if (existsSync(parked)) renameSync(parked, appApi) // leftover from an interrupted run
renameSync(appApi, parked)
// A previous `next dev`/`next build` leaves route-manifest types for
// app/api/* in .next — stale once those routes are parked, and next build
// then fails type-checking against files that no longer exist.
rmSync(join(root, ".next"), { recursive: true, force: true })

let status = 1
try {
  const result = spawnSync("next", ["build"], {
    stdio: "inherit",
    env: { ...process.env, NEXT_PUBLIC_EXPORT_MODE: "1" },
  })
  status = result.status ?? 1
} finally {
  renameSync(parked, appApi)
}

process.exit(status)
