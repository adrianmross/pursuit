"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import { useAuth } from "@/lib/auth"
import { SignInForm } from "@/components/SignInForm"

export function SignInGate({
  allowTokenLogin,
  demoAvailable,
}: {
  allowTokenLogin: boolean
  demoAvailable: boolean
}) {
  const { me } = useAuth()
  const router = useRouter()

  useEffect(() => {
    if (me) router.replace("/start")
  }, [me, router])

  if (me) return null
  return <SignInForm allowTokenLogin={allowTokenLogin} demoAvailable={demoAvailable} />
}
