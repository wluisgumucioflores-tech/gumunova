package purchase

// SameItems reports whether two purchases bring in exactly the same stock: the same products,
// quantities, roll lengths and costs (including the shares of additional costs), line by line.
// Editing only the supplier, date, reference or notes keeps the items the same, so the existing
// rolls and their codes stay as they are.
func SameItems(a, b Purchase) bool {
	if len(a.Lines) != len(b.Lines) {
		return false
	}
	for index := range a.Lines {
		left, right := a.Lines[index], b.Lines[index]
		// A line that creates a product is never the same as one that uses an existing product.
		if left.NewProduct != nil || right.NewProduct != nil {
			return false
		}
		if left.ProductID != right.ProductID || left.ProductType != right.ProductType ||
			left.Quantity != right.Quantity || left.UnitCost != right.UnitCost ||
			left.AdditionalCost != right.AdditionalCost || left.Total != right.Total ||
			len(left.Rolls) != len(right.Rolls) {
			return false
		}
		for rollIndex := range left.Rolls {
			l, r := left.Rolls[rollIndex], right.Rolls[rollIndex]
			if l.LengthM != r.LengthM || l.AcquisitionCost != r.AcquisitionCost || l.AdditionalCost != r.AdditionalCost {
				return false
			}
		}
	}
	return true
}
