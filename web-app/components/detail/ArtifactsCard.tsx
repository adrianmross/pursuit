"use client"

import { useEffect, useState } from "react"
import { ChevronDown, Eye, GitCompareArrows } from "lucide-react"
import { api } from "@adrianmross/appkit-ui"
import { useAuth } from "@adrianmross/appkit-ui"
import { useFeatures } from "@/lib/features"
import type { Artifact, Document, Page } from "@/lib/types"
import { Badge } from "@adrianmross/appkit-ui/ui/badge"
import { SectionCard } from "@adrianmross/appkit-ui"
import { Button } from "@adrianmross/appkit-ui/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger } from "@adrianmross/appkit-ui/ui/dropdown-menu"
import { CompareDialog } from "@/components/detail/CompareDialog"
import { docName } from "@/components/profile/DocumentsPage"
import { useList } from "@/components/profile/ProfilePage"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemMedia, ItemTitle } from "@adrianmross/appkit-ui/ui/item"
import { FileDrop } from "@/components/FileDrop"
import { FileIcon, FilePreview, isTextType } from "@/components/FilePreview"

const LABELS: Record<string, string> = {
  resume_pdf: "Résumé",
  cover_letter: "Cover letter",
  confirmation: "Confirmation",
  answers_json: "Prepared answers",
  form_snapshot: "Form snapshot",
}
const UPLOADS = { resume_pdf: "Tailored résumé", cover_letter: "Cover letter", confirmation: "Confirmation" }
// Files people read first; prepare's JSON output after.
const ORDER = ["resume_pdf", "cover_letter", "confirmation", "answers_json", "form_snapshot"]

// Mirrors service.AddArtifact: these kinds, and any non-text upload, are base64.
const base64 = (kind?: string) => (f: { content_type?: string }) =>
  kind === "resume_pdf" || kind === "confirmation" || !isTextType(f.content_type)

// First lines of a text cover letter, so the card shows what was written
// without opening it.
function Snippet({ a }: { a: Artifact }) {
  const [text, setText] = useState("")
  const textual = a.kind === "cover_letter" && isTextType(a.content_type)
  useEffect(() => {
    if (!textual) return
    void api<{ body: string }>("/v1/artifacts/" + a.id).then((f) => setText(f.body.slice(0, 240)), () => {})
  }, [a.id, textual])
  return text ? <ItemDescription className="line-clamp-2 italic">{text}</ItemDescription> : null
}

const IN_USE = [
  ["resume_pdf", "base_resume", "Résumé"],
  ["cover_letter", "cover_letter", "Cover letter"],
] as const

