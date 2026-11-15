package sale

import (
	"math/big"

	"gumunova/backend/internal/domain/shared/decimal"
)

// All amounts are hundredths. A product of two amounts, or a share of one, is rounded half up to
// the hundredth exactly once, with integer arithmetic: no floating point is ever involved.

// roundedRatio returns round-half-up(numerator / denominator) for a non-negative numerator and a
// positive denominator.
func roundedRatio(numerator, denominator *big.Int) *big.Int {
	twice := new(big.Int).Mul(numerator, big.NewInt(2))
	twice.Add(twice, denominator)
	return twice.Div(twice, new(big.Int).Mul(denominator, big.NewInt(2)))
}

func fromBig(value *big.Int) (decimal.Decimal, error) {
	if !value.IsInt64() {
		return decimal.Decimal{}, decimal.ErrOverflow
	}
	return decimal.FromHundredths(value.Int64()), nil
}

// LineTotal is unit price × quantity (meters, rolls or units), rounded to the hundredth.
func LineTotal(unitPrice, quantity decimal.Decimal) (decimal.Decimal, error) {
	product := new(big.Int).Mul(big.NewInt(unitPrice.Hundredths()), big.NewInt(quantity.Hundredths()))
	return fromBig(roundedRatio(product, big.NewInt(100)))
}

// RollCostSoldToDate is the cost of the first soldM meters of a roll: its attributable cost
// shared in proportion to the meters sold. Taking the difference of two such values gives the
// cost of one sale, so the costs of successive sales add up exactly to the roll's cost, and
// the sale that empties the roll takes whatever is left, to the hundredth.
func RollCostSoldToDate(attributable, initial, soldM decimal.Decimal) (decimal.Decimal, error) {
	product := new(big.Int).Mul(big.NewInt(attributable.Hundredths()), big.NewInt(soldM.Hundredths()))
	return fromBig(roundedRatio(product, big.NewInt(initial.Hundredths())))
}

// TableclothCost is the weighted-average cost of the units sold: the value of the stock shared
// in proportion to the units. When every unit is sold it is exactly the value of the stock.
func TableclothCost(stockValue decimal.Decimal, stockUnits, units int) (decimal.Decimal, error) {
	product := new(big.Int).Mul(big.NewInt(stockValue.Hundredths()), big.NewInt(int64(units)))
	return fromBig(roundedRatio(product, big.NewInt(int64(stockUnits))))
}
