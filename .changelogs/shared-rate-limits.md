---
component: pursuit
bump: patch
type: fix
---

Count rate limits in the database so they hold across serverless instances, add limits to the OAuth token endpoint and MCP calls, and let a refresh token be retried for a few seconds without revoking the connection
