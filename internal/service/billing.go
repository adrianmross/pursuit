package service

import (
	"context"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/billing"
	"github.com/pursuit/pursuit/internal/blobstore"
	"github.com/pursuit/pursuit/internal/embed"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

// Feature is something a user may turn on. Key is also the feature's Stripe
// lookup_key, so attaching that feature to the product in Stripe is what
// grants it.
type Feature struct {
	Key         string
	Name        string
	Description string
	// Premium needs an entitlement when billing is on. Without Stripe
	// configured (self-hosted), every configured feature is entitled.
	Premium bool
	// Experimental starts off until the user turns it on.
	Experimental bool
	// Costly calls a paid API, so the tutorial sandbox doesn't get it.
	Costly bool
}

// Features is every feature, in the order Settings lists them.
var Features = []Feature{
	{Key: "smart_search", Name: "Smart search", Premium: true, Experimental: true, Costly: true,
		Description: "Find answers, notes and documents by meaning, and fill form questions from the closest saved answer."},
	{Key: "tailored_documents", Name: "Tailored documents", Premium: true, Experimental: true,
		Description: "Keep a tailored résumé and cover letter per application and compare each with your default."},
}

// FeatureView is one feature as this user sees it. Available is the gate:
// configured and entitled and enabled.
type FeatureView struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Premium      bool   `json:"premium"`
	Experimental bool   `json:"experimental"`
	Configured   bool   `json:"configured"`
	Entitled     bool   `json:"entitled"`
	Enabled      bool   `json:"enabled"`
	Available    bool   `json:"available"`
}

func findFeature(key string) (Feature, bool) {
	for _, f := range Features {
		if f.Key == key {
			return f, true
		}
	}
	return Feature{}, false
}

// billingOn is true when this deployment sells plans. It hangs on the key
// alone: a missing price must break Checkout, not hand everyone the plan.
func (s *Service) billingOn() bool { return s.Stripe != nil }

// comped is a user who gets every feature without paying: only the
// PURSUIT_TRUST_PROXY_AUTH owner. Anyone else, the operator included, is
// comped in Stripe (a $0 price or a 100%-off coupon on their customer), so
// the grant is tied to the account and not to a claim like an email.
func (s *Service) comped(_ context.Context, a domain.Actor) (bool, error) {
	return a.Proxy, nil
}

func (s *Service) featureViews(ctx context.Context, a domain.Actor) ([]FeatureView, error) {
	comp, err := s.comped(ctx, a)
	if err != nil {
		return nil, err
	}
	ents, err := s.DB.ListEntitlements(ctx, a.UserID)
	if err != nil {
		return nil, err
	}
	has := map[string]bool{}
	for _, e := range ents {
		has[e.Feature] = true
	}
	prefs, err := s.DB.UserFeatures(ctx, a.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]FeatureView, 0, len(Features))
	for _, f := range Features {
		v := FeatureView{Key: f.Key, Name: f.Name, Description: f.Description, Premium: f.Premium, Experimental: f.Experimental}
		v.Configured = s.configured(f.Key) && !(a.Demo && f.Costly)
		// A demo sandbox shows off what the deployment has; it can't buy anything.
		v.Entitled = !f.Premium || !s.billingOn() || a.Demo || comp || has[f.Key]
		v.Enabled = !f.Experimental
		if on, ok := prefs[f.Key]; ok {
			v.Enabled = on
		}
		v.Available = v.Configured && v.Entitled && v.Enabled
		out = append(out, v)
	}
	return out, nil
}

func (s *Service) ListFeatures(ctx context.Context) ([]FeatureView, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return nil, err
	}
	return s.featureViews(ctx, a)
}

