import { useEffect, useState } from "react"
import { cn } from "cn"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

export function InlineText({
  value,
  placeholder = "—",
  onSave,
  disabled,
  className = "",
  inputClassName = "",
}: {
  value: string
  placeholder?: string
  onSave: (next: string) => void
  disabled?: boolean
  className?: string
  inputClassName?: string
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(value)

  useEffect(() => {
    if (!editing) setDraft(value)
  }, [value, editing])

  if (disabled) {
    return <span className={className}>{value || <span className="text-muted-foreground">{placeholder}</span>}</span>
  }

  if (editing) {
    return (
      <Input
        autoFocus
        className={inputClassName}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={() => {
          setEditing(false)
          if (draft !== value) onSave(draft)
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") e.currentTarget.blur()
          if (e.key === "Escape") {
            setDraft(value)
            setEditing(false)
          }
        }}
      />
    )
  }

  return (
    <button
      type="button"
      className={cn(
        "w-full rounded-lg border border-transparent px-2.5 py-1 text-left transition-colors hover:border-input hover:bg-muted/40",
        className,
        inputClassName,
      )}
      onClick={() => setEditing(true)}
    >
      {value || <span className="text-muted-foreground">{placeholder}</span>}
    </button>
  )
}

export function InlineSelect({
  value,
  options,
  display,
  onSave,
  disabled,
  className = "",
}: {
  value: string
  options: [string, string][]
  display: string
  onSave: (next: string) => void
  disabled?: boolean
  className?: string
}) {
  const [editing, setEditing] = useState(false)

  if (disabled) return <span className={className}>{display}</span>

  if (editing) {
    return (
      <Select
        defaultOpen
        value={value}
        onValueChange={(next) => {
          setEditing(false)
          if (next !== value) onSave(next)
        }}
        onOpenChange={(open) => !open && setEditing(false)}
      >
        <SelectTrigger className="h-7 w-full text-sm">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map(([v, text]) => (
            <SelectItem key={v} value={v}>
              {text}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    )
  }

  return (
    <button
      type="button"
      className={cn(
        "h-7 w-full rounded-lg border border-transparent px-2.5 text-left transition-colors hover:border-input hover:bg-muted/40",
        className,
      )}
      onClick={() => setEditing(true)}
    >
      {display}
    </button>
  )
}
