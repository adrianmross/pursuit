"use client"

import { Fragment, useEffect, useState } from "react"
import { api } from "@/lib/api-client"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemSeparator } from "@/components/ui/item"

export function AboutPanel() {
  const [version, setVersion] = useState("…")
  useEffect(() => {
    api<{ version: string }>("/version")
      .then((v) => setVersion(v.version))
      .catch(() => setVersion("unknown"))
  }, [])

  const rows: [string, string][] = [
    ["Version", version],
    ["Server", window.location.origin],
    ["Web build", process.env.NODE_ENV || "unknown"],
  ]
  return (
    <div className="flex flex-col gap-4">
      <ItemGroup className="gap-0 rounded-lg border">
        {rows.map(([k, v], i) => (
          <Fragment key={k}>
            {i > 0 && <ItemSeparator className="my-0" />}
            <Item size="sm">
              <ItemContent>
                <ItemDescription>{k}</ItemDescription>
              </ItemContent>
              <ItemActions className="font-mono">{v}</ItemActions>
            </Item>
          </Fragment>
        ))}
      </ItemGroup>
      <ButtonGroup>
        <Button variant="outline" size="sm" asChild>
          <a href="/reference">API reference</a>
        </Button>
        <Button variant="outline" size="sm" asChild>
          <a href="/docs">Docs</a>
        </Button>
        <Button variant="outline" size="sm" asChild>
          <a href="/llms.txt">llms.txt</a>
        </Button>
      </ButtonGroup>
    </div>
  )
}
