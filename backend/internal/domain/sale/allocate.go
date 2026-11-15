package sale

import (
	"fmt"

	"gumunova/backend/internal/domain/shared/decimal"
	"gumunova/backend/internal/domain/shared/validation"
)

// RollStock is a physical roll as it stands while the sale is being confirmed.
type RollStock struct {
	ID            string
	Code          string
	ProductID     string
	SKU           string
	Name          string
	ProductActive bool
	// InitialM is the usable length received; AvailableM what is left of it.
	InitialM     decimal.Decimal
	AvailableM   decimal.Decimal
	Attributable decimal.Decimal // acquisition cost plus the share of additional costs
	// Catalog reference prices at this moment, kept on the line as a snapshot.
	ReferencePricePerMeter decimal.Decimal
	ReferencePricePerRoll  decimal.Decimal
}

// TableclothStock is the stock of one tablecloth variant while the sale is being confirmed.
type TableclothStock struct {
	ProductID      string
	SKU            string
	Name           string
	ProductActive  bool
	Units          int
	Value          decimal.Decimal // cost of the units in stock; the average is Value / Units
	ReferencePrice decimal.Decimal
}

// Inventory is the stock a sale takes from, read under lock by persistence. Allocate changes the
// balances here, so lines of the same sale see what earlier lines already took.
type Inventory struct {
	Rolls       map[string]*RollStock
	Tablecloths map[string]*TableclothStock
	// NotTablecloth holds ids of products that exist but are not tablecloths.
	NotTablecloth map[string]bool
}

// Allocate checks every line against the inventory and fills in what the sale needs: the product
// and roll, the reference price snapshot, the line total and the cost of the stock sold. It
// reduces the balances in the inventory and returns the problems found (stock that is missing,
// rolls that are not intact, unknown or inactive products). The sale is only valid when none
// were found; line totals are summed into the sale.
func (s *Sale) Allocate(inventory *Inventory) []validation.FieldError {
	var problems []validation.FieldError
	add := func(field, code string) { problems = append(problems, validation.FieldError{Field: field, Code: code}) }

	var total, totalCost decimal.Decimal
	for index := range s.Lines {
		line := &s.Lines[index]
		prefix := fmt.Sprintf("lines[%d]", index)

		var ok bool
		switch line.Kind {
		case KindRollMeters, KindFullRoll:
			ok = allocateRoll(line, prefix, inventory, add)
		case KindTablecloth:
			ok = allocateTablecloth(line, prefix, inventory, add)
		}
		if !ok {
			continue
		}

		var err error
		if line.LineTotal, err = LineTotal(line.UnitPrice, line.Sold()); err != nil || line.LineTotal.Hundredths() <= 0 {
			add(prefix+".unit_price", validation.CodeOutOfRange)
			continue
		}
		if total, err = total.Add(line.LineTotal); err != nil {
			add("lines", validation.CodeOutOfRange)
			continue
		}
		if totalCost, err = totalCost.Add(line.CostOfSale); err != nil {
			add("lines", validation.CodeOutOfRange)
		}
	}
	s.Total, s.TotalCost = total, totalCost
	return problems
}

func allocateRoll(line *Line, prefix string, inventory *Inventory, add func(field, code string)) bool {
	roll, found := inventory.Rolls[line.RollID]
	if !found {
		add(prefix+".roll_id", validation.CodeNotFound)
		return false
	}
	if !roll.ProductActive {
		add(prefix+".roll_id", validation.CodeInactive)
		return false
	}
	line.ProductID, line.ProductSKU, line.ProductName, line.RollCode = roll.ProductID, roll.SKU, roll.Name, roll.Code

	soldBefore := roll.InitialM.Hundredths() - roll.AvailableM.Hundredths()
	var soldNow decimal.Decimal
	switch line.Kind {
	case KindRollMeters:
		if line.Meters.Hundredths() > roll.AvailableM.Hundredths() {
			add(prefix+".meters", validation.CodeInsufficientStock)
			return false
		}
		line.ReferencePrice = roll.ReferencePricePerMeter
		soldNow = line.Meters
	case KindFullRoll:
		// Only a roll that has never been cut or partly sold can be sold whole.
		if soldBefore != 0 {
			add(prefix+".roll_id", validation.CodeRollNotIntact)
			return false
		}
		line.ReferencePrice = roll.ReferencePricePerRoll
		soldNow = roll.InitialM
	}

	// The cost of this sale is the difference of the cost sold to date after and before it, so
	// the costs of every sale of a roll add up exactly to its cost.
	before, err := RollCostSoldToDate(roll.Attributable, roll.InitialM, decimal.FromHundredths(soldBefore))
	if err != nil {
		add(prefix+".roll_id", validation.CodeOutOfRange)
		return false
	}
	after, err := RollCostSoldToDate(roll.Attributable, roll.InitialM, decimal.FromHundredths(soldBefore+soldNow.Hundredths()))
	if err != nil {
		add(prefix+".roll_id", validation.CodeOutOfRange)
		return false
	}
	line.CostOfSale = decimal.FromHundredths(after.Hundredths() - before.Hundredths())
	roll.AvailableM = decimal.FromHundredths(roll.AvailableM.Hundredths() - soldNow.Hundredths())
	return true
}

func allocateTablecloth(line *Line, prefix string, inventory *Inventory, add func(field, code string)) bool {
	stock, found := inventory.Tablecloths[line.ProductID]
	switch {
	case inventory.NotTablecloth[line.ProductID]:
		add(prefix+".product_id", validation.CodeWrongType)
		return false
	case !found:
		add(prefix+".product_id", validation.CodeNotFound)
		return false
	case !stock.ProductActive:
		add(prefix+".product_id", validation.CodeInactive)
		return false
	}
	line.ProductSKU, line.ProductName, line.ReferencePrice = stock.SKU, stock.Name, stock.ReferencePrice

	if line.Quantity > stock.Units {
		add(prefix+".quantity", validation.CodeInsufficientStock)
		return false
	}
	cost, err := TableclothCost(stock.Value, stock.Units, line.Quantity)
	if err != nil {
		add(prefix+".quantity", validation.CodeOutOfRange)
		return false
	}
	line.CostOfSale = cost
	stock.Units -= line.Quantity
	stock.Value = decimal.FromHundredths(stock.Value.Hundredths() - cost.Hundredths())
	return true
}
