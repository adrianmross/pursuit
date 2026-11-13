import { EXPORT_MODE, serverApi } from "@/lib/server"
import { SignInGate } from "./sign-in-gate"

export default async function SignInPage() {
  let allowTokenLogin = true
  let demoAvailable = false
  if (!EXPORT_MODE) {
    try {
      const result = await serverApi<{ allow_token_login: boolean; demo_available: boolean }>("/v1/auth/providers")
      allowTokenLogin = result.allow_token_login
      demoAvailable = result.demo_available
    } catch {
      // public endpoint; leave the defaults if it's unreachable
    }
  }
  return <SignInGate allowTokenLogin={allowTokenLogin} demoAvailable={demoAvailable} />
}
