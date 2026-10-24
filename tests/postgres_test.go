package tests

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"orderflow/internal/order"
	"orderflow/migrations"
)

// Use a dedicated disposable database. This test applies the Phase 1 migration.
func TestPostgresPersistence(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is unset; PostgreSQL integration test requires a dedicated database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err = pool.Exec(ctx, migrations.Initial); err != nil {
		t.Fatal(err)
	}
	// Applying the bootstrap a second time must preserve data and succeed.
	if _, err = pool.Exec(ctx, migrations.Initial); err != nil {
		t.Fatal(err)
	}
	s := order.NewService(order.NewPostgres(pool))
	o, err := s.Create(ctx, order.CreateRequest{CustomerID: "integration", Items: []order.Item{{ProductID: "book", Quantity: 3, UnitPriceCents: 99}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanup, "DELETE FROM orders.orders WHERE id=$1", o.ID); err != nil {
			t.Error(err)
		}
	})
	// Read through a separate pool, so this verifies committed database state.
	other, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	got, err := order.NewPostgres(other).Get(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != o.ID || got.CustomerID != o.CustomerID || got.Status != order.Pending || got.TotalCents != 297 || !reflect.DeepEqual(got.Items, o.Items) || got.CreatedAt.Sub(o.CreatedAt).Abs() > time.Microsecond {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if err := order.NewPostgres(pool).Create(ctx, o); err == nil {
		t.Fatal("duplicate primary key accepted")
	}
	if _, err := order.NewPostgres(pool).Get(ctx, "does-not-exist"); err != order.ErrNotFound {
		t.Fatalf("missing order: %v", err)
	}
}
