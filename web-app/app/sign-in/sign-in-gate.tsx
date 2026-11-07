"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import { useAuth } from "@/lib/auth"
import { SignInForm } from "@/components/SignInForm"

export function SignInGate({
  allowTokenLogin,
  demoAvailable,
  initialMode,
}: {
  allowTokenLogin: boolean
  demoAvailable: boolean
  initialMode?: "sign-in" | "sign-up"
}) {
  const { me } = useAuth()
  const router = useRouter()

  useEffect(() => {
    if (!me) return
    // ?next= brings a visitor back to where the sign-in started (a full load,
    // so it also reaches pages the Go server renders outside Next).
    // Parsed against this origin so "//host" or "/\thost" can't leave the site.
    const next = new URLSearchParams(window.location.search).get("next")
    const target = next ? URL.parse(next, window.location.origin) : null
    if (target && target.origin === window.location.origin) window.location.replace(target.pathname + target.search + target.hash)
    else router.replace("/start")
  }, [me, router])

  if (me) return null
  return <SignInForm allowTokenLogin={allowTokenLogin} demoAvailable={demoAvailable} initialMode={initialMode} />
}
