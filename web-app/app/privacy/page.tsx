import type { Metadata } from "next"
import { LegalPage } from "@/components/LegalPage"
import { legalHtml } from "@/lib/legal-md"

export const metadata: Metadata = { title: "Privacy" }

export default function Page() {
  return <LegalPage html={legalHtml("privacy")} />
}
