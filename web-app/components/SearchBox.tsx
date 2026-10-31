"use client"

import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react"
import { usePathname, useRouter, useSearchParams } from "next/navigation"
import commandScore from "command-score"
import { Search } from "lucide-react"
import { api } from "@/lib/api-client"
import { applicationHref } from "@/lib/links"
import { QUALIFIER_KEYS, matchesQualifiers, parseSearch, tokenAt, tokenize, type Token } from "@/lib/search"
import { getPref } from "@/lib/settings"
import { FILTERS, MORE, label, type Application, type Page } from "@/lib/types"
import { cn } from "@/lib/utils"
import { Input } from "@/components/ui/input"

type Item = { id: string; group: string; text: string; hint?: string; apply?: string; appId?: string; count?: number }

const STATUSES = FILTERS.concat(MORE).filter(([v]) => v)
const KIND: Record<Token["kind"], string> = {
  space: "",
  free: "",
  key: "font-medium text-sky-600 dark:text-sky-400",
  op: "text-amber-600 dark:text-amber-400",
  value: "text-emerald-700 dark:text-emerald-400",
  unknown: "text-muted-foreground",
}
// Fixed so the typed text and its highlighted copy line up exactly.
const TEXT = { fontSize: "0.9375rem", lineHeight: "1.5rem" }

const ago = (days: number) => new Date(Date.now() - days * 864e5).toISOString().slice(0, 10)

function valueItems(key: string, partial: string, apps: Application[], tags: string[]): Item[] {
  const p = partial.toLowerCase()
  const mk = (v: string, hint?: string, text = v): Item => ({ id: key + v, group: key, text, hint, apply: `${key}:${v}` })
  const uniq = (xs: (string | undefined)[]) => [...new Set(xs.filter((x): x is string => !!x && !/\s/.test(x)))]
  switch (key) {
    case "status":
      return STATUSES.filter(([v, l]) => v.includes(p) || l.toLowerCase().includes(p)).map(([v, l]) => mk(v, l))
    case "tag":
      return tags.filter((t) => t.toLowerCase().includes(p)).map((t) => mk(t))
    case "company":
      return uniq(apps.map((a) => a.company)).filter((c) => c.toLowerCase().includes(p)).map((c) => mk(c))
    case "title":
      return uniq(apps.flatMap((a) => (a.title || "").split(/\s+/))).filter((t) => t.toLowerCase().includes(p)).map((t) => mk(t))
    case "applied":
      return [
        mk(">" + ago(7), "after 7 days ago"),
        mk(">" + ago(30), "after 30 days ago"),
        mk(ago(0).slice(0, 7), "this month"),
        mk(ago(0).slice(0, 4), "this year"),
      ].filter((i) => i.text.includes(p))
    default:
      return []
  }
}

