"use client"

import { DeviceConsent } from "@adrianmross/appkit-ui"

// No query parameters: the code is typed, never taken from the link.
export function DevicePage({ allowTokenLogin }: { allowTokenLogin: boolean }) {
  return <DeviceConsent allowTokenLogin={allowTokenLogin} />
}
