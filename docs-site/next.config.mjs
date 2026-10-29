import nextra from "nextra"

const withNextra = nextra({
  defaultShowCopyCode: true,
})

/** @type {import('next').NextConfig} */
const nextConfig = {
  poweredByHeader: false,
}

export default withNextra(nextConfig)
