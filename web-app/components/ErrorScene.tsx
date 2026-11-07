import { SiteFooter } from "@/components/SiteFooter"

// Full-page layout shared by the 404 and crash pages: graphic, heading,
// copy, actions, footer.
export function ErrorScene({
  graphic,
  title,
  children,
  actions,
}: {
  graphic: React.ReactNode
  title: string
  children: React.ReactNode
  actions: React.ReactNode
}) {
  return (
    <div className="flex min-h-svh flex-col bg-background text-foreground">
      <main className="flex flex-1 flex-col items-center justify-center gap-4 px-4 py-16 text-center">
        {graphic}
        <h1 className="mt-4 font-heading text-4xl font-semibold tracking-tight text-balance sm:text-5xl">{title}</h1>
        <p className="max-w-md text-balance text-muted-foreground">{children}</p>
        <div className="mt-4 flex flex-wrap justify-center gap-3">{actions}</div>
      </main>
      <SiteFooter />
    </div>
  )
}
