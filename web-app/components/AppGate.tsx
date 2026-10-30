"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import { EXPORT_MODE } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"

// Hosted build: proxy.ts already redirected unauthenticated requests to
// /sign-in server-side before this ever renders — this is a no-op there.
//
// Export mode has no server to do that gating (no middleware at runtime in
// a static export), so this is the client-side equivalent: wait for
// AuthProvider's mount-time /v1/me check (see lib/auth.tsx) to settle,
// then bounce to /sign-in if it didn't produce an identity.
export function AppGate({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const { me, loading } = useAuth()

  useEffect(() => {
    if (EXPORT_MODE && !loading && me == null) router.replace("/sign-in")
  }, [loading, me, router])

  if (EXPORT_MODE && (loading || me == null)) return null
  return children
}
