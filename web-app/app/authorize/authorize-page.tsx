"use client"

import { Suspense } from "react"
import { useSearchParams } from "next/navigation"
import { AuthorizeConsent } from "@/components/CliLogin"

function WithParams({ allowTokenLogin }: { allowTokenLogin: boolean }) {
  const params = useSearchParams()
  return <AuthorizeConsent params={new URLSearchParams(params.toString())} allowTokenLogin={allowTokenLogin} />
}

export function AuthorizePage({ allowTokenLogin }: { allowTokenLogin: boolean }) {
  return (
    <Suspense>
      <WithParams allowTokenLogin={allowTokenLogin} />
    </Suspense>
  )
}
