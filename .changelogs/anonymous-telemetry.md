---
component: pursuit
bump: minor
type: feat
---

Send an anonymous install ping from pursuit serve (version, OS, arch, database, container; opt out with PURSUIT_TELEMETRY=off or DO_NOT_TRACK=1) that also reports newer releases, and report hosted server errors to Sentry when PURSUIT_SENTRY_DSN is set