// What this application sends for one kind: the newest file of it, where it
// came from, a menu to switch to another of your versions, and a diff
// against your main one.
function InUse({ appId, kind, docKind, label, artifacts, docs, tailored, onChanged }: {
  appId: string
  kind: "resume_pdf" | "cover_letter"
  docKind: string
  label: string
  artifacts: Artifact[]
  docs: Document[]
  tailored: boolean
  onChanged: () => void
}) {
  const { can } = useAuth()
  const [comparing, setComparing] = useState(false)
  const [error, setError] = useState("")
  const current = artifacts.filter((a) => a.kind === kind).at(-1)
  const versions = docs.filter((d) => d.kind === docKind)
  const main = versions.find((d) => d.primary)
  const from = versions.find((d) => d.id === current?.source_document_id)
  if (!current && !versions.length) return null
  const source = !current ? "Nothing attached yet" : from ? (from.primary ? "Your main " + label.toLowerCase() : "Version: " + docName(from)) : "Tailored for this application"

  async function use(d: Document) {
    setError("")
    try {
      await api(`/v1/applications/${encodeURIComponent(appId)}/documents`, { method: "POST", body: { document_id: d.id } })
      onChanged()
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not switch")
    }
  }

  return (
    <Item variant="outline" size="sm" className="flex-wrap">
      <ItemMedia variant="icon">
        <FileIcon type={current?.content_type} name={current?.filename} />
      </ItemMedia>
      <ItemContent>
        <ItemTitle>
          {label}
          {current && <Badge variant="secondary">In use</Badge>}
        </ItemTitle>
        <ItemDescription>
          {current?.filename ? current.filename + " · " : ""}
          {source}
        </ItemDescription>
      </ItemContent>
      <ItemActions>
        {tailored && current && main && from?.id !== main.id && (
          <Button size="sm" variant="ghost" onClick={() => setComparing(true)}>
            <GitCompareArrows /> Compare
          </Button>
        )}
        {can("write") && versions.length > 0 && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button size="sm" variant="outline">
                {current ? "Change" : "Attach"} <ChevronDown />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuLabel>Your {label.toLowerCase()}s</DropdownMenuLabel>
              {versions.map((d) => (
                <DropdownMenuItem key={d.id} disabled={d.id === current?.source_document_id || (!d.primary && !tailored)} onClick={() => void use(d)}>
                  {docName(d)}
                  {d.primary && <Badge variant="secondary">Main</Badge>}
                </DropdownMenuItem>
              ))}
              {!tailored && versions.length > 1 && <DropdownMenuLabel className="font-normal text-muted-foreground">Other versions need Tailored documents (Settings › Experimental)</DropdownMenuLabel>}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </ItemActions>
      {error && <p className="basis-full text-xs text-destructive">{error}</p>}
      {comparing && <CompareDialog appId={appId} kind={kind} open onOpenChange={setComparing} />}
    </Item>
  )
}

export function ArtifactsCard({ appId, artifacts: initial }: { appId: string; artifacts: Artifact[] }) {
  const { can } = useAuth()
  const [artifacts, setArtifacts] = useState(initial)
  const path = `/v1/applications/${encodeURIComponent(appId)}/artifacts`
  const reload = () => void api<Page<Artifact>>(path).then((p) => setArtifacts(p.data || []))
  const sorted = [...artifacts].sort((a, b) => ORDER.indexOf(a.kind || "") - ORDER.indexOf(b.kind || ""))
  const { rows: docs } = useList<Document>("/v1/documents")
  const { features } = useFeatures()
  const tailored = !!features?.find((f) => f.key === "tailored_documents")?.available

  return (
    <SectionCard
      id="documents"
      title="Documents"
      summary={sorted.length ? `${sorted.length} file${sorted.length === 1 ? "" : "s"}` : undefined}
      description="The résumé and cover letter sent with this application, plus what prepare produced."
    >
      <ItemGroup className="gap-2">
        {IN_USE.map(([kind, docKind, label]) => (
          <InUse key={kind} appId={appId} kind={kind} docKind={docKind} label={label} artifacts={artifacts} docs={docs ?? []} tailored={tailored} onChanged={reload} />
        ))}
      </ItemGroup>
      {sorted.length > 0 && (
        <ItemGroup className="gap-2">
          {sorted.map((a) => (
            <FilePreview key={a.id} path={"/v1/artifacts/" + a.id} base64={base64(a.kind)} title={a.filename || LABELS[a.kind || ""] || "File"} description={LABELS[a.kind || ""]}>
              <Item variant="outline" size="sm" asChild>
                <button type="button" className="text-left hover:bg-muted/50">
                  <ItemMedia variant="icon">
                    <FileIcon type={a.content_type} name={a.filename} />
                  </ItemMedia>
                  <ItemContent>
                    <ItemTitle>
                      {a.filename || a.kind}
                      <Badge variant="secondary">{LABELS[a.kind || ""] || a.kind}</Badge>
                    </ItemTitle>
                    {a.created_at && <ItemDescription>{new Date(a.created_at).toLocaleString()}</ItemDescription>}
                    <Snippet a={a} />
                  </ItemContent>
                  <ItemActions>
                    <Eye className="size-4 text-muted-foreground" aria-hidden />
                  </ItemActions>
                </button>
              </Item>
            </FilePreview>
          ))}
        </ItemGroup>
      )}
      {can("write") && (
        <FileDrop
          path={path}
          kinds={UPLOADS}
          compact
          description={sorted.length ? "Attach a tailored résumé or cover letter to this application." : "Nothing attached yet. Prepare adds your base résumé, or attach a tailored one."}
          onUploaded={reload}
        />
      )}
    </SectionCard>
  )
}
