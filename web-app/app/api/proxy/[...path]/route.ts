import { proxyRoute } from "@/lib/server"

// Every client component talks to the Go API through here; the browser
// never sees the token.
export const GET = proxyRoute.GET
export const POST = proxyRoute.POST
export const PATCH = proxyRoute.PATCH
export const DELETE = proxyRoute.DELETE
