package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pursuit/pursuit/internal/billing"
	"github.com/pursuit/pursuit/internal/service"
	"github.com/pursuit/pursuit/internal/sqlstore"
	"github.com/pursuit/pursuit/internal/sqlstore/sqlstoretest"
)

// fakeStripe answers the calls billing makes. ents is what the customer
// is entitled to right now; reads counts the entitlement reads (one per sync).
type fakeStripe struct {
	ents     atomic.Value // []string
	reads    atomic.Int32
	checkout atomic.Value // the last Checkout form
}

func (f *fakeStripe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/customers":
		_, _ = w.Write([]byte(`{"id":"cus_1"}`))
	case "/v1/checkout/sessions":
		_ = r.ParseForm()
		f.checkout.Store(r.PostForm)
		_, _ = w.Write([]byte(`{"url":"https://checkout.stripe.test/s"}`))
	case "/v1/promotion_codes":
		if r.URL.Query().Get("code") == "FRIEND" && r.URL.Query().Get("active") == "true" {
			_, _ = w.Write([]byte(`{"data":[{"id":"promo_1"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	case "/v1/billing_portal/sessions":
		_, _ = w.Write([]byte(`{"url":"https://billing.stripe.test/p"}`))
	case "/v1/subscriptions":
		if len(f.ents.Load().([]string)) == 0 {
			_, _ = w.Write([]byte(`{"data":[{"id":"sub_1","status":"canceled","items":{"data":[]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"sub_1","status":"active","items":{"data":[{"current_period_end":1900000000,"price":{"id":"price_1"}}]}}]}`))
	case "/v1/entitlements/active_entitlements":
		f.reads.Add(1)
		var data []string
		for _, k := range f.ents.Load().([]string) {
			data = append(data, `{"id":"ent_`+k+`","lookup_key":"`+k+`"}`)
		}
		_, _ = w.Write([]byte(`{"data":[` + strings.Join(data, ",") + `],"has_more":false}`))
	default:
		w.WriteHeader(404)
	}
}

type billingEnv struct {
	srv    *httptest.Server
	svc    *service.Service
	stripe *fakeStripe
	token  string
}

func newBillingEnv(t *testing.T, withStripe bool) billingEnv {
	t.Helper()
	db, err := sqlstore.Open(context.Background(), sqlstoretest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	svc := service.New(db)
	fake := &fakeStripe{}
	fake.ents.Store([]string{})
	env := map[string]string{"AI_GATEWAY_API_KEY": "k"}
	if withStripe {
		st := httptest.NewServer(fake)
		t.Cleanup(st.Close)
		env["STRIPE_SECRET_KEY"], env["STRIPE_PRICE_ID"], env["STRIPE_WEBHOOK_SECRET"] = "sk_test", "price_1", "whsec_test"
		svc.ConfigureFromEnv(func(k string) string { return env[k] }, "https://app.test")
		svc.Stripe.Base = st.URL
	} else {
		svc.ConfigureFromEnv(func(k string) string { return env[k] }, "https://app.test")
	}
	token, _, err := svc.Bootstrap(context.Background(), "Owner", "owner@example.com", "owner", "")
	if err != nil {
		t.Fatal(err)
	}
	api := &Server{Svc: svc, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	api.Ready.Store(true)
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	return billingEnv{srv: srv, svc: svc, stripe: fake, token: token}
}

func (e billingEnv) call(t *testing.T, token, method, path, body string, out any) int {
	t.Helper()
	req, _ := http.NewRequest(method, e.srv.URL+path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func (e billingEnv) features(t *testing.T, token string) map[string]service.FeatureView {
	t.Helper()
	var out struct {
		Data []service.FeatureView `json:"data"`
	}
	if code := e.call(t, token, "GET", "/v1/features", "", &out); code != 200 {
		t.Fatalf("GET /v1/features: %d", code)
	}
	m := map[string]service.FeatureView{}
	for _, f := range out.Data {
		m[f.Key] = f
	}
	return m
}

func (e billingEnv) webhook(t *testing.T, id, typ, signWith string) int {
	t.Helper()
	body := `{"id":"` + id + `","type":"` + typ + `","data":{"object":{"object":"entitlements.active_entitlement_summary","customer":"cus_1"}}}`
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/billing/webhook", strings.NewReader(body))
	req.Header.Set("Stripe-Signature", billing.Sign([]byte(body), signWith, time.Now()))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// Self-hosted: no Stripe, so whatever is configured is entitled, and the
// experimental switch is the only thing left to flip.
func TestFeaturesWithoutBilling(t *testing.T) {
	e := newBillingEnv(t, false)
	f := e.features(t, e.token)
	if s := f["smart_search"]; !s.Configured || !s.Entitled || s.Enabled || s.Available {
		t.Fatalf("smart_search before opting in: %+v", s)
	}
	if d := e.features(t, e.token); !d["tailored_documents"].Configured || d["smart_search"].Configured != true {
		t.Fatalf("capabilities: %+v", d)
	}
	var v service.FeatureView
	if code := e.call(t, e.token, "PATCH", "/v1/features/smart_search", `{"enabled":true}`, &v); code != 200 || !v.Available {
		t.Fatalf("enable: %d %+v", code, v)
	}
	var errBody struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if code := e.call(t, e.token, "POST", "/v1/billing/checkout", "", &errBody); code != 403 || errBody.Error.Code != "feature_unavailable" {
		t.Fatalf("checkout without billing: %d %+v", code, errBody)
	}
	if code := e.call(t, "", "POST", "/v1/billing/webhook", "{}", nil); code != 404 {
		t.Fatalf("webhook without billing: %d", code)
	}
}

func TestBillingSyncFromStripe(t *testing.T) {
	e := newBillingEnv(t, true)
	ctx := context.Background()
	if f := e.features(t, e.token)["smart_search"]; f.Entitled {
		t.Fatalf("entitled before paying: %+v", f)
	}
	var co map[string]string
	if code := e.call(t, e.token, "POST", "/v1/billing/checkout", "", &co); code != 200 || co["url"] == "" {
		t.Fatalf("checkout: %d %v", code, co)
	}

	// The user pays; Stripe now reports the feature, and says so by webhook.
	e.stripe.ents.Store([]string{"smart_search"})
	if code := e.webhook(t, "evt_1", "entitlements.active_entitlement_summary.updated", "whsec_wrong"); code != 400 {
		t.Fatalf("bad signature: %d", code)
	}
	if code := e.webhook(t, "evt_1", "entitlements.active_entitlement_summary.updated", "whsec_test"); code != 200 {
		t.Fatalf("webhook: %d", code)
	}
	if code := e.webhook(t, "evt_1", "entitlements.active_entitlement_summary.updated", "whsec_test"); code != 200 || e.stripe.reads.Load() != 1 {
		t.Fatalf("duplicate delivery re-synced: %d reads=%d", code, e.stripe.reads.Load())
	}
	if code := e.webhook(t, "evt_2", "customer.created", "whsec_test"); code != 200 || e.stripe.reads.Load() != 1 {
		t.Fatalf("ignored type synced: reads=%d", e.stripe.reads.Load())
	}
	var b service.BillingView
	if e.call(t, e.token, "GET", "/v1/billing", "", &b); !b.Active || b.Status != "active" || len(b.Entitlements) != 1 {
		t.Fatalf("billing after webhook: %+v", b)
	}
	if f := e.features(t, e.token)["smart_search"]; !f.Entitled || !f.Enabled || !f.Available {
		t.Fatalf("paying should entitle and turn it on: %+v", f)
	}

	// A comp survives the sync that follows a cancellation; the paid feature doesn't.
	me := meViaToken(t, e.srv, e.token)
	if err := e.svc.GrantFeature(ctx, me.User.ID, "tailored_documents", true); err != nil {
		t.Fatal(err)
	}
	e.stripe.ents.Store([]string{})
	if code := e.call(t, e.token, "POST", "/v1/billing/sync", "", &b); code != 200 || b.Active || b.Status != "canceled" {
		t.Fatalf("sync after cancel: %d %+v", code, b)
	}
	f := e.features(t, e.token)
	if f["smart_search"].Entitled || !f["tailored_documents"].Entitled {
		t.Fatalf("after cancel: smart_search %+v tailored_documents %+v", f["smart_search"], f["tailored_documents"])
	}
}

// An invite is a Stripe promotion code applied at Checkout; an unknown or
// used-up one is refused before Stripe is asked for a session.
func TestBillingInvite(t *testing.T) {
	e := newBillingEnv(t, true)
	if code := e.call(t, e.token, "POST", "/v1/billing/checkout", `{"code":"NOPE"}`, nil); code != 400 {
		t.Fatalf("unknown invite: %d", code)
	}
	if code := e.call(t, e.token, "POST", "/v1/billing/checkout", `{"code":"FRIEND"}`, nil); code != 200 {
		t.Fatalf("invite checkout: %d", code)
	}
	form := e.stripe.checkout.Load().(url.Values)
	if form.Get("discounts[0][promotion_code]") != "promo_1" || form.Get("allow_promotion_codes") != "" || form.Get("payment_method_collection") != "if_required" {
		t.Fatalf("invite form: %v", form)
	}
	e.call(t, e.token, "POST", "/v1/billing/checkout", "", nil)
	if form := e.stripe.checkout.Load().(url.Values); form.Get("allow_promotion_codes") != "true" {
		t.Fatalf("plain checkout should take a code: %v", form)
	}
}

// The tutorial shows off configured features but can't buy anything.
func TestBillingDemo(t *testing.T) {
	e := newBillingEnv(t, true)
	secret, _, err := e.svc.CreateDemoSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if f := e.features(t, secret)["smart_search"]; !f.Entitled {
		t.Fatalf("demo not entitled: %+v", f)
	}
	if code := e.call(t, secret, "POST", "/v1/billing/checkout", "", nil); code != 400 {
		t.Fatalf("demo checkout: %d", code)
	}
}
