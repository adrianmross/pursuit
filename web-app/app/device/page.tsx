import type { Metadata } from "next"
import { allowTokenLogin } from "@/lib/cli-login"
import { DevicePage } from "./device-page"

export const metadata: Metadata = { title: "Connect a device" }

// verification_uri for `pursuit auth login --device`.
export default async function Page() {
  return <DevicePage allowTokenLogin={await allowTokenLogin()} />
}
