package service

import (
	"context"
	"errors"
	"testing"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/sqlstore"
	"github.com/pursuit/pursuit/internal/sqlstore/sqlstoretest"
)

// A new user runs OnUserCreated through appkit's account layer, and a hook
// that fails leaves no user behind.
func TestOnUserCreated(t *testing.T) {
	ctx := context.Background()
	db, err := sqlstore.Open(ctx, sqlstoretest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db)

	var created []string
	s.OnUserCreated = func(_ context.Context, u domain.User) error {
		created = append(created, u.Email)
		return nil
	}
	create := func(email string) error {
		a := s.Accounts()
		return a.DB.WithLock(ctx, func(ctx context.Context) error {
			_, err := a.CreateUser(ctx, "", email)
			return err
		})
	}
	if err := create("a@example.com"); err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 || created[0] != "a@example.com" {
		t.Fatalf("hook ran for %v, want once for a@example.com", created)
	}

	boom := errors.New("boom")
	s.OnUserCreated = func(context.Context, domain.User) error { return boom }
	if err := create("b@example.com"); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the hook's error", err)
	}
	if _, err := s.Accounts().Store().FindUserByEmail(ctx, "b@example.com"); !domain.IsCode(err, "not_found") {
		t.Fatalf("user survived a failed hook: %v", err)
	}
}
