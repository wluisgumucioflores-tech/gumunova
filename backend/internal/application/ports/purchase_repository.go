package ports

import (
	"context"
	"time"

	"gumunova/backend/internal/domain/purchase"
	"gumunova/backend/internal/domain/shared/decimal"
)

// PurchaseFilter narrows a purchase listing. Zero values mean "no filter".
type PurchaseFilter struct {
	SupplierID string
	From       *time.Time // purchase date, inclusive
	To         *time.Time // purchase date, inclusive
	Query      string     // matches the document reference or the supplier name
	Limit      int
	Offset     int
}

// PurchaseSummary is one row of the purchase listing.
type PurchaseSummary struct {
	ID                string
	SupplierID        string
	SupplierName      string
	PurchaseDate      time.Time
	DocumentReference string
	Total             decimal.Decimal
	LineCount         int
	CreatedAt         time.Time
}

type PurchasePage struct {
	Items []PurchaseSummary
	Total int
}

// IdempotencyKey identifies one intended purchase, so a retry cannot register it twice.
type IdempotencyKey struct {
	Key         string
	Fingerprint string // hash of the request, to detect a key reused for something else
}

type RegisterResult struct {
	Purchase purchase.Purchase
	// Replayed is true when the key had already registered this same purchase and the stored
	// one is returned instead of creating another.
	Replayed bool
}

type PurchaseRepository interface {
	// Register persists the purchase, its lines, its physical rolls (assigning their codes),
	// the stock increases and the matching inventory movements in ONE transaction: either all
	// of it is saved or none. It returns purchase.ErrIdempotencyKeyReused when the key belongs
	// to a different purchase.
	Register(ctx context.Context, p purchase.Purchase, key IdempotencyKey) (RegisterResult, error)
	// Update saves the corrected data of purchase id in ONE transaction. When replaceItems is
	// true the items are replaced: the previous lines and rolls are marked superseded, a
	// compensating movement cancels the stock they added, and the new items are written. It
	// returns purchase.ErrItemsLocked, saving nothing, when stock from the purchase was already
	// sold, and purchase.ErrNotFound when the purchase does not exist.
	Update(ctx context.Context, id string, p purchase.Purchase, replaceItems bool) (purchase.Purchase, error)
	// ItemsEditable reports whether the items of the purchase can still be replaced.
	ItemsEditable(ctx context.Context, id string) (bool, error)
	// GetByID returns purchase.ErrNotFound when the purchase does not exist.
	GetByID(ctx context.Context, id string) (purchase.Purchase, error)
	List(ctx context.Context, filter PurchaseFilter) (PurchasePage, error)
}
