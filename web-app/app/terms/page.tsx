import type { Metadata } from "next"
import { LegalPage } from "@/components/LegalPage"
import { legalHtml } from "@/lib/legal-md"

export const metadata: Metadata = { title: "Terms" }

export default function Page() {
  return <LegalPage html={legalHtml("terms")} />
}
