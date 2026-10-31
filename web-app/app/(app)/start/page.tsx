import Link from "next/link"
import { ArrowRight, BookOpen, Bot, Download, Puzzle, Terminal } from "lucide-react"
import { EXPORT_MODE, pursuitApiAuthed } from "@/lib/pursuit-api"
import type { Application, Page } from "@/lib/types"
import { Button } from "@/components/ui/button"
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemFooter,
  ItemGroup,
  ItemMedia,
  ItemTitle,
} from "@/components/ui/item"
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

      <ItemGroup>
        <Item variant="outline">
          <ItemMedia variant="icon">
            <Bot />
          </ItemMedia>
          <ItemContent>
            <ItemTitle>Connect an agent</ItemTitle>
            <ItemDescription>
              Copy one prompt into your coding agent. It installs the CLI and signs in with a code you type here.
            </ItemDescription>
          </ItemContent>
          <ItemActions>
            <Button size="sm" asChild>
              <Link href="/agents">
                Set up <ArrowRight />
              </Link>
            </Button>
          </ItemActions>
        </Item>

        <Item variant="outline">
          <ItemMedia variant="icon">
            <Terminal />
          </ItemMedia>
          <ItemContent>
            <ItemTitle>Prefer the terminal?</ItemTitle>
            <ItemDescription>
              Everything here is also a <code className="font-mono text-foreground">pursuit</code> command.
            </ItemDescription>
          </ItemContent>
          <ItemActions>
            <Button variant="outline" size="sm" asChild>
              <a href="https://hire-pursuit.app/docs" target="_blank" rel="noopener noreferrer">
                <BookOpen /> Read the docs
              </a>
            </Button>
          </ItemActions>
          <ItemFooter className="block">
            <CommandBlock command="pursuit list --stale" />
          </ItemFooter>
        </Item>

        <Item variant="outline">
          <ItemMedia variant="icon">
            <Puzzle />
          </ItemMedia>
          <ItemContent>
            <ItemTitle>Browser extension</ItemTitle>
            <ItemDescription>
              Save the posting you have open and autofill Greenhouse, Ashby, and Lever application forms from this server.
            </ItemDescription>
          </ItemContent>
          <ItemActions>
            <Button variant="outline" size="sm" asChild>
              <a href="https://github.com/adrianmross/pursuit#browser-extension" target="_blank" rel="noopener noreferrer">
                <Download /> Get the extension
              </a>
            </Button>
          </ItemActions>
        </Item>
      </ItemGroup>

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
