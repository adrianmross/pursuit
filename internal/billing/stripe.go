// Package billing is the few Stripe calls a subscription needs, over plain
// HTTP: create a customer, open Checkout and the customer portal, and read
// back the customer's subscriptions and active entitlements.
//
// ponytail: hand-rolled instead of stripe-go for five form posts and an
// HMAC; switch to stripe-go once more of the API is needed.
package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// APIVersion pins response shapes instead of following the account's
// default. Since 2025-03-31.basil the billing period is on subscription
// items, which is where LatestSubscription reads it.
const APIVersion = "2026-10-28.endive"

// Client talks to the Stripe API with a secret key.
type Client struct {
	Key  string
	HTTP *http.Client
	// Base is the API root; tests point it at an httptest server.
	Base string
}

func (c *Client) base() string {
	if c.Base != "" {
		return c.Base
	}
	return "https://api.stripe.com"
}

// Error is a Stripe API error.
type Error struct {
	Status  int
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return fmt.Sprintf("stripe %d %s: %s", e.Status, e.Type, e.Message) }

func (c *Client) do(ctx context.Context, method, path string, form url.Values, idem string, out any) error {
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader(form.Encode())
	} else if len(form) > 0 {
		path += "?" + form.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base()+path, body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Key, "")
	req.Header.Set("Stripe-Version", APIVersion)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		var e struct {
			Error Error `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		e.Error.Status = res.StatusCode
		return &e.Error
	}
	return json.Unmarshal(raw, out)
}

// CreateCustomer makes the customer for one user. The idempotency key is
// the user, so a double click never mints two customers.
func (c *Client) CreateCustomer(ctx context.Context, userID, email, name string) (string, error) {
	form := url.Values{"metadata[user_id]": {userID}}
	if email != "" {
		form.Set("email", email)
	}
	if name != "" {
		form.Set("name", name)
	}
	var out struct {
		ID string `json:"id"`
	}
	err := c.do(ctx, http.MethodPost, "/v1/customers", form, "customer-"+userID, &out)
	return out.ID, err
}

// CheckoutURL starts a subscription Checkout for one price. promo is a
// promotion code ID applied up front; without one the page takes a code.
// A card is asked for only when something is due, so a 100%-off invite
// needs none.
func (c *Client) CheckoutURL(ctx context.Context, customer, price, promo, successURL, cancelURL string) (string, error) {
	form := url.Values{
		"mode":                      {"subscription"},
		"customer":                  {customer},
		"line_items[0][price]":      {price},
		"line_items[0][quantity]":   {"1"},
		"payment_method_collection": {"if_required"},
		"success_url":               {successURL},
		"cancel_url":                {cancelURL},
	}
	if promo != "" {
		form.Set("discounts[0][promotion_code]", promo)
	} else {
		form.Set("allow_promotion_codes", "true")
	}
	var out struct {
		URL string `json:"url"`
	}
	err := c.do(ctx, http.MethodPost, "/v1/checkout/sessions", form, "", &out)
	return out.URL, err
}

// PromotionCode returns the ID of the active promotion code a customer
// types, or "" when there's none (unknown, expired, or used up).
func (c *Client) PromotionCode(ctx context.Context, code string) (string, error) {
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/promotion_codes", url.Values{"code": {code}, "active": {"true"}, "limit": {"1"}}, "", &out); err != nil {
		return "", err
	}
	if len(out.Data) == 0 {
		return "", nil
	}
	return out.Data[0].ID, nil
}

// PortalURL opens the customer portal, where the plan is changed or cancelled.
func (c *Client) PortalURL(ctx context.Context, customer, returnURL string) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	err := c.do(ctx, http.MethodPost, "/v1/billing_portal/sessions", url.Values{"customer": {customer}, "return_url": {returnURL}}, "", &out)
	return out.URL, err
}

// Subscription is the part of a Stripe subscription the app keeps.
type Subscription struct {
	ID                string
	Status            string
	PriceID           string
	CurrentPeriodEnd  time.Time
	CancelAtPeriodEnd bool
}

// LatestSubscription is the customer's newest subscription of any status, or
// nil. One per customer is assumed: turn on "limit customers to one
// subscription" in the Stripe dashboard.
func (c *Client) LatestSubscription(ctx context.Context, customer string) (*Subscription, error) {
	var out struct {
		Data []struct {
			ID                string `json:"id"`
			Status            string `json:"status"`
			CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
			Items             struct {
				Data []struct {
					CurrentPeriodEnd int64 `json:"current_period_end"`
					Price            struct {
						ID string `json:"id"`
					} `json:"price"`
				} `json:"data"`
			} `json:"items"`
		} `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, "/v1/subscriptions", url.Values{"customer": {customer}, "status": {"all"}, "limit": {"1"}}, "", &out)
	if err != nil || len(out.Data) == 0 {
		return nil, err
	}
	d := out.Data[0]
	sub := &Subscription{ID: d.ID, Status: d.Status, CancelAtPeriodEnd: d.CancelAtPeriodEnd}
	if len(d.Items.Data) > 0 {
		sub.PriceID = d.Items.Data[0].Price.ID
		if t := d.Items.Data[0].CurrentPeriodEnd; t > 0 {
			sub.CurrentPeriodEnd = time.Unix(t, 0).UTC()
		}
	}
	return sub, nil
}

