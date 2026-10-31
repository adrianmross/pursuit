import type { Metadata } from "next"
import { allowTokenLogin } from "@/lib/cli-login"
import { AuthorizePage } from "./authorize-page"

export const metadata: Metadata = { title: "Sign in the CLI" }

// `pursuit auth login` opens this page with the OAuth2 request in the
// query string (see internal/httpapi/oauth.go).
export default async function Page() {
  return <AuthorizePage allowTokenLogin={await allowTokenLogin()} />
}
