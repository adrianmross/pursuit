"use client"

import { DeviceConsent } from "@/components/CliLogin"

// No query parameters: the code is typed, never taken from the link.
export function DevicePage({ allowTokenLogin }: { allowTokenLogin: boolean }) {
  return <DeviceConsent allowTokenLogin={allowTokenLogin} />
}
