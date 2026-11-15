// Package ports declares the interfaces the application layer needs from the outside world.
package ports

import (
	"context"

	"gumunova/backend/internal/domain/product"
)

// ProductFilter narrows a product listing. Zero values mean "no filter".
type ProductFilter struct {
	Type     product.Type
	IsActive *bool
	Query    string // matches SKU or name, case-insensitive
	Limit    int
	Offset   int
}

type ProductPage struct {
	Items []product.Product
	Total int
}

type ProductRepository interface {
	// Create persists a new product and returns it with its generated ID and timestamps.
	// It returns product.ErrSKUAlreadyExists when the SKU is taken.
	Create(ctx context.Context, p product.Product) (product.Product, error)
	// Update persists the editable fields and the active flag.
	// It returns product.ErrNotFound when the product does not exist.
	Update(ctx context.Context, p product.Product) (product.Product, error)
	// SetImageKey stores the object key of the product image; an empty key removes it.
	SetImageKey(ctx context.Context, id, imageKey string) error
	// ResetLabelCode replaces the product's label code with a new unique one, so labels printed
	// with the previous code stop identifying it. It returns product.ErrNotFound when the product does not exist.
	ResetLabelCode(ctx context.Context, id string) (product.Product, error)
	// GetByLabelCode finds the product whose label carries the code (product.ErrNotFound if none).
	GetByLabelCode(ctx context.Context, code string) (product.Product, error)
	// GetByID returns product.ErrNotFound when the product does not exist.
	GetByID(ctx context.Context, id string) (product.Product, error)
	List(ctx context.Context, filter ProductFilter) (ProductPage, error)
}
