package sqlstore

import (
	"bytes"
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/sqlstore/sqlstoretest"
)

func TestMigrateTwiceAndForeignKey(t *testing.T) {
	dsn := sqlstoretest.DSN(t)
	s, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	defer s2.Close()
	ctx := context.Background()
	if err := s.CreateUser(ctx, domain.User{ID: "u", Name: "A", Email: "a@localhost", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	err = s.CreateRole(ctx, domain.Role{
		ID: "r", UserID: "u", CompanyID: "missing", Title: "t", Version: 1,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})
	if err == nil || !domain.IsCode(err, "conflict") {
		t.Fatalf("fk err=%v", err)
	}
	var wg sync.WaitGroup
	errCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- Migrate(ctx, s.db, s.dialect)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestClaimConcurrencyAndExpiry(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.CreateUser(ctx, domain.User{ID: "u", Name: "A", Email: "a@localhost", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateCompany(ctx, domain.Company{ID: "c", UserID: "u", Name: "Acme", Version: 1, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRole(ctx, domain.Role{
		ID: "role", UserID: "u", CompanyID: "c", Title: "Engineer", JobType: "Full-time", Version: 1,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	stamp := domain.FormatTime(now)
	for _, id := range []string{"a1", "a2"} {
		if err := s.CreateApplication(ctx, domain.Application{
			ID: id, UserID: "u", RoleID: "role", Status: domain.StatusToApply, Version: 1,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.EnqueueWork(ctx, WorkItem{
			ID: "w-" + id, UserID: "u", ApplicationID: id, Kind: "prepare", CreatedAt: stamp, UpdatedAt: stamp,
		}); err != nil {
			t.Fatal(err)
		}
	}

	var mu sync.Mutex
	got := map[string]string{}
	var wg sync.WaitGroup
	errCh := make(chan error, 8)
	until := domain.FormatTime(now.Add(2 * time.Minute))
	for i := 0; i < 8; i++ {
		wg.Add(1)
		worker := "worker-" + string(rune('a'+i))
		go func(worker string) {
			defer wg.Done()
			err := s.WithTx(ctx, func(ctx context.Context) error {
				item, ok, err := s.ClaimWork(ctx, "u", worker, stamp, until)
				if err != nil || !ok {
					return err
				}
				mu.Lock()
				got[item.ID] = worker
				mu.Unlock()
				return nil
			})
			errCh <- err
		}(worker)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(got) != 2 {
		t.Fatalf("leased %d jobs, want 2 distinct: %#v", len(got), got)
	}

	past := domain.FormatTime(now.Add(-time.Minute))
	future := domain.FormatTime(now.Add(time.Hour))
	var expiredID string
	for id := range got {
		expiredID = id
		break
	}
	if _, err := s.exec(ctx, `UPDATE work_items SET lease_until=? WHERE id=?`, past, expiredID); err != nil {
		t.Fatal(err)
	}
	again, ok, err := s.ClaimWork(ctx, "u", "reclaimer", domain.FormatTime(now.Add(2*time.Second)), future)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || again.ID != expiredID || again.LeaseOwner != "reclaimer" {
		t.Fatalf("expiry did not return the job: ok=%v item=%+v", ok, again)
	}
}

func TestLeaderLeaseExclusive(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	now := time.Now().UTC()
	first, ok, err := s.AcquireLeader(ctx, "link-sweep", "replica-a", now, now.Add(time.Minute))
	if err != nil || !ok {
		t.Fatalf("first leader ok=%v err=%v", ok, err)
	}
	_, ok, err = s.AcquireLeader(ctx, "link-sweep", "replica-b", now, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("second replica acquired the sweep lock")
	}
	first.Release()
	_, ok, err = s.AcquireLeader(ctx, "link-sweep", "replica-b", now.Add(time.Second), now.Add(time.Minute))
	if err != nil || !ok {
		t.Fatalf("after release ok=%v err=%v", ok, err)
	}
}

func TestSkipLockedClausePresent(t *testing.T) {
	b, err := os.ReadFile("pipeline.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("FOR UPDATE SKIP LOCKED")) {
		t.Fatal("postgres queue claim must use FOR UPDATE SKIP LOCKED")
	}
}

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), sqlstoretest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
