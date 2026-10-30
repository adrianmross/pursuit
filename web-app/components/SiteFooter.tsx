import Link from "next/link"
import { Heart } from "lucide-react"

// The footer every surface shares. docs-site/app/layout.tsx and
// internal/spec/reference.html repeat this markup; keep the three in step.
// One row from md up (brand · credit · links); stacked and centred below.
export function SiteFooter() {
  return (
    <footer className="border-t text-sm text-muted-foreground">
      <div className="mx-auto flex max-w-6xl flex-col items-center gap-4 px-5 py-8 md:grid md:grid-cols-[1fr_auto_1fr] md:gap-6">
        <Link href="/" className="flex items-center gap-2 text-foreground md:justify-self-start">
          <img src="/brand/hire-pursuit-mark.svg" alt="" className="h-5 w-auto dark:hidden" />
          <img src="/brand/hire-pursuit-mark-reverse.svg" alt="" className="hidden h-5 w-auto dark:block" />
          Hire Pursuit
        </Link>
        <p className="order-last flex flex-wrap items-center justify-center gap-x-1.5 gap-y-1 text-center text-xs md:order-none">
          <span className="inline-flex items-center gap-1.5">
            Built with
            <Heart
              aria-label="love"
              className="size-3.5 fill-current transition-colors hover:text-red-500 motion-safe:hover:[animation:heartbeat_1.4s_ease-in-out_infinite]"
            />
            by Adrian Ross
          </span>
          <span aria-hidden className="hidden sm:inline">
            ·
          </span>
          <a
            href="https://github.com/adrianmross/pursuit/blob/main/LICENSE"
            className="basis-full hover:text-foreground sm:basis-auto"
          >
            MIT License © 2026
          </a>
        </p>
        <nav className="flex gap-5 md:justify-self-end">
          <a href="/docs/" className="hover:text-foreground">
            Docs
          </a>
          <a href="/reference" className="hover:text-foreground">
            Reference
          </a>
          <a href="https://github.com/adrianmross/pursuit" className="hover:text-foreground">
            GitHub
          </a>
        </nav>
      </div>
    </footer>
  )
}
