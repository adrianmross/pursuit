"use client"

import { Fragment, useEffect, useState } from "react"
import { api } from "@/lib/api-client"
import { goToStripe, useFeatures, type Billing } from "@/lib/features"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemSeparator, ItemTitle } from "@/components/ui/item"
import { Spinner } from "@/components/ui/spinner"
import { Switch } from "@/components/ui/switch"

// Each feature's switch is the user's own on/off. A feature the server
// isn't set up for, or the plan doesn't include, says why instead.
export function ExperimentalPanel() {
  const { features, set } = useFeatures()
  const [error, setError] = useState("")
  if (!features) return <Spinner className="mx-auto my-6 text-muted-foreground" />
  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-muted-foreground">Early features. They start off; turn on the ones you want to try.</p>
      <ItemGroup className="gap-0 rounded-lg border">
        {features.map((f, i) => (
          <Fragment key={f.key}>
            {i > 0 && <ItemSeparator className="my-0" />}
            <Item size="sm">
              <ItemContent>
                <ItemTitle>
                  {f.name}
                  {f.premium && <Badge variant="secondary">Plus</Badge>}
                </ItemTitle>
                <ItemDescription>
                  {!f.configured ? "This server isn't set up for it yet." : !f.entitled ? "Included with Plus." : f.description}
                </ItemDescription>
              </ItemContent>
              <ItemActions>
                {f.configured && !f.entitled ? (
                  <Button variant="outline" size="sm" onClick={() => goToStripe("/v1/billing/checkout").catch((e) => setError(e.message))}>
                    Upgrade
                  </Button>
                ) : (
                  <Switch
                    checked={f.enabled}
                    disabled={!f.configured}
                    aria-label={`${f.name} on`}
                    onCheckedChange={(on) => set(f.key, on).catch((e) => setError(e.message))}
                  />
                )}
              </ItemActions>
            </Item>
          </Fragment>
        ))}
      </ItemGroup>
      {error && <p className="text-sm text-destructive">{error}</p>}
    </div>
  )
}

const STATUS: Record<string, string> = {
  active: "Active",
  trialing: "Trial",
  past_due: "Payment due",
  canceled: "Cancelled",
  unpaid: "Unpaid",
  incomplete: "Incomplete",
  paused: "Paused",
}

// The plan, on a server that sells one. Stripe's own pages change or cancel it.
export function PlanPanel() {
  const [billing, setBilling] = useState<Billing | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  useEffect(() => {
    api<Billing>("/v1/billing")
      .then(setBilling)
      .catch(() => setBilling({ enabled: false, status: "none", active: false, cancel_at_period_end: false }))
  }, [])
  if (!billing) return <Spinner className="mx-auto my-6 text-muted-foreground" />
  if (!billing.enabled) return <p className="text-sm text-muted-foreground">This server doesn&apos;t sell plans: every feature it&apos;s set up for is included.</p>
  const go = async (path: "/v1/billing/checkout" | "/v1/billing/portal") => {
    setBusy(true)
    setError("")
    try {
      await goToStripe(path)
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not open Stripe")
      setBusy(false)
    }
  }
  const end = billing.current_period_end ? new Date(billing.current_period_end).toLocaleDateString() : ""
  return (
    <div className="flex flex-col gap-3">
      <Item size="sm" variant="outline">
        <ItemContent>
          <ItemTitle>
            {billing.active ? "Plus" : "Free"}
            {billing.status !== "none" && <Badge variant="secondary">{STATUS[billing.status] || billing.status}</Badge>}
          </ItemTitle>
          <ItemDescription>
            {billing.active
              ? billing.cancel_at_period_end
                ? `Ends ${end}.`
                : end && `Renews ${end}.`
              : "Plus adds smart search and document storage."}
          </ItemDescription>
        </ItemContent>
        <ItemActions>
          {!billing.active ? (
            <Button size="sm" disabled={busy} onClick={() => go("/v1/billing/checkout")}>
              Upgrade
            </Button>
          ) : (
            <Button variant="outline" size="sm" disabled={busy} onClick={() => go("/v1/billing/portal")}>
              Manage
            </Button>
          )}
        </ItemActions>
      </Item>
      {error && <p className="text-sm text-destructive">{error}</p>}
    </div>
  )
}
