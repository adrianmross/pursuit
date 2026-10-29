import { createAuthClient } from "better-auth/client"
import { emailOTPClient, jwtClient } from "better-auth/client/plugins"

// Only defined when the app is deployed with Neon Auth configured.
const baseURL = import.meta.env.VITE_NEON_AUTH_URL as string | undefined

export const authClient = baseURL ? createAuthClient({ baseURL, plugins: [jwtClient(), emailOTPClient()] }) : null
