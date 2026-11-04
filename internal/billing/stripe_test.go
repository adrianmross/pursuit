package billing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const body = `{"id":"evt_1","type":"invoice.paid","data":{"object":{"object":"invoice","customer":"cus_1"}}}`

// Computed outside Go: HMAC-SHA256("whsec_test", "1700000000." + body).
const vector = "d09aa73a0451504af97f27a24580ae518851992120c9131cdcca529499dd4bf2"

func TestVerify(t *testing.T) {
	at := time.Unix(1700000000, 0)
	cases := []struct {
		name, header, secret string
		now                  time.Time
		ok                   bool
	}{
		{"documented scheme", "t=1700000000,v1=" + vector, "whsec_test", at, true},
		{"rolled secret: any v1 may match", "t=1700000000,v1=00,v1=" + vector + ",v0=ff", "whsec_test", at, true},
		{"v0 alone is ignored", "t=1700000000,v0=" + vector, "whsec_test", at, false},
		{"wrong secret", "t=1700000000,v1=" + vector, "whsec_other", at, false},
		{"too old", "t=1700000000,v1=" + vector, "whsec_test", at.Add(6 * time.Minute), false},
		{"no timestamp", "v1=" + vector, "whsec_test", at, false},
		{"no secret configured", "t=1700000000,v1=" + vector, "", at, false},
	}
	for _, c := range cases {
		ev, err := Verify([]byte(body), c.header, c.secret, c.now)
		if c.ok != (err == nil) {
			t.Errorf("%s: err = %v", c.name, err)
		}
		if c.ok && (ev.ID != "evt_1" || ev.CustomerID() != "cus_1") {
			t.Errorf("%s: event = %+v", c.name, ev)
		}
		if !c.ok && !errors.Is(err, ErrSignature) {
			t.Errorf("%s: want ErrSignature, got %v", c.name, err)
		}
	}
	if _, err := Verify([]byte(body), Sign([]byte(body), "whsec_test", at), "whsec_test", at); err != nil {
		t.Errorf("Sign does not round-trip: %v", err)
	}
}

func TestClient(t *testing.T) {
	var idem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, _, _ := r.BasicAuth(); u != "sk_test" {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"bad key"}}`))
			return
		}
		switch r.URL.Path {
		case "/v1/customers":
			idem = r.Header.Get("Idempotency-Key")
			_, _ = w.Write([]byte(`{"id":"cus_1"}`))
		case "/v1/subscriptions":
			_, _ = w.Write([]byte(`{"data":[{"id":"sub_1","status":"active","cancel_at_period_end":true,"items":{"data":[{"current_period_end":1700000000,"price":{"id":"price_1"}}]}}]}`))
		case "/v1/entitlements/active_entitlements":
			if r.URL.Query().Get("starting_after") == "" {
				_, _ = w.Write([]byte(`{"data":[{"id":"ent_1","lookup_key":"smart_search"}],"has_more":true}`))
			} else {
				_, _ = w.Write([]byte(`{"data":[{"id":"ent_2","lookup_key":"document_storage"}],"has_more":false}`))
			}
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	c := &Client{Key: "sk_test", Base: srv.URL}
	if id, err := c.CreateCustomer(ctx, "u1", "a@b.c", "A"); err != nil || id != "cus_1" || idem != "customer-u1" {
		t.Fatalf("customer %q %v idem %q", id, err, idem)
	}
	sub, err := c.LatestSubscription(ctx, "cus_1")
	if err != nil || sub.Status != "active" || sub.PriceID != "price_1" || !sub.CancelAtPeriodEnd || sub.CurrentPeriodEnd.Unix() != 1700000000 {
		t.Fatalf("subscription %+v %v", sub, err)
	}
	keys, err := c.ActiveEntitlements(ctx, "cus_1")
	if err != nil || len(keys) != 2 || keys[1] != "document_storage" {
		t.Fatalf("entitlements %v %v", keys, err)
	}
	var se *Error
	if _, err := (&Client{Key: "nope", Base: srv.URL}).CreateCustomer(ctx, "u1", "", ""); !errors.As(err, &se) || se.Status != 401 {
		t.Fatalf("want a 401 Error, got %v", err)
	}
}
