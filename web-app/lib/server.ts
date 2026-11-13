// The server half of appkit-ui, with the product named first. Server code
// (pages, route handlers, the proxy) imports from here, never from the
// package directly, so the cookie name and the API address are always set.
import { configure } from "@adrianmross/appkit-ui/server"
import { BRAND } from "@/lib/brand"

configure(BRAND)

export * from "@adrianmross/appkit-ui/server"
