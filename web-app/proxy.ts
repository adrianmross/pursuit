import { authProxy } from "@/lib/server"

// Only runs on the hosted (server) build — static export has no proxy
// (formerly "middleware") at runtime, self-hosted auth is gated
// client-side instead (see AuthProvider). These open without signing in,
// besides the sign-in and consent pages appkit-ui always leaves open.
export default authProxy(["/", "/privacy", "/terms", "/docs", "/docs/", "/practice.html"])

export const config = {
  matcher: ["/((?!api/proxy).*)"],
}
