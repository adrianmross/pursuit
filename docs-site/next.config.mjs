import nextra from "nextra"

const withNextra = nextra({
  defaultShowCopyCode: true,
})

/** @type {import('next').NextConfig} */
const nextConfig = {
  poweredByHeader: false,
  // Mounted at /docs so it can sit behind the same origin as the dashboard and API.
  basePath: "/docs",
  // The dev-mode indicator sits bottom-left, on top of Nextra's own
  // theme-switch/collapse-sidebar buttons there — turn it off.
  devIndicators: false,
}

export default withNextra(nextConfig)
