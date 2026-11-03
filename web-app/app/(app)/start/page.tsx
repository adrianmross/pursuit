import { EXPORT_MODE, pursuitApiAuthed } from "@/lib/pursuit-api"
import type { Application, Page } from "@/lib/types"
import { ItemGroup } from "@/components/ui/item"
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
    const page = await pursuitApiAuthed<Page<Application>>("/v1/applications?limit=1")
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

      <GetStarted />

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