// RequireFeature is the server-side gate every endpoint that spends money
// on a feature calls; the Settings toggle only hides UI.
func (s *Service) RequireFeature(ctx context.Context, key string) error {
	a, err := s.actor(ctx, false)
	if err != nil {
		return err
	}
	views, err := s.featureViews(ctx, a)
	if err != nil {
		return err
	}
	for _, v := range views {
		if v.Key != key {
			continue
		}
		switch {
		case !v.Configured:
			return domain.FeatureUnavailable(key, "not_configured", "this server isn't set up for "+v.Name)
		case !v.Entitled:
			return domain.FeatureUnavailable(key, "not_entitled", "upgrade under Settings › Billing, or POST /v1/billing/checkout")
		case !v.Enabled:
			return domain.FeatureUnavailable(key, "disabled", "turn it on under Settings › Experimental, or PATCH /v1/features/"+key)
		}
		return nil
	}
	return domain.NotFound("feature")
}

type FeaturePatch struct {
	Enabled *bool `json:"enabled"`
}

func (s *Service) SetFeature(ctx context.Context, key string, in FeaturePatch) (FeatureView, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return FeatureView{}, err
	}
	if _, ok := findFeature(key); !ok {
		return FeatureView{}, domain.NotFound("feature")
	}
	if in.Enabled == nil {
		return FeatureView{}, domain.Validation("enabled is required").WithHint(`{"enabled": true}`)
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.SetUserFeature(ctx, a.UserID, key, *in.Enabled, domain.FormatTime(s.now())); err != nil {
			return err
		}
		verb := "turned off "
		if *in.Enabled {
			verb = "turned on "
		}
		return s.audit(ctx, a, "feature.update", "feature", key, verb+key, map[string]any{"enabled": *in.Enabled})
	})
	if err != nil {
		return FeatureView{}, err
	}
	views, err := s.featureViews(ctx, a)
	if err != nil {
		return FeatureView{}, err
	}
	for _, v := range views {
		if v.Key == key {
			return v, nil
		}
	}
	return FeatureView{}, domain.NotFound("feature")
}

