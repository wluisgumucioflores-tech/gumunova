package ports

import (
	"context"

	"gumunova/backend/internal/domain/supplier"
)

// SupplierFilter narrows a supplier listing. Zero values mean "no filter".
type SupplierFilter struct {
	IsActive *bool
	Query    string // matches name, phone or email, case-insensitive
	Limit    int
	Offset   int
}

type SupplierPage struct {
	Items []supplier.Supplier
	Total int
}

type SupplierRepository interface {
	// Create persists a new supplier and returns it with its generated ID and timestamps.
	Create(ctx context.Context, s supplier.Supplier) (supplier.Supplier, error)
	// Update persists the editable fields and the active flag.
	// It returns supplier.ErrNotFound when the supplier does not exist.
	Update(ctx context.Context, s supplier.Supplier) (supplier.Supplier, error)
	// GetByID returns supplier.ErrNotFound when the supplier does not exist.
	GetByID(ctx context.Context, id string) (supplier.Supplier, error)
	List(ctx context.Context, filter SupplierFilter) (SupplierPage, error)
}
