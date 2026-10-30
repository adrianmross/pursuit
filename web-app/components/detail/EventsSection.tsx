import { pursuitApi } from "@/lib/pursuit-api"
import type { Event, Page } from "@/lib/types"
import { EventsHistoryCard } from "@/components/detail/EventsHistoryCard"
import { SectionError } from "@/components/detail/SectionError"

// A 401 here deliberately doesn't redirect like pursuitApiAuthed would —
// the core page already rendered (application was awaited and worked), so
// a token that expires mid-request should surface as "this one section
// couldn't load, retry," not yank the user off a page they're looking at.
export async function EventsSection({ id }: { id: string }) {
  let page: Page<Event> | null = null
  try {
    page = await pursuitApi<Page<Event>>(`/v1/applications/${encodeURIComponent(id)}/events?limit=100`)
  } catch {
    // fall through — page stays null
  }
  // JSX construction stays outside the try/catch: it can't throw from the
  // await above, and keeping it separate means a real render error in
  // EventsHistoryCard itself isn't silently swallowed as "couldn't load".
  if (page == null) return <SectionError label="history" />
  return <EventsHistoryCard events={page.data || []} />
}
