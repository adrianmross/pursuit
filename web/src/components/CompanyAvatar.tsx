const COLORS = [
  "bg-rose-500",
  "bg-orange-500",
  "bg-amber-500",
  "bg-lime-500",
  "bg-emerald-500",
  "bg-teal-500",
  "bg-cyan-500",
  "bg-blue-500",
  "bg-violet-500",
  "bg-fuchsia-500",
]

function hash(value: string) {
  let h = 0
  for (let i = 0; i < value.length; i++) h = (h * 31 + value.charCodeAt(i)) >>> 0
  return h
}

export function CompanyAvatar({ company, className = "" }: { company?: string; className?: string }) {
  const name = company?.trim() || "?"
  const color = COLORS[hash(name) % COLORS.length]
  return (
    <span
      className={`inline-flex size-6 shrink-0 items-center justify-center rounded-full text-[11px] font-medium text-white ${color} ${className}`}
      aria-hidden="true"
    >
      {name[0]!.toUpperCase()}
    </span>
  )
}
