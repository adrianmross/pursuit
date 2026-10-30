import { NextResponse, type NextRequest } from "next/server"
import { TOKEN_COOKIE } from "@/lib/pursuit-api"

// Only runs on the hosted (server) build — static export has no proxy
// (formerly "middleware") at runtime, self-hosted auth is gated
// client-side instead (see lib/api-client.ts / AuthProvider).
export default function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl
  const isPublic =
    pathname === "/" ||
    pathname === "/sign-in" ||
    pathname.startsWith("/api/auth/") ||
    pathname.startsWith("/_next/") ||
    pathname.startsWith("/brand/") ||
    pathname === "/favicon.ico"
  if (isPublic) return NextResponse.next()

  const hasToken = request.cookies.has(TOKEN_COOKIE)
  if (!hasToken) {
    const url = request.nextUrl.clone()
    url.pathname = "/sign-in"
    return NextResponse.redirect(url)
  }
  return NextResponse.next()
}

export const config = {
  matcher: ["/((?!api/proxy).*)"],
}
