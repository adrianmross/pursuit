import { ArrowRight, MousePointerClick } from "lucide-react"
import { EXPORT_MODE, serverApiAuthed } from "@/lib/server"
import type { Application, Page } from "@/lib/types"
import { Button } from "@adrianmross/appkit-ui/ui/button"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemMedia, ItemTitle } from "@adrianmross/appkit-ui/ui/item"
import { GetStarted } from "@/components/GetStarted"
import { HomeCta } from "@/components/HomeCta"
import { HomeCtaClient } from "@/components/HomeCtaClient"
import { ImportCard } from "@/components/ImportCard"
import { StartCalendar } from "@/components/StartCalendar"
import { IMPORTERS } from "@/lib/importers"

export default async function Home() {
  // Hosted: resolved server-side, no client fetch/flash. Export mode: no
  // server at request time, so HomeCtaClient fetches it after mount.
  let cta = <HomeCtaClient />
  if (!EXPORT_MODE) {
    const page = await serverApiAuthed<Page<Application>>("/v1/applications?limit=1")
    cta = <HomeCta hasApplications={page.page.total > 0} />
  }

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-8 py-8">
      <div className="flex flex-col items-center gap-3 text-center">
        <h1 className="font-display text-xl font-semibold">For your higher pursuits.</h1>
        <p className="max-w-md text-muted-foreground">Land the role.</p>
        {cta}
      </div>

      <StartCalendar />

      <div data-tour="get-started">
        <GetStarted />
      </div>

      <Item variant="outline">
        <ItemMedia variant="icon">
          <MousePointerClick />
        </ItemMedia>
        <ItemContent>
          <ItemTitle>Try it on a practice application</ItemTitle>
          <ItemDescription>
            With the extension paired, open the example job, press Alt+Shift+P, save it, then choose Autofill this form.
            Acme Robotics is made up and nothing is sent.
          </ItemDescription>
        </ItemContent>
        <ItemActions>
          <Button variant="outline" size="sm" asChild>
            <a href="/practice.html" target="_blank" rel="noopener noreferrer">
              Open the example <ArrowRight />
            </a>
          </Button>
        </ItemActions>
      </Item>

      <div className="flex flex-col gap-3">
        <h2 className="text-sm font-medium text-muted-foreground">Import existing data</h2>
        <ItemGroup className="gap-2">
          {IMPORTERS.map((importer) => (
            <ImportCard key={importer.key} importer={importer} />
          ))}
        </ItemGroup>
      </div>
    </div>
  )
}
