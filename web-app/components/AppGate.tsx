"use client"

import { useEffect } from "react"
import { usePathname, useRouter } from "next/navigation"
import { EXPORT_MODE } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import { Landing } from "@/components/Landing"

// Hosted build: proxy.ts already redirected unauthenticated requests to
// /sign-in server-side before this ever renders — this is a no-op there
// (so "/" there still redirects rather than showing Landing; hosted mode
// isn't deployed anywhere today — see proxy.ts).
//
// Export mode has no server to do that gating (no middleware at runtime in
// a static export), so this is the client-side equivalent: wait for
// AuthProvider's mount-time /v1/me check (see lib/auth.tsx) to settle,
// then either show the public Landing page ("/") or bounce to /sign-in
// (everything else) if it didn't produce an identity.
export function AppGate({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const pathname = usePathname()
  const { me, loading } = useAuth()
  const anonymous = EXPORT_MODE && !loading && me == null

  useEffect(() => {
    if (anonymous && pathname !== "/") router.replace("/sign-in")
  }, [anonymous, pathname, router])

  if (EXPORT_MODE && loading) return null
  if (anonymous) return pathname === "/" ? <Landing /> : null
  return children
}