// ActiveEntitlements is the lookup_key of every feature the customer has now.
func (c *Client) ActiveEntitlements(ctx context.Context, customer string) ([]string, error) {
	keys := []string{}
	form := url.Values{"customer": {customer}, "limit": {"100"}}
	for {
		var out struct {
			Data []struct {
				ID        string `json:"id"`
				LookupKey string `json:"lookup_key"`
			} `json:"data"`
			HasMore bool `json:"has_more"`
		}
		if err := c.do(ctx, http.MethodGet, "/v1/entitlements/active_entitlements", form, "", &out); err != nil {
			return nil, err
		}
		for _, d := range out.Data {
			keys = append(keys, d.LookupKey)
		}
		if !out.HasMore || len(out.Data) == 0 {
			return keys, nil
		}
		form.Set("starting_after", out.Data[len(out.Data)-1].ID)
	}
}

// Event is the envelope of a webhook delivery: enough to dedupe it and find
// whose state to re-read. The payload itself is never trusted beyond that.
type Event struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object struct {
			Object   string `json:"object"`
			ID       string `json:"id"`
			Customer string `json:"customer"`
		} `json:"object"`
	} `json:"data"`
}

// CustomerID is the customer the event is about.
func (e Event) CustomerID() string {
	if e.Data.Object.Object == "customer" {
		return e.Data.Object.ID
	}
	return e.Data.Object.Customer
}

// Tolerance is how old a signed delivery may be, the libraries' default.
const Tolerance = 5 * time.Minute

var ErrSignature = errors.New("stripe signature does not match")

// Verify checks a Stripe-Signature header against the raw body and returns
// the event. Per docs.stripe.com/webhooks#verify-manually: the signed
// payload is "<t>.<body>", HMAC-SHA256 keyed by the endpoint secret, compared
// in constant time to every v1 entry (several while a secret is rolled);
// other schemes are ignored.
func Verify(body []byte, header, secret string, now time.Time) (Event, error) {
	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch k {
		case "t":
			ts = v
		case "v1":
			sigs = append(sigs, v)
		}
	}
	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || len(sigs) == 0 || secret == "" {
		return Event{}, ErrSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	want := mac.Sum(nil)
	ok := false
	for _, s := range sigs {
		got, err := hex.DecodeString(s)
		if err == nil && hmac.Equal(got, want) {
			ok = true
		}
	}
	if !ok {
		return Event{}, ErrSignature
	}
	if d := now.Sub(time.Unix(t, 0)); d > Tolerance || d < -Tolerance {
		return Event{}, fmt.Errorf("stripe signature is %s old: %w", d.Round(time.Second), ErrSignature)
	}
	var ev Event
	if err := json.Unmarshal(body, &ev); err != nil {
		return Event{}, err
	}
	return ev, nil
}

// Sign is the header Stripe would send, for tests and local replays.
func Sign(body []byte, secret string, t time.Time) string {
	ts := strconv.FormatInt(t.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	return "t=" + ts + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}
