import type { Metadata } from "next"
import { Footer, Layout, Navbar } from "nextra-theme-docs"
import { Banner } from "nextra/components"
import { getPageMap } from "nextra/page-map"
import "nextra-theme-docs/style.css"
import "./globals.css"

// Dashboard and API live on the origin that serves /docs, so these are
// root-relative. Set NEXT_PUBLIC_SITE_URL (e.g. https://hire-pursuit.app)
// only if the docs are deployed on a different origin.
const site = process.env.NEXT_PUBLIC_SITE_URL ?? ""

export const metadata: Metadata = {
  title: {
    default: "Hire Pursuit docs",
    template: "%s — Hire Pursuit docs",
  },
  description: "API and CLI reference for pursuit, a self-hosted job-application tracker.",
}

const navbar = (
  <Navbar logo={<b>Hire Pursuit docs</b>} projectLink="https://github.com/adrianmross/pursuit">
    <a href={`${site}/reference`} className="x:text-sm x:font-medium">
      API reference
    </a>
    <a href={`${site}/`} className="x:text-sm x:font-medium">
      Dashboard
    </a>
  </Navbar>
)

const banner = (
  <Banner storageKey="pursuit-docs-pre">
    Pre-release — pursuit has no versioned releases yet. Everything here can change.
  </Banner>
)

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const pageMap = await getPageMap()
  return (
    <html lang="en" dir="ltr" suppressHydrationWarning>
      <body>
        <Layout
          banner={banner}
          navbar={navbar}
          footer={<Footer>MIT {new Date().getFullYear()} © Adrian Ross.</Footer>}
          pageMap={pageMap}
          docsRepositoryBase="https://github.com/adrianmross/pursuit/tree/main/docs-site"
        >
          {children}
        </Layout>
      </body>
    </html>
  )
}
