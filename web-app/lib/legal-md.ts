import { readFileSync } from "node:fs"
import { join } from "node:path"

// The legal pages are plain markdown (headings, paragraphs, bullet lists, and
// **bold**, `code` and [links] inside them), so this renders just that. The
// text is ours, from the repository, never visitor input.
const esc = (s: string) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
const inline = (s: string) =>
  esc(s)
    .replace(/`([^`]+)`/g, '<code class="rounded bg-muted px-1 py-0.5 text-[0.9em]">$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a class="underline underline-offset-4" href="$2">$1</a>')

export function legalHtml(name: "privacy" | "terms") {
  const out: string[] = []
  let list = false
  for (const line of readFileSync(join(process.cwd(), "content/legal", `${name}.md`), "utf8").split("\n")) {
    const item = line.startsWith("- ") ? line.slice(2) : null
    if (list && item === null) (out.push("</ul>"), (list = false))
    if (item !== null) {
      if (!list) (out.push('<ul class="my-3 list-disc space-y-2 pl-6">'), (list = true))
      out.push(`<li>${inline(item)}</li>`)
    } else if (line.startsWith("# ")) out.push(`<h1 class="font-display text-3xl font-semibold">${inline(line.slice(2))}</h1>`)
    else if (line.startsWith("## ")) out.push(`<h2 class="mt-10 text-xl font-semibold">${inline(line.slice(3))}</h2>`)
    else if (line.trim()) out.push(`<p class="my-3 leading-7">${inline(line)}</p>`)
  }
  if (list) out.push("</ul>")
  return out.join("\n")
}
