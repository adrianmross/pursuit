import type { Artifact } from "@/lib/types"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export function ArtifactsCard({ artifacts }: { artifacts: Artifact[] }) {
  if (artifacts.length === 0) return null
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Artifacts</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-2">
        {artifacts.map((a) => (
          <div key={a.id} className="flex items-center justify-between rounded-md border p-2 text-sm">
            <span>{a.filename || a.kind}</span>
            <span className="text-muted-foreground">{a.kind}</span>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
