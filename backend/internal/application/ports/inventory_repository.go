package ports

import (
	"context"
	"time"

	"gumunova/backend/internal/domain/inventory"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/shared/decimal"
)

// StockFilter narrows the stock summary. Zero values mean "no filter".
type StockFilter struct {
	Type    product.Type
	OnlyLow bool   // only variants at or below their minimum stock
	Query   string // matches SKU or name
	Limit   int
	Offset  int
}

// StockItem is the stock of one variant. PVC is counted in meters across its rolls and
// tablecloths in units. Values marked "display" are rounded for reading and are never used
// to calculate anything else.
type StockItem struct {
	ProductID string
	SKU       string
	Name      string
	Type      product.Type
	IsActive  bool
	// Quantity is meters (PVC) or units (tablecloths) available.
	Quantity decimal.Decimal
	// Minimum is the configured minimum stock, in the same unit. Zero means no alert.
	Minimum decimal.Decimal
	IsLow   bool
	// RollCount is the number of PVC rolls with meters left (zero for tablecloths).
	RollCount int
	// StockValue is the cost of what is left (display).
	StockValue decimal.Decimal
	// AverageCost is the weighted average cost per unit of tablecloths, four decimals (display).
	// Empty for PVC, where each roll has its own cost per meter, and when nothing is left.
	AverageCost string
}

type StockPage struct {
	Items []StockItem
	Total int
}

// RollFilter narrows the roll listing. Zero values mean "no filter".
type RollFilter struct {
	ProductID string
	// Status is "available" (meters left; the default), "depleted" or "all".
	Status string
	Query  string // matches the roll code, SKU or name
	Limit  int
	Offset int
}

const (
	RollStatusAvailable = "available"
	RollStatusDepleted  = "depleted"
	RollStatusAll       = "all"
)

// RollItem is one physical roll with its own cost, length and balance.
type RollItem struct {
	ID   string
	Code string
	// LabelCode is the unique code printed on the roll's label (QR or barcode).
	LabelCode   string
	ProductID   string
	SKU         string
	Name        string
	PurchaseID  string
	ReceivedAt  time.Time
	InitialM    decimal.Decimal
	AvailableM  decimal.Decimal
	Acquisition decimal.Decimal
	Additional  decimal.Decimal
	// Attributable is the acquisition cost plus the share of additional costs.
	Attributable decimal.Decimal
	// CostPerMeter is attributable cost over the initial usable length, four decimals (display).
	CostPerMeter string
	// RemainingValue is the cost of the meters left (display).
	RemainingValue decimal.Decimal
}

// IsIntact is true while the roll has had no cut or other outflow.
func (r RollItem) IsIntact() bool { return r.AvailableM.Hundredths() == r.InitialM.Hundredths() }

type RollPage struct {
	Items []RollItem
	Total int
}

// MovementFilter narrows the movement ledger. Zero values mean "no filter".
type MovementFilter struct {
	ProductID string
	RollID    string
	Type      inventory.MovementType
	Limit     int
	Offset    int
}

// MovementItem is one entry of the auditable ledger.
type MovementItem struct {
	ID            string
	CreatedAt     time.Time
	ProductID     string
	SKU           string
	Name          string
	RollID        string
	RollCode      string
	Type          inventory.MovementType
	QuantityDelta decimal.Decimal
	Unit          inventory.Unit
	ReferenceType string
	ReferenceID   string
	Reason        string
}

type MovementPage struct {
	Items []MovementItem
	Total int
}

// InventoryRepository answers the read-only inventory queries. Stock only changes through
// operations (purchases today) that also write their movement; nothing here changes stock.
type InventoryRepository interface {
	ListStock(ctx context.Context, filter StockFilter) (StockPage, error)
	ListRolls(ctx context.Context, filter RollFilter) (RollPage, error)
	// GetRoll returns inventory.ErrRollNotFound when the roll does not exist.
	GetRoll(ctx context.Context, id string) (RollItem, error)
	// GetRollByLabelCode finds the roll whose label carries the code. A roll replaced by a purchase
	// edit no longer counts. It returns inventory.ErrRollNotFound when there is none.
	GetRollByLabelCode(ctx context.Context, code string) (RollItem, error)
	// ResetRollLabelCode replaces the roll's label code with a new unique one, so labels printed
	// with the previous code stop identifying it. It changes no stock and writes no movement.
	// It returns inventory.ErrRollNotFound when the roll does not exist.
	ResetRollLabelCode(ctx context.Context, id string) (RollItem, error)
	ListMovements(ctx context.Context, filter MovementFilter) (MovementPage, error)
}
