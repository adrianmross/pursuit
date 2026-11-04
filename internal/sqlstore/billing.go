package sqlstore

import (
	"context"
	"database/sql"

	"github.com/pursuit/pursuit/internal/domain"
)

// BillingAccount is a user's Stripe customer and subscription, as last synced.
type BillingAccount struct {
	UserID            string
	CustomerID        string
	SubscriptionID    string
	Status            string
	PriceID           string
	CurrentPeriodEnd  string
	CancelAtPeriodEnd bool
	SyncedAt          string
}

const billingCols = `user_id, stripe_customer_id, subscription_id, status, price_id, current_period_end, cancel_at_period_end, synced_at`

func (s *Store) oneBilling(ctx context.Context, where string, arg string) (BillingAccount, error) {
	var b BillingAccount
	var cancel int
	err := s.queryRow(ctx, `SELECT `+billingCols+` FROM billing_accounts WHERE `+where, arg).
		Scan(&b.UserID, &b.CustomerID, &b.SubscriptionID, &b.Status, &b.PriceID, &b.CurrentPeriodEnd, &cancel, &b.SyncedAt)
	if err == sql.ErrNoRows {
		return BillingAccount{}, domain.NotFound("billing account")
	}
	b.CancelAtPeriodEnd = cancel != 0
	return b, err
}

func (s *Store) GetBilling(ctx context.Context, userID string) (BillingAccount, error) {
	return s.oneBilling(ctx, `user_id=?`, userID)
}

func (s *Store) GetBillingByCustomer(ctx context.Context, customerID string) (BillingAccount, error) {
	return s.oneBilling(ctx, `stripe_customer_id=?`, customerID)
}

// PutBilling writes the whole row.
func (s *Store) PutBilling(ctx context.Context, b BillingAccount) error {
	cancel := 0
	if b.CancelAtPeriodEnd {
		cancel = 1
	}
	_, err := s.exec(ctx, `INSERT INTO billing_accounts (`+billingCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET stripe_customer_id=excluded.stripe_customer_id, subscription_id=excluded.subscription_id,
		status=excluded.status, price_id=excluded.price_id, current_period_end=excluded.current_period_end,
		cancel_at_period_end=excluded.cancel_at_period_end, synced_at=excluded.synced_at`,
		b.UserID, b.CustomerID, b.SubscriptionID, b.Status, b.PriceID, b.CurrentPeriodEnd, cancel, b.SyncedAt)
	return err
}

// Entitlement is one feature a user may use, and where it came from.
type Entitlement struct {
	Feature   string `json:"feature"`
	Source    string `json:"source"`
	GrantedAt string `json:"granted_at"`
}

func (s *Store) ListEntitlements(ctx context.Context, userID string) ([]Entitlement, error) {
	rows, err := s.query(ctx, `SELECT feature, source, granted_at FROM entitlements WHERE user_id=? ORDER BY feature, source`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entitlement{}
	for rows.Next() {
		var e Entitlement
		if err := rows.Scan(&e.Feature, &e.Source, &e.GrantedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ReplaceEntitlements makes the user's rows of one source exactly features.
// Rows of other sources are left alone. Call inside WithTx.
func (s *Store) ReplaceEntitlements(ctx context.Context, userID, source string, features []string, at string) error {
	if _, err := s.exec(ctx, `DELETE FROM entitlements WHERE user_id=? AND source=?`, userID, source); err != nil {
		return err
	}
	for _, f := range features {
		if _, err := s.exec(ctx, `INSERT INTO entitlements (user_id, feature, source, granted_at) VALUES (?, ?, ?, ?)
			ON CONFLICT(user_id, feature, source) DO NOTHING`, userID, f, source, at); err != nil {
			return err
		}
	}
	return nil
}

// UserFeatures is the user's own on/off per feature; absent means default.
func (s *Store) UserFeatures(ctx context.Context, userID string) (map[string]bool, error) {
	rows, err := s.query(ctx, `SELECT feature, enabled FROM user_features WHERE user_id=?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var f string
		var on int
		if err := rows.Scan(&f, &on); err != nil {
			return nil, err
		}
		out[f] = on != 0
	}
	return out, rows.Err()
}

func (s *Store) SetUserFeature(ctx context.Context, userID, feature string, enabled bool, at string) error {
	on := 0
	if enabled {
		on = 1
	}
	_, err := s.exec(ctx, `INSERT INTO user_features (user_id, feature, enabled, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, feature) DO UPDATE SET enabled=excluded.enabled, updated_at=excluded.updated_at`, userID, feature, on, at)
	return err
}

// MarkStripeEvent records a delivery and reports whether it is new.
func (s *Store) MarkStripeEvent(ctx context.Context, id, typ, at string) (bool, error) {
	res, err := s.exec(ctx, `INSERT INTO stripe_events (id, type, received_at) VALUES (?, ?, ?) ON CONFLICT(id) DO NOTHING`, id, typ, at)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// ForgetStripeEvent undoes MarkStripeEvent when handling failed, so the
// redelivery is processed instead of skipped.
func (s *Store) ForgetStripeEvent(ctx context.Context, id string) error {
	_, err := s.exec(ctx, `DELETE FROM stripe_events WHERE id=?`, id)
	return err
}
