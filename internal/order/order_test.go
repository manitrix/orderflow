package order

import (
	"context"
	"errors"
	"testing"
)

type memoryRepo struct {
	orders map[string]Order
	err    error
}

func (m *memoryRepo) Create(_ context.Context, o Order) error {
	if m.err != nil {
		return m.err
	}
	m.orders[o.ID] = o
	return nil
}
func (m *memoryRepo) Get(_ context.Context, id string) (Order, error) {
	if m.err != nil {
		return Order{}, m.err
	}
	o, ok := m.orders[id]
	if !ok {
		return Order{}, ErrNotFound
	}
	return o, nil
}
func validRequest() CreateRequest {
	return CreateRequest{CustomerID: "customer-1", Items: []Item{{ProductID: "book", Quantity: 2, UnitPriceCents: 1500}}}
}

func TestCreate(t *testing.T) {
	repo := &memoryRepo{orders: make(map[string]Order)}
	s := NewService(repo)
	o, err := s.Create(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if o.ID == "" || o.Status != Pending || o.TotalCents != 3000 || o.CreatedAt.IsZero() {
		t.Fatalf("invalid order: %+v", o)
	}
	got, err := s.Get(context.Background(), o.ID)
	if err != nil || got.ID != o.ID {
		t.Fatalf("get: %+v %v", got, err)
	}
}
func TestValidation(t *testing.T) {
	tests := map[string]func(*CreateRequest){
		"empty customer":    func(r *CreateRequest) { r.CustomerID = " " },
		"no items":          func(r *CreateRequest) { r.Items = nil },
		"empty product":     func(r *CreateRequest) { r.Items[0].ProductID = "" },
		"zero quantity":     func(r *CreateRequest) { r.Items[0].Quantity = 0 },
		"negative price":    func(r *CreateRequest) { r.Items[0].UnitPriceCents = -1 },
		"huge quantity":     func(r *CreateRequest) { r.Items[0].Quantity = 1 << 62 },
		"huge price":        func(r *CreateRequest) { r.Items[0].UnitPriceCents = 1 << 62 },
		"total limit":       func(r *CreateRequest) { r.Items[0].Quantity = 1_000_000; r.Items[0].UnitPriceCents = 100_000_000 },
		"duplicate product": func(r *CreateRequest) { r.Items = append(r.Items, r.Items[0]) },
		"too many items":    func(r *CreateRequest) { r.Items = make([]Item, 101) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			r := validRequest()
			mutate(&r)
			repo := &memoryRepo{orders: make(map[string]Order)}
			_, err := NewService(repo).Create(context.Background(), r)
			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("expected validation error, got %v", err)
			}
			if len(repo.orders) != 0 {
				t.Fatal("invalid order persisted")
			}
		})
	}
}
func TestRepositoryFailure(t *testing.T) {
	failure := errors.New("database down")
	s := NewService(&memoryRepo{err: failure})
	if _, err := s.Create(context.Background(), validRequest()); !errors.Is(err, failure) {
		t.Fatalf("got %v", err)
	}
}
