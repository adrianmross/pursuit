import { version } from "@/package.json"

// The product name wherever the dashboard shows it. Renaming? See "Renaming the product" in docs/brand/README.md.
export const PRODUCT_NAME = "Hire Pursuit"

// What appkit-ui needs to know about this product. Plain data, so the proxy
// and the route handlers can read it without loading any component. slug
// decides the localStorage keys (pursuit.token) and the token cookie
// (pursuit_token), so changing it signs every browser out.
export const BRAND = {
  name: PRODUCT_NAME,
  shortName: "Pursuit",
  slug: "pursuit",
  cli: "pursuit",
  envPrefix: "PURSUIT",
  version,
  // Self-hosted serves the dashboard and the API from the same pursuit
  // binary/port — same origin, so a relative path is correct by default.
  // Only set NEXT_PUBLIC_PURSUIT_API_URL if the dashboard is genuinely
  // deployed separately from the API it talks to.
  apiUrl: process.env.NEXT_PUBLIC_PURSUIT_API_URL || "",
  serverApiUrl: process.env.PURSUIT_API_URL || "http://127.0.0.1:8787",
  scopes: {
    read: "See your applications, companies, and settings.",
    write: "Create and change applications. This cannot approve anything.",
  },
  signIn: { title: "Your move.", subtitle: "Sign in to pick up your pipeline, from the CLI or from here." },
}
