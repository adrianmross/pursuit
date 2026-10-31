"use client"

import { useEffect, useMemo, useState, type ReactNode } from "react"
import { Braces, Download, File, FileImage, FileText } from "lucide-react"
import { api } from "@/lib/api-client"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Spinner } from "@/components/ui/spinner"

type Stored = { filename?: string; content_type?: string; body: string }

export function isTextType(ct = "") {
  return ct.startsWith("text/") || ct.includes("json")
}

// The server stores binary bodies (and every resume_pdf/confirmation
// artifact) as base64, text as-is. `base64` says which this one is.
export async function loadFile(path: string, base64: (f: Stored) => boolean) {
  const f = await api<Stored>(path)
  const bytes = base64(f) ? Uint8Array.from(atob(f.body), (c) => c.charCodeAt(0)) : new TextEncoder().encode(f.body)
  return { blob: new Blob([bytes], { type: f.content_type || "application/octet-stream" }), filename: f.filename || "file", type: f.content_type || "" }
}

export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  Object.assign(document.createElement("a"), { href: url, download: filename }).click()
  URL.revokeObjectURL(url)
}

export function FileIcon({ type = "", name = "" }: { type?: string; name?: string }) {
  if (type.startsWith("image/")) return <FileImage />
  if (type.includes("json")) return <Braces />
  if (type === "application/pdf" || name.endsWith(".pdf") || isTextType(type)) return <FileText />
  return <File />
}

function Body({ blob, type }: { blob: Blob; type: string }) {
  const url = useMemo(() => URL.createObjectURL(blob), [blob])
  useEffect(() => () => URL.revokeObjectURL(url), [url])
  const [text, setText] = useState<string | null>(null)
  useEffect(() => {
    if (!isTextType(type)) return
    void blob.text().then((t) => {
      try {
        setText(type.includes("json") ? JSON.stringify(JSON.parse(t), null, 2) : t)
      } catch {
        setText(t)
      }
    })
  }, [blob, type])

  if (text != null) return <pre className="max-h-[60vh] overflow-auto rounded-md bg-muted p-3 font-mono text-xs whitespace-pre-wrap">{text}</pre>
  if (type === "application/pdf") return <iframe src={url} title="Preview" className="h-[60vh] w-full rounded-md border" />
  if (type.startsWith("image/"))
    // eslint-disable-next-line @next/next/no-img-element -- a local blob: URL, nothing for next/image to optimize
    return <img src={url} alt="" className="max-h-[60vh] w-full rounded-md object-contain" />
  return (
    <Empty className="border border-dashed">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <File />
        </EmptyMedia>
        <EmptyTitle>No preview for this file type</EmptyTitle>
        <EmptyDescription>Download it to open it.</EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

// A dialog that fetches the file when opened and previews PDFs, images,
// text, and JSON.
export function FilePreview({
  path,
  base64,
  title,
  description,
  children,
}: {
  path: string
  base64: (f: Stored) => boolean
  title: string
  description?: ReactNode
  children: ReactNode
}) {
  const [file, setFile] = useState<Awaited<ReturnType<typeof loadFile>> | null>(null)
  const [error, setError] = useState("")
  function onOpenChange(open: boolean) {
    if (!open || file) return
    loadFile(path, base64).then(setFile, (err) => setError(err instanceof Error ? err.message : "Could not load the file"))
  }
  return (
    <Dialog onOpenChange={onOpenChange}>
      <DialogTrigger asChild>{children}</DialogTrigger>
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle className="truncate">{title}</DialogTitle>
          {description && <DialogDescription>{description}</DialogDescription>}
        </DialogHeader>
        {error ? (
          <p className="text-sm text-destructive">{error}</p>
        ) : file ? (
          <Body blob={file.blob} type={file.type} />
        ) : (
          <div className="flex h-40 items-center justify-center">
            <Spinner />
          </div>
        )}
        <DialogFooter>
          <Button variant="outline" disabled={!file} onClick={() => file && saveBlob(file.blob, file.filename)}>
            <Download /> Download
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
