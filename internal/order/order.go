package order

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("order not found")

type Status string

const (
	Pending   Status = "PENDING"
	Reserved  Status = "RESERVED"
	Confirmed Status = "CONFIRMED"
	Failed    Status = "FAILED"
	Cancelled Status = "CANCELLED"
)

// Prices are integer minor units (e.g. USD cents), never floating point.
type Item struct {
	ProductID      string `json:"product_id"`
	Quantity       int64  `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}
type CreateRequest struct {
	CustomerID string `json:"customer_id"`
	Items      []Item `json:"items"`
}
type Order struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Status     Status    `json:"status"`
	Items      []Item    `json:"items"`
	TotalCents int64     `json:"total_cents"`
	CreatedAt  time.Time `json:"created_at"`
}

type Repository interface {
	Create(context.Context, Order) error
	Get(context.Context, string) (Order, error)
}
type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// Validate bounds both individual values and the aggregate total.
func (r CreateRequest) Validate() (int64, error) {
	if strings.TrimSpace(r.CustomerID) == "" || len(r.CustomerID) > 128 {
		return 0, errors.New("customer_id must contain 1-128 bytes")
	}
	if len(r.Items) == 0 || len(r.Items) > 100 {
		return 0, errors.New("items must contain 1-100 entries")
	}
	const maxTotal int64 = 1_000_000_000_000
	var total int64
	seen := make(map[string]bool)
	for _, item := range r.Items {
		if strings.TrimSpace(item.ProductID) == "" || len(item.ProductID) > 128 {
			return 0, errors.New("product_id must contain 1-128 bytes")
		}
		if seen[item.ProductID] {
			return 0, errors.New("duplicate product_id")
		}
		seen[item.ProductID] = true
		if item.Quantity < 1 || item.Quantity > 1_000_000 {
			return 0, errors.New("quantity must be between 1 and 1000000")
		}
		if item.UnitPriceCents < 1 || item.UnitPriceCents > 100_000_000 {
			return 0, errors.New("unit_price_cents must be between 1 and 100000000")
		}
		subtotal := item.Quantity * item.UnitPriceCents
		if subtotal > maxTotal-total {
			return 0, errors.New("order total exceeds limit")
		}
		total += subtotal
	}
	return total, nil
}

type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }

func (s *Service) Create(ctx context.Context, req CreateRequest) (Order, error) {
	total, err := req.Validate()
	if err != nil {
		return Order{}, &ValidationError{err}
	}
	o := Order{ID: rand.Text(), CustomerID: req.CustomerID, Status: Pending, Items: append([]Item(nil), req.Items...), TotalCents: total, CreatedAt: time.Now().UTC()}
	if err = s.repo.Create(ctx, o); err != nil {
		return Order{}, fmt.Errorf("create order: %w", err)
	}
	return o, nil
}
func (s *Service) Get(ctx context.Context, id string) (Order, error) { return s.repo.Get(ctx, id) }
