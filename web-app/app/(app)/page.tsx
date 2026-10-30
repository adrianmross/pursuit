import { Download, BookOpen } from "lucide-react"
import { EXPORT_MODE, pursuitApi } from "@/lib/pursuit-api"
import type { Application, Page } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { CommandBlock } from "@/components/CommandBlock"
import { HomeCta } from "@/components/HomeCta"
import { HomeCtaClient } from "@/components/HomeCtaClient"
import { ImportCard } from "@/components/ImportCard"
import { IMPORTERS } from "@/lib/importers"

export default async function Home() {
  // Hosted: resolved server-side, no client fetch/flash. Export mode: no
  // server at request time, so HomeCtaClient fetches it after mount.
  let cta = <HomeCtaClient />
  if (!EXPORT_MODE) {
    const page = await pursuitApi<Page<Application>>("/v1/applications?limit=1")
    cta = <HomeCta hasApplications={page.page.total > 0} />
  }

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-8 py-8">
      <div className="flex flex-col items-center gap-3 text-center">
        <h1 className="font-display text-xl font-semibold">For your higher pursuits.</h1>
        <p className="max-w-md text-muted-foreground">Land the role.</p>
        {cta}
      </div>

      <Card size="sm">
        <CardHeader>
          <CardTitle>Prefer the terminal?</CardTitle>
          <CardDescription>
            Everything here is also a <code className="font-mono text-foreground">pursuit</code> command.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <CommandBlock command="pursuit list --stale" />
          <Button variant="outline" size="sm" asChild className="self-start">
            <a href="https://hire-pursuit.app/docs" target="_blank" rel="noopener noreferrer">
              <BookOpen className="size-3.5" /> Read the docs
            </a>
          </Button>
        </CardContent>
      </Card>

      <Card size="sm">
        <CardHeader>
          <CardTitle>Browser extension</CardTitle>
          <CardDescription>
            Save the posting you have open and autofill Greenhouse, Ashby, and Lever application forms from this server.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="outline" size="sm" asChild>
            <a href="https://github.com/adrianmross/pursuit#browser-extension" target="_blank" rel="noopener noreferrer">
              <Download className="size-3.5" /> Get the extension
            </a>
          </Button>
        </CardContent>
      </Card>

      <div className="flex flex-col gap-3">
        <h2 className="text-sm font-medium text-muted-foreground">Import existing data</h2>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          {IMPORTERS.map((importer) => (
            <ImportCard key={importer.key} importer={importer} />
          ))}
        </div>
      </div>
    </div>
  )
}
