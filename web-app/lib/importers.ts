export const IMPORTERS: { key: string; path: string; label: string; contentType: string; accept: string }[] = [
  { key: "csv", path: "/v1/import/csv", label: "CSV", contentType: "text/csv", accept: ".csv,text/csv" },
  { key: "json", path: "/v1/import/json", label: "JSON", contentType: "application/json", accept: ".json,application/json" },
  { key: "markdown", path: "/v1/import/markdown", label: "Markdown", contentType: "text/markdown", accept: ".md,text/markdown" },
]
