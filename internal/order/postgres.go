package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

// A single INSERT atomically stores the order and all items in one JSONB value.
func (p *Postgres) Create(ctx context.Context, o Order) error {
	items, err := json.Marshal(o.Items)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `INSERT INTO orders.orders (id,customer_id,status,items,total_cents,created_at) VALUES ($1,$2,$3,$4,$5,$6)`, o.ID, o.CustomerID, o.Status, items, o.TotalCents, o.CreatedAt)
	return err
}
func (p *Postgres) Get(ctx context.Context, id string) (Order, error) {
	var o Order
	var items []byte
	err := p.pool.QueryRow(ctx, `SELECT id,customer_id,status,items,total_cents,created_at FROM orders.orders WHERE id=$1`, id).Scan(&o.ID, &o.CustomerID, &o.Status, &items, &o.TotalCents, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	if err = json.Unmarshal(items, &o.Items); err != nil {
		return Order{}, fmt.Errorf("decode stored items: %w", err)
	}
	return o, nil
}
