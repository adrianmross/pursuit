"use client"

import { useEffect, useSyncExternalStore } from "react"
import { useRouter } from "next/navigation"
import { useAuth } from "@adrianmross/appkit-ui"
import { SignInForm } from "@/components/SignInForm"

const noop = () => () => {}
const modeFromUrl = () => (new URLSearchParams(window.location.search).get("mode") === "sign-up" ? ("sign-up" as const) : ("sign-in" as const))

export function SignInGate({
  allowTokenLogin,
  demoAvailable,
}: {
  allowTokenLogin: boolean
  demoAvailable: boolean
}) {
  // ?mode=sign-up opens the create-account form. Read in the browser: this page is statically exported,
  // so it can't read search params on the server.
  const initialMode = useSyncExternalStore(noop, modeFromUrl, () => "sign-in" as const)
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
