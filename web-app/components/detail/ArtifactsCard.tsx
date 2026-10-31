"use client"

import { useEffect, useState } from "react"
import { Eye } from "lucide-react"
import { api } from "@/lib/api-client"
import { useAuth } from "@/lib/auth"
import type { Artifact, Page } from "@/lib/types"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemMedia, ItemTitle } from "@/components/ui/item"
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

export function ArtifactsCard({ appId, artifacts: initial }: { appId: string; artifacts: Artifact[] }) {
  const { can } = useAuth()
  const [artifacts, setArtifacts] = useState(initial)
  const path = `/v1/applications/${encodeURIComponent(appId)}/artifacts`
  const reload = () => void api<Page<Artifact>>(path).then((p) => setArtifacts(p.data || []))
  const sorted = [...artifacts].sort((a, b) => ORDER.indexOf(a.kind || "") - ORDER.indexOf(b.kind || ""))

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Documents</CardTitle>
        <CardDescription>The résumé and cover letter sent with this application, plus what prepare produced.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
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
      </CardContent>
    </Card>
  )
}
