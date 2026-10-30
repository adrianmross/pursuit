import type { Metadata } from "next"
import { Footer, Layout, Navbar } from "nextra-theme-docs"
import { Banner } from "nextra/components"
import { getPageMap } from "nextra/page-map"
import "nextra-theme-docs/style.css"
import "./globals.css"

export const metadata: Metadata = {
  title: {
    default: "Hire Pursuit docs",
    template: "%s — Hire Pursuit docs",
  },
  // Same brand icon as the dashboard; served by the daemon at the root.
  icons: { icon: "/favicon.svg" },
  description: "API and CLI reference for pursuit, a self-hosted job-application tracker.",
}

const navbar = (
  <Navbar logo={<b>Hire Pursuit docs</b>} projectLink="https://github.com/adrianmross/pursuit">
    <a href="/reference" className="x:text-sm x:font-medium">
      API reference
    </a>
    <a href="/" className="x:text-sm x:font-medium">
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
