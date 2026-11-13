"use client"

import { Fragment, useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { api } from "@adrianmross/appkit-ui"
import { useAuth } from "@adrianmross/appkit-ui"
import { setPref } from "@adrianmross/appkit-ui"
import { ConfirmDialog } from "@adrianmross/appkit-ui"
import { Button } from "@adrianmross/appkit-ui/ui/button"
import { ButtonGroup } from "@adrianmross/appkit-ui/ui/button-group"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemSeparator, ItemTitle } from "@adrianmross/appkit-ui/ui/item"

// Signs into a fresh seeded sandbox (POST /v1/demo). Only where the deployment has demo mode on.
function Tutorial({ onClose }: { onClose: () => void }) {
  const router = useRouter()
  const { startDemo, isDemo, error } = useAuth()
  const [available, setAvailable] = useState(false)
  const [confirm, setConfirm] = useState(false)
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    api<{ demo_available: boolean }>("/v1/auth/providers")
      .then((p) => setAvailable(p.demo_available))
      .catch(() => {})
  }, [])
  if (!available) return null
  const start = async () => {
    setBusy(true)
    try {
      await startDemo()
      setPref("tour", "go")
      // Out of Settings and back to the start page, where the tour begins.
      onClose()
      router.push("/start")
    } finally {
      setBusy(false)
      setConfirm(false)
    }
  }
  return (
    <Item size="sm" variant="outline">
      <ItemContent>
        <ItemTitle>Tutorial</ItemTitle>
        <ItemDescription>{error || "A throwaway sandbox with example applications to try things in."}</ItemDescription>
      </ItemContent>
      <ItemActions>
        <Button variant="outline" size="sm" onClick={() => setConfirm(true)}>
          {isDemo ? "Replay tutorial" : "Try the tutorial"}
        </Button>
      </ItemActions>
      <ConfirmDialog
        open={confirm}
        onOpenChange={setConfirm}
        title={isDemo ? "Start a new sandbox?" : "Switch to the tutorial?"}
        description={
          isDemo
            ? "You get a brand-new sandbox. Whatever you changed in this one is left behind."
            : "You're signed into a sandbox with example data. Your account is untouched; sign back in to return to it."
        }
        confirmLabel={isDemo ? "Replay" : "Start tutorial"}
        busy={busy}
        onConfirm={start}
      />
    </Item>
  )
}

// Runs the guided tour again in this account, no sandbox needed.
function ReplayTour({ onClose }: { onClose: () => void }) {
  const router = useRouter()
  return (
    <Item size="sm" variant="outline">
      <ItemContent>
        <ItemTitle>Guided tour</ItemTitle>
        <ItemDescription>A short walk through the dashboard.</ItemDescription>
      </ItemContent>
      <ItemActions>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            setPref("tour", "go")
            onClose()
            router.push("/start")
          }}
        >
          Replay tour
        </Button>
      </ItemActions>
    </Item>
  )
}

export function AboutPanel({ onClose }: { onClose: () => void }) {
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
      <ReplayTour onClose={onClose} />
      <Tutorial onClose={onClose} />
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
