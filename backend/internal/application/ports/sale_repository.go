package ports

import (
	"context"
	"time"

	"gumunova/backend/internal/domain/sale"
	"gumunova/backend/internal/domain/shared/decimal"
)

// SaleFilter narrows a sale listing. Zero values mean "no filter".
type SaleFilter struct {
	From   *time.Time // sale date, inclusive
	To     *time.Time // sale date, inclusive
	Query  string     // matches the SKU or name of a sold product, or the code of a sold roll
	Limit  int
	Offset int
}

// SaleSummary is one row of the sale listing.
type SaleSummary struct {
	ID        string
	SaleDate  time.Time
	Total     decimal.Decimal
	TotalCost decimal.Decimal
	LineCount int
	CreatedAt time.Time
}

// GrossProfit is what was charged minus what the sold stock cost.
func (s SaleSummary) GrossProfit() decimal.Decimal {
	return decimal.FromHundredths(s.Total.Hundredths() - s.TotalCost.Hundredths())
}

type SalePage struct {
	Items []SaleSummary
	Total int
}

type SaleRegisterResult struct {
	Sale sale.Sale
	// Replayed is true when the key had already registered this same sale and the stored one is
	// returned instead of creating another.
	Replayed bool
}

// SaleRepository stores confirmed sales.
type SaleRepository interface {
	// Register confirms the sale in one transaction: it locks the rolls and tablecloth stock it
	// takes from, checks and prices every line with sale.Sale.Allocate, reduces the stock and
	// writes the sale, its lines and one movement per line. If anything fails, nothing is saved.
	// Stock or reference problems come back as a *validation.ValidationError. Repeating a key
	// with the same fingerprint returns the stored sale; with a different one it returns
	// sale.ErrIdempotencyKeyReused.
	Register(ctx context.Context, s sale.Sale, key IdempotencyKey) (SaleRegisterResult, error)
	// GetByID returns sale.ErrNotFound when the sale does not exist.
	GetByID(ctx context.Context, id string) (sale.Sale, error)
	List(ctx context.Context, filter SaleFilter) (SalePage, error)
}
