import Link from "next/link"
import { PRODUCT_NAME } from "@/lib/brand"
import { SiteFooter } from "@/components/SiteFooter"

// Privacy and Terms: public pages at the site root, outside the signed-in app.
export function LegalPage({ html }: { html: string }) {
  return (
    <div className="flex min-h-svh flex-col">
      <header className="mx-auto w-full max-w-3xl px-5 py-6">
        <Link href="/" aria-label={PRODUCT_NAME} className="inline-flex">
          <img src="/brand/wordmark.svg" alt="" className="h-6 w-auto dark:hidden" />
          <img src="/brand/wordmark-reverse.svg" alt="" className="hidden h-6 w-auto dark:block" />
        </Link>
      </header>
      <main className="mx-auto w-full max-w-3xl flex-1 px-5 pb-16" dangerouslySetInnerHTML={{ __html: html }} />
      <SiteFooter />
    </div>
  )
}