// BillingView is the user's plan. Enabled is false when this server doesn't
// sell plans, and the rest is then empty. Comped has every feature without
// a plan: the PURSUIT_TRUST_PROXY_AUTH owner.
type BillingView struct {
	Enabled           bool                   `json:"enabled"`
	Status            string                 `json:"status"`
	Active            bool                   `json:"active"`
	Comped            bool                   `json:"comped"`
	CurrentPeriodEnd  string                 `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd bool                   `json:"cancel_at_period_end"`
	Entitlements      []sqlstore.Entitlement `json:"entitlements"`
	SyncedAt          string                 `json:"synced_at,omitempty"`
}

// activeStatus is a subscription that pays for the plan. past_due keeps
// access while Stripe retries the card; Stripe's entitlements decide the
// features either way.
func activeStatus(st string) bool {
	return st == "active" || st == "trialing" || st == "past_due"
}

func (s *Service) Billing(ctx context.Context) (BillingView, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return BillingView{}, err
	}
	v, err := s.billingView(ctx, a.UserID)
	if err != nil {
		return v, err
	}
	v.Comped, err = s.comped(ctx, a)
	return v, err
}

func (s *Service) billingView(ctx context.Context, userID string) (BillingView, error) {
	v := BillingView{Enabled: s.billingOn(), Status: "none", Entitlements: []sqlstore.Entitlement{}}
	ents, err := s.DB.ListEntitlements(ctx, userID)
	if err != nil {
		return v, err
	}
	v.Entitlements = ents
	b, err := s.DB.GetBilling(ctx, userID)
	if domain.IsCode(err, "not_found") {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	v.Status, v.Active, v.CurrentPeriodEnd, v.CancelAtPeriodEnd, v.SyncedAt = b.Status, activeStatus(b.Status), b.CurrentPeriodEnd, b.CancelAtPeriodEnd, b.SyncedAt
	return v, nil
}

func (s *Service) billingActor(ctx context.Context) (domain.Actor, error) {
	a, err := s.actor(ctx, true)
	if err != nil {
		return a, err
	}
	if !s.billingOn() {
		return a, domain.FeatureUnavailable("billing", "not_configured", "this server doesn't sell plans; every configured feature is already on")
	}
	if a.Demo {
		return a, domain.Validation("the tutorial sandbox can't subscribe").WithHint("sign in to your own account to upgrade")
	}
	return a, nil
}

// customer returns the user's Stripe customer, creating it first: Checkout
// always runs against a known customer, never one it makes up.
func (s *Service) customer(ctx context.Context, a domain.Actor) (string, error) {
	b, err := s.DB.GetBilling(ctx, a.UserID)
	if err == nil {
		return b.CustomerID, nil
	}
	if !domain.IsCode(err, "not_found") {
		return "", err
	}
	u, err := s.DB.GetUser(ctx, a.UserID)
	if err != nil {
		return "", err
	}
	id, err := s.Stripe.CreateCustomer(ctx, u.ID, u.Email, u.Name)
	if err != nil {
		return "", err
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.PutBilling(ctx, sqlstore.BillingAccount{UserID: a.UserID, CustomerID: id, Status: "none"}); err != nil {
			return err
		}
		return s.audit(ctx, a, "billing.customer", "user", a.UserID, "created Stripe customer", map[string]any{"customer": id})
	})
	return id, err
}

// Checkout returns a Stripe Checkout URL for the plan. base is the
// dashboard's origin, where Stripe sends the user back. code is an invite:
// a Stripe promotion code applied up front, so a 100%-off one needs no card.
func (s *Service) Checkout(ctx context.Context, base, code string) (string, error) {
	a, err := s.billingActor(ctx)
	if err != nil {
		return "", err
	}
	if comp, err := s.comped(ctx, a); err != nil {
		return "", err
	} else if comp {
		return "", domain.Conflict("your account already has every feature", "nothing to buy")
	}
	if v, err := s.billingView(ctx, a.UserID); err != nil {
		return "", err
	} else if v.Active {
		return "", domain.Conflict("you already have a plan", "manage it in the billing portal: POST /v1/billing/portal")
	}
	if s.StripePrice == "" {
		return "", domain.FeatureUnavailable("billing", "not_configured", "the server has no STRIPE_PRICE_ID; try again later")
	}
	cus, err := s.customer(ctx, a)
	if err != nil {
		return "", err
	}
	promo := ""
	if code = strings.TrimSpace(code); code != "" {
		if promo, err = s.Stripe.PromotionCode(ctx, code); err != nil {
			return "", err
		}
		if promo == "" {
			return "", domain.Validation("this invite has expired or been used up").WithHint("ask whoever sent it for a new link")
		}
	}
	return s.Stripe.CheckoutURL(ctx, cus, s.StripePrice, promo, base+"/start?billing=success", base+"/start?billing=cancel")
}

// Portal returns a Stripe customer portal URL.
func (s *Service) Portal(ctx context.Context, base string) (string, error) {
	a, err := s.billingActor(ctx)
	if err != nil {
		return "", err
	}
	b, err := s.DB.GetBilling(ctx, a.UserID)
	if domain.IsCode(err, "not_found") {
		return "", domain.Validation("you don't have a plan yet").WithHint("POST /v1/billing/checkout")
	}
	if err != nil {
		return "", err
	}
	return s.Stripe.PortalURL(ctx, b.CustomerID, base+"/start")
}

// SyncMyBilling re-reads the user's plan now, for the page Checkout returns
// to: the user can land there before any webhook does.
func (s *Service) SyncMyBilling(ctx context.Context) (BillingView, error) {
	a, err := s.billingActor(ctx)
	if err != nil {
		return BillingView{}, err
	}
	b, err := s.DB.GetBilling(ctx, a.UserID)
	if domain.IsCode(err, "not_found") {
		return s.billingView(ctx, a.UserID)
	}
	if err != nil {
		return BillingView{}, err
	}
	if err := s.syncCustomer(ctx, b); err != nil {
		return BillingView{}, err
	}
	return s.billingView(ctx, a.UserID)
}

// syncCustomer is the one place billing state is written: it reads the
// subscription and active entitlements from Stripe and overwrites ours,
// whatever triggered it. Webhook payloads only say whom to re-read.
func (s *Service) syncCustomer(ctx context.Context, b sqlstore.BillingAccount) error {
	sub, err := s.Stripe.LatestSubscription(ctx, b.CustomerID)
	if err != nil {
		return err
	}
	keys, err := s.Stripe.ActiveEntitlements(ctx, b.CustomerID)
	if err != nil {
		return err
	}
	now := domain.FormatTime(s.now())
	next := sqlstore.BillingAccount{UserID: b.UserID, CustomerID: b.CustomerID, Status: "none", SyncedAt: now}
	if sub != nil {
		next.SubscriptionID, next.Status, next.PriceID, next.CancelAtPeriodEnd = sub.ID, sub.Status, sub.PriceID, sub.CancelAtPeriodEnd
		if !sub.CurrentPeriodEnd.IsZero() {
			next.CurrentPeriodEnd = sub.CurrentPeriodEnd.Format(time.RFC3339)
		}
	}
	before, err := s.DB.ListEntitlements(ctx, b.UserID)
	if err != nil {
		return err
	}
	had := map[string]bool{}
	for _, e := range before {
		had[e.Feature] = true
	}
	return s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.PutBilling(ctx, next); err != nil {
			return err
		}
		if err := s.DB.ReplaceEntitlements(ctx, b.UserID, "stripe", keys, now); err != nil {
			return err
		}
		// Paying for a feature is opting in: turn on what was just bought,
		// so the user doesn't come back from Checkout to switches still off.
		for _, k := range keys {
			if _, known := findFeature(k); known && !had[k] {
				if err := s.DB.SetUserFeature(ctx, b.UserID, k, true, now); err != nil {
					return err
				}
			}
		}
		sys := domain.Actor{UserID: b.UserID, Name: "stripe"}
		return s.audit(ctx, sys, "billing.sync", "user", b.UserID, "plan is "+next.Status, map[string]any{"status": next.Status, "entitlements": keys})
	})
}

// stripeEvents are the deliveries that can change a plan or its features.
// Anything else is acknowledged and ignored.
var stripeEvents = map[string]bool{
	"checkout.session.completed":                      true,
	"customer.subscription.created":                   true,
	"customer.subscription.updated":                   true,
	"customer.subscription.deleted":                   true,
	"customer.subscription.paused":                    true,
	"customer.subscription.resumed":                   true,
	"customer.subscription.pending_update_applied":    true,
	"customer.subscription.pending_update_expired":    true,
	"customer.subscription.trial_will_end":            true,
	"invoice.paid":                                    true,
	"invoice.payment_failed":                          true,
	"invoice.payment_action_required":                 true,
	"invoice.marked_uncollectible":                    true,
	"invoice.payment_succeeded":                       true,
	"entitlements.active_entitlement_summary.updated": true,
}

// HandleStripeEvent verifies a webhook delivery and syncs its customer.
// A duplicate, an ignored type, or a customer this server doesn't know is
// not an error: Stripe only needs to stop retrying.
func (s *Service) HandleStripeEvent(ctx context.Context, body []byte, signature string) error {
	if s.Stripe == nil || s.StripeWebhookSecret == "" {
		return domain.NotFound("billing webhook")
	}
	ev, err := billing.Verify(body, signature, s.StripeWebhookSecret, s.now())
	if err != nil {
		return domain.Validation("invalid Stripe signature").WithHint("check STRIPE_WEBHOOK_SECRET matches this endpoint's signing secret")
	}
	if !stripeEvents[ev.Type] || ev.CustomerID() == "" {
		return nil
	}
	fresh, err := s.DB.MarkStripeEvent(ctx, ev.ID, ev.Type, domain.FormatTime(s.now()))
	if err != nil || !fresh {
		return err
	}
	b, err := s.DB.GetBillingByCustomer(ctx, ev.CustomerID())
	if domain.IsCode(err, "not_found") {
		return nil
	}
	if err == nil {
		err = s.syncCustomer(ctx, b)
	}
	if err != nil {
		// Let Stripe's retry run the sync again.
		_ = s.DB.ForgetStripeEvent(ctx, ev.ID)
	}
	return err
}

// GrantFeature comps a feature outside Stripe. There is no route for it:
// it's for the operator, from code, and a sync never removes it.
func (s *Service) GrantFeature(ctx context.Context, userID, key string, on bool) error {
	if _, ok := findFeature(key); !ok {
		return domain.NotFound("feature")
	}
	ents, err := s.DB.ListEntitlements(ctx, userID)
	if err != nil {
		return err
	}
	keys := []string{}
	for _, e := range ents {
		if e.Source == "manual" && e.Feature != key {
			keys = append(keys, e.Feature)
		}
	}
	if on {
		keys = append(keys, key)
	}
	return s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.ReplaceEntitlements(ctx, userID, "manual", keys, domain.FormatTime(s.now())); err != nil {
			return err
		}
		return s.audit(ctx, domain.Actor{UserID: userID, Name: "operator"}, "entitlement.grant", "feature", key, "manual "+key, map[string]any{"on": on})
	})
}

// ConfigureFromEnv sets billing, document storage and feature capabilities
// from the environment, the same for `pursuit serve` and the Vercel function.
// Billing needs STRIPE_SECRET_KEY and STRIPE_PRICE_ID; the webhook also
// needs STRIPE_WEBHOOK_SECRET. It returns what's half set up, for the
// caller to log loudly.
func (s *Service) ConfigureFromEnv(getenv func(string) string, publicURL string) (warnings []string) {
	env := func(k string) string { return strings.TrimSpace(getenv(k)) }
	if key := env("STRIPE_SECRET_KEY"); key != "" {
		s.Stripe = &billing.Client{Key: key, HTTP: s.HTTP}
		s.StripePrice = env("STRIPE_PRICE_ID")
		s.StripeWebhookSecret = env("STRIPE_WEBHOOK_SECRET")
		if s.StripePrice == "" {
			warnings = append(warnings, "STRIPE_SECRET_KEY is set but STRIPE_PRICE_ID isn't: nobody can upgrade")
		}
		if s.StripeWebhookSecret == "" {
			warnings = append(warnings, "STRIPE_SECRET_KEY is set but STRIPE_WEBHOOK_SECRET isn't: renewals and cancellations won't sync")
		}
	}
	s.PublicURL = publicURL
	if key := env("AI_GATEWAY_API_KEY"); key != "" {
		// AI_GATEWAY_BASE_URL is for a proxy in front of the gateway; empty is the gateway itself.
		s.Embedder = &embed.Gateway{Key: key, HTTP: s.HTTP, Base: env("AI_GATEWAY_BASE_URL")}
	}
	if bucket, err := blobstore.FromEnv(getenv); err != nil {
		warnings = append(warnings, err.Error()+": documents stay in the database")
	} else if bucket != nil {
		// Only a real bucket: a nil *S3 in the interface would read as configured.
		s.Blobs = bucket
	}
	s.Capabilities = map[string]bool{
		"smart_search": env("AI_GATEWAY_API_KEY") != "",
		// Needs nothing beyond the server itself.
		"tailored_documents": true,
	}
	return warnings
}

// configured is whether this deployment has what a feature needs.
func (s *Service) configured(key string) bool {
	if key == "smart_search" {
		return s.Embedder != nil
	}
	if key == "tailored_documents" {
		return true // needs nothing beyond the server itself
	}
	return s.Capabilities[key]
}

// BaseURL is where Stripe returns the user: the configured public URL, else
// the origin the request came to.
func (s *Service) BaseURL(requestOrigin string) string {
	if s.PublicURL != "" {
		return strings.TrimRight(s.PublicURL, "/")
	}
	return strings.TrimRight(requestOrigin, "/")
}
