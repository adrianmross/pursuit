import type { Metadata } from "next"
import { Footer, Layout, Navbar } from "nextra-theme-docs"
import { Banner } from "nextra/components"
import { getPageMap } from "nextra/page-map"
import "nextra-theme-docs/style.css"
import "@scalar/api-reference-react/style.css"
import "./globals.css"

export const metadata: Metadata = {
  title: {
    default: "Hire Pursuit docs",
    template: "%s — Hire Pursuit docs",
  },
  description: "API and CLI reference for pursuit, a self-hosted job-application tracker.",
}

const navbar = <Navbar logo={<b>Hire Pursuit docs</b>} projectLink="https://github.com/adrianmross/pursuit" />

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
