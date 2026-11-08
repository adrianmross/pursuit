// Turns CHANGELOG.md (Release Please's output) into the docs Changelog page,
// an Atom feed and a JSON Feed, so readers, bots and self-hosted installs can
// follow releases. Runs before the docs build (scripts/vercel-build.sh); the
// generated files are gitignored.
import { readFileSync, writeFileSync } from "node:fs"

const SITE = "https://hirepursuit.app/docs"
const REPO = "https://github.com/adrianmross/pursuit"
const root = new URL("../", import.meta.url)

// "## [0.6.0](compare-url) (2026-11-07)" opens a release, "### Features" a group,
// "* text ([#1](..)) ([abc](..))" an entry.
export function parse(md) {
  const releases = []
  for (const line of md.split("\n")) {
    const h = line.match(/^## \[?(\d+\.\d+\.\d+[^\]\s)]*)\]?(?:\([^)]*\))? \((\d{4}-\d{2}-\d{2})\)/)
    if (h) releases.push({ version: h[1], date: h[2], groups: [] })
    else if (releases.length && line.startsWith("### ")) releases.at(-1).groups.push({ title: line.slice(4).trim(), items: [] })
    else if (releases.length && line.startsWith("* ") && releases.at(-1).groups.length) releases.at(-1).groups.at(-1).items.push(line.slice(2).trim())
  }
  return releases
}

// Entries are markdown links already; drop the commit-hash link and turn "#N" links into plain text.
const nohash = (s) => s.replace(/\s*\(\[[0-9a-f]{7,}\]\([^)]*\)\)$/, "")
const plain = (s) => nohash(s).replace(/\[([^\]]+)\]\([^)]*\)/g, "$1").replace(/\*\*([^*]+)\*\*/g, "$1")
const mdx = (s) => s.replace(/[<>{}]/g, (c) => `\\${c}`)
const xml = (s) => s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c])

export function render(releases) {
  const page = releases
    .map((r) => `## ${r.version}\n\n${r.date} · [GitHub release](${REPO}/releases/tag/v${r.version})\n\n` +
      r.groups.map((g) => `### ${g.title}\n\n${g.items.map((i) => `- ${mdx(nohash(i))}`).join("\n")}\n`).join("\n"))
    .join("\n")
  const html = (r) => r.groups.map((g) => `<h3>${xml(g.title)}</h3><ul>${g.items.map((i) => `<li>${xml(plain(i))}</li>`).join("")}</ul>`).join("")
  const atom = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Hire Pursuit releases</title>
  <id>${SITE}/changelog</id>
  <link rel="self" href="${SITE}/changelog.xml"/>
  <link rel="alternate" href="${SITE}/changelog"/>
  <updated>${releases[0].date}T00:00:00Z</updated>
${releases.map((r) => `  <entry>
    <title>v${r.version}</title>
    <id>${SITE}/changelog#${r.version}</id>
    <link rel="alternate" href="${REPO}/releases/tag/v${r.version}"/>
    <updated>${r.date}T00:00:00Z</updated>
    <content type="html">${xml(html(r))}</content>
  </entry>`).join("\n")}
</feed>
`
  const json = {
    version: "https://jsonfeed.org/version/1.1",
    title: "Hire Pursuit releases",
    home_page_url: `${SITE}/changelog`,
    feed_url: `${SITE}/changelog.json`,
    items: releases.map((r) => ({
      id: r.version,
      url: `${REPO}/releases/tag/v${r.version}`,
      title: `v${r.version}`,
      date_published: `${r.date}T00:00:00Z`,
      content_html: html(r),
    })),
  }
  return { page, atom, json: JSON.stringify(json, null, 2) + "\n" }
}

if (import.meta.main) {
  const releases = parse(readFileSync(new URL("CHANGELOG.md", root), "utf8"))
  if (!releases.length || !releases[0].groups.length) throw new Error("CHANGELOG.md: no releases parsed")
  const { page, atom, json } = render(releases)
  const intro = readFileSync(new URL("docs-site/changelog-intro.mdx", root), "utf8")
  writeFileSync(new URL("docs-site/content/changelog.mdx", root), `${intro.trimEnd()}\n\n[Atom feed](${SITE}/changelog.xml) · [JSON Feed](${SITE}/changelog.json)\n\n${page}`)
  writeFileSync(new URL("docs-site/public/changelog.xml", root), atom)
  writeFileSync(new URL("docs-site/public/changelog.json", root), json)
  // The dashboard's start page announces minor releases (x.y.0): one line from the release's first entry.
  const minor = releases.filter((r) => r.version.endsWith(".0") && r.groups.length).map((r) => ({
    id: `v${r.version}`,
    at: `${r.date}T00:00:00Z`,
    title: `Hire Pursuit ${r.version.replace(/\.0$/, "")} is out`,
    body: plain(r.groups[0].items[0]).replace(/^pursuit:\s*/, "").replace(/\s*\(#\d+\)$/, ""),
    href: `${SITE}/changelog#${r.version.replaceAll(".", "")}`,
  }))
  writeFileSync(new URL("web-app/lib/releases.generated.json", root), JSON.stringify(minor, null, 2) + "\n")
}
