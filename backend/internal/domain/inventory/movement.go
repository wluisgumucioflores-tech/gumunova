// Package inventory names the kinds of stock movements recorded in the auditable ledger.
// Stock never changes without a movement that explains it.
package inventory

import "errors"

// ErrRollNotFound means no physical roll has that id.
var ErrRollNotFound = errors.New("roll not found")

type MovementType string

const (
	// MovementPurchaseIn is stock received through a confirmed purchase.
	MovementPurchaseIn MovementType = "purchase_in"
	// MovementPurchaseReversal cancels the stock added by a purchase whose items were replaced.
	MovementPurchaseReversal MovementType = "purchase_reversal"
	// MovementSaleOut is stock sold through a confirmed sale.
	MovementSaleOut MovementType = "sale_out"
)

// IsKnown reports whether the type is one the ledger records.
func (t MovementType) IsKnown() bool {
	return t == MovementPurchaseIn || t == MovementPurchaseReversal || t == MovementSaleOut
}

type Unit string

const (
	UnitMeter Unit = "m"
	UnitPiece Unit = "unit"
)

// Reference kinds link a movement to the operation that caused it.
const (
	ReferencePurchase = "purchase"
	ReferenceSale     = "sale"
)
