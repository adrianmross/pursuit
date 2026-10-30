import { pursuitApi } from "@/lib/pursuit-api"
import type { Artifact, Page } from "@/lib/types"
import { ArtifactsCard } from "@/components/detail/ArtifactsCard"
import { SectionError } from "@/components/detail/SectionError"

export async function ArtifactsSection({ id }: { id: string }) {
  let page: Page<Artifact> | null = null
  try {
    page = await pursuitApi<Page<Artifact>>(`/v1/applications/${encodeURIComponent(id)}/artifacts`)
  } catch {
    // fall through — page stays null
  }
  if (page == null) return <SectionError label="artifacts" />
  return <ArtifactsCard artifacts={page.data || []} />
}
