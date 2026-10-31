import { Landing } from "@/components/Landing"

// Public for everyone, signed in or not (proxy.ts lets "/" through). The
// signed-in home is /start.
export default function Home() {
  return <Landing />
}
