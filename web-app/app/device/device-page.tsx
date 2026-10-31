"use client"

import { Suspense } from "react"
import { useSearchParams } from "next/navigation"
import { DeviceConsent } from "@/components/CliLogin"

function WithParams({ allowTokenLogin }: { allowTokenLogin: boolean }) {
  const code = useSearchParams().get("user_code") || ""
  return <DeviceConsent initialCode={code} allowTokenLogin={allowTokenLogin} />
}

export function DevicePage({ allowTokenLogin }: { allowTokenLogin: boolean }) {
  return (
    <Suspense>
      <WithParams allowTokenLogin={allowTokenLogin} />
    </Suspense>
  )
}