export function SearchBox({ placeholder, onBlurEmpty }: { placeholder: string; onBlurEmpty?: () => void }) {
  const pathname = usePathname()
  const router = useRouter()
  const params = useSearchParams()
  const onPipeline = pathname === "/applications"
  const [value, setValue] = useState(onPipeline ? params.get("q") || "" : "")
  const [caret, setCaret] = useState(0)
  const [scroll, setScroll] = useState(0)
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(-1)
  const [apps, setApps] = useState<Application[]>([])
  const [tags, setTags] = useState<string[]>([])
  const input = useRef<HTMLInputElement>(null)

  useEffect(() => {
    // Syncs the box with the URL's ?q= when it changes elsewhere.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (onPipeline) setValue(params.get("q") || "")
  }, [onPipeline, params])

  function load() {
    api<Page<Application>>("/v1/applications?limit=200&sort=-updated_at").then((p) => setApps(p.data || [])).catch(() => {})
    api<Page<{ name: string }>>("/v1/tags?limit=100").then((p) => setTags((p.data || []).map((t) => t.name))).catch(() => {})
  }

  function commit(next: string) {
    setValue(next)
    window.clearTimeout((window as unknown as { __search?: number }).__search)
    ;(window as unknown as { __search?: number }).__search = window.setTimeout(() => {
      const merged = new URLSearchParams(onPipeline ? params.toString() : undefined)
      if (next) merged.set("q", next)
      else merged.delete("q")
      router[onPipeline ? "replace" : "push"](`/applications?${merged.toString()}`)
    }, 200)
  }

  const items = useMemo(() => {
    if (!open) return []
    const syntax = getPref("searchSyntax") !== "off"
    const tok = tokenAt(value, caret)
    const out: Item[] = []
    const m = syntax ? tok.text.match(/^([a-zA-Z_]+):(.*)$/) : null
    if (m && QUALIFIER_KEYS.some(([k]) => k === m[1].toLowerCase())) {
      // Count against the applications the other filters already keep, so a
      // choice that would leave nothing is not offered.
      const others = parseSearch(value.slice(0, tok.start) + " " + value.slice(tok.end)).qualifiers
      const pool = others.length ? apps.filter((a) => matchesQualifiers(a, others)) : apps
      out.push(
        ...valueItems(m[1].toLowerCase(), m[2], pool, tags)
          .map((item) => ({ ...item, count: pool.filter((a) => matchesQualifiers(a, parseSearch(item.apply!).qualifiers)).length }))
          .filter((item) => item.count > 0)
          .sort((a, b) => b.count - a.count)
          .slice(0, 8),
      )
    } else {
      if (syntax && (tok.text || !value)) {
        out.push(
          ...QUALIFIER_KEYS.filter(([k]) => k.startsWith(tok.text.toLowerCase()) && k !== tok.text.toLowerCase()).map(
            ([k, hint]): Item => ({ id: "q" + k, group: "Filters", text: k + ":", hint, apply: k + ":" }),
          ),
        )
      }
      const { free, qualifiers } = parseSearch(value)
      // Suggest only applications the current filters would keep.
      const pool = qualifiers.length ? apps.filter((a) => matchesQualifiers(a, qualifiers)) : apps
      const ranked = free
        ? pool.map((a) => ({ a, s: commandScore([a.company, a.title, label(a.status)].filter(Boolean).join(" "), free) })).filter((x) => x.s > 0).sort((x, y) => y.s - x.s).map((x) => x.a)
        : pool
      out.push(
        ...ranked.slice(0, 5).map((a): Item => ({ id: "a" + a.id, group: "Applications", text: a.title || "Untitled", hint: a.company, appId: a.id })),
      )
    }
    return out
  }, [open, value, caret, apps, tags])

  function pick(item: Item) {
    if (item.appId) {
      setOpen(false)
      router.push(applicationHref(item.appId))
      return
    }
    const tok = tokenAt(value, caret)
    const complete = !item.apply!.endsWith(":")
    const next = value.slice(0, tok.start) + item.apply + (complete ? " " : "") + value.slice(tok.end).replace(/^\s+/, "")
    const pos = tok.start + item.apply!.length + (complete ? 1 : 0)
    commit(next)
    setCaret(pos)
    setActive(-1)
    requestAnimationFrame(() => input.current?.setSelectionRange(pos, pos))
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    const down = e.key === "ArrowDown" || (e.ctrlKey && e.key === "n")
    const up = e.key === "ArrowUp" || (e.ctrlKey && e.key === "p")
    if (e.key === "Escape") {
      if (open && items.length) setOpen(false)
      else e.currentTarget.blur()
    } else if ((down || up) && items.length) {
      e.preventDefault()
      setOpen(true)
      setActive((i) => (down ? (i + 1) % items.length : (i <= 0 ? items.length : i) - 1))
    } else if (e.key === "Enter" && active >= 0 && items[active]) {
      e.preventDefault()
      pick(items[active])
    } else if (e.key === "Tab" && !e.shiftKey && items.length && items[active >= 0 ? active : 0].apply) {
      e.preventDefault()
      pick(items[active >= 0 ? active : 0])
    }
  }

  const sync = (el: HTMLInputElement) => {
    setCaret(el.selectionStart ?? el.value.length)
    setScroll(el.scrollLeft)
  }
  let lastGroup = ""

  return (
    <div className="relative w-full">
      <Search className="pointer-events-none absolute top-1/2 left-3 z-10 size-4 -translate-y-1/2 text-muted-foreground" />
      <div aria-hidden className="pointer-events-none absolute inset-0 flex items-center overflow-hidden pl-9 pr-3" style={TEXT}>
        <span className="whitespace-pre" style={{ transform: `translateX(${-scroll}px)` }}>
          {tokenize(value).map((t, i) => (
            <span key={i} className={KIND[t.kind]}>
              {t.text}
            </span>
          ))}
        </span>
      </div>
      <Input
        ref={input}
        data-search="global"
        role="combobox"
        aria-expanded={open && items.length > 0}
        aria-controls="search-suggestions"
        aria-label="Search applications"
        placeholder={placeholder}
        autoComplete="off"
        spellCheck={false}
        autoFocus={!!onBlurEmpty}
        className="h-10 pl-9 text-transparent caret-foreground"
        style={TEXT}
        value={value}
        onChange={(e) => {
          commit(e.target.value)
          sync(e.target)
          setOpen(true)
          setActive(-1)
        }}
        onFocus={(e) => {
          load()
          sync(e.currentTarget)
          setOpen(true)
        }}
        onBlur={(e) => {
          setOpen(false)
          if (!e.currentTarget.value) onBlurEmpty?.()
        }}
        onSelect={(e) => sync(e.currentTarget)}
        onScroll={(e) => setScroll(e.currentTarget.scrollLeft)}
        onKeyUp={(e) => sync(e.currentTarget)}
        onClick={() => setOpen(true)}
        onKeyDown={onKeyDown}
      />
      {open && items.length > 0 && (
        <div
          id="search-suggestions"
          role="listbox"
          className="absolute top-full right-0 left-0 z-50 mt-1 max-h-80 overflow-y-auto rounded-lg border bg-popover p-1 text-popover-foreground shadow-md"
        >
          {items.map((item, i) => {
            const header = item.group !== lastGroup ? ((lastGroup = item.group), item.group) : null
            return (
              <div key={item.id}>
                {header && (
                  <div className="px-2 pt-1.5 pb-1 text-xs font-medium text-muted-foreground first-letter:uppercase">
                    {header === "Filters" || header === "Applications" ? header : `${header} values`}
                  </div>
                )}
                <div
                  role="option"
                  aria-selected={i === active}
                  // mouseDown, not click: keep focus in the input.
                  onMouseDown={(e) => {
                    e.preventDefault()
                    pick(item)
                  }}
                  onMouseEnter={() => setActive(i)}
                  className={cn("flex cursor-pointer items-baseline justify-between gap-3 rounded-md px-2 py-1.5 text-sm", i === active && "bg-muted")}
                >
                  <span className={cn("truncate", item.apply && "font-mono text-xs")}>{item.text}</span>
                  <span className="flex shrink-0 items-baseline gap-2">
                    {item.hint && <span className="truncate text-xs text-muted-foreground">{item.hint}</span>}
                    {item.count != null && <span className="min-w-5 text-right text-xs font-medium tabular-nums">{item.count}</span>}
                  </span>
                </div>
              </div>
            )
          })}
          <div className="border-t px-2 pt-1.5 pb-1 text-[11px] text-muted-foreground">↑↓ move · Tab or Enter select · Esc close</div>
        </div>
      )}
    </div>
  )
}
