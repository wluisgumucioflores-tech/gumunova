package purchase

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/shared/decimal"
)

// Fingerprint is a stable hash of everything the user entered. Two requests with the same
// idempotency key are the same purchase only when their fingerprints match.
func (d Draft) Fingerprint() string {
	var builder strings.Builder
	write := func(value string) {
		// Length-prefixed, so no value can be confused with the field that follows it.
		builder.WriteString(strconv.Itoa(len(value)))
		builder.WriteByte(':')
		builder.WriteString(value)
		builder.WriteByte(';')
	}

	write("v2")
	write(d.SupplierID)
	write(dateOnly(d.PurchaseDate).Format("2006-01-02"))
	write(strings.TrimSpace(d.DocumentReference))
	write(strings.TrimSpace(d.Notes))
	write(d.AdditionalCosts.String())
	for _, line := range d.Lines {
		write("line")
		write(line.ProductID)
		writeNewProduct(write, line.NewProduct)
		write(strconv.Itoa(line.Quantity))
		write(line.UnitCost.String())
		for _, roll := range line.Rolls {
			write("roll")
			write(roll.LengthM.String())
			write(roll.Cost.String())
		}
	}

	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

// writeNewProduct adds the data of a product created by a line, so a retry that changes it is
// not mistaken for the same request.
func writeNewProduct(write func(string), draft *product.Draft) {
	if draft == nil {
		write("existing")
		return
	}
	write("new")
	write(strings.ToUpper(strings.TrimSpace(draft.SKU)))
	write(strings.TrimSpace(draft.Name))
	write(string(draft.Type))
	optional := func(value *decimal.Decimal) {
		if value == nil {
			write("-")
			return
		}
		write(value.String())
	}
	if details := draft.PVC; details != nil {
		write("pvc")
		for _, text := range []string{details.Material, details.Design, details.Color, details.Texture} {
			write(strings.TrimSpace(text))
		}
		write(details.WidthM.String())
		write(details.NominalLengthM.String())
		optional(details.ThicknessMM)
		write(details.ReferencePricePerMeter.String())
		write(details.ReferencePricePerRoll.String())
	}
	if details := draft.Tablecloth; details != nil {
		write("tablecloth")
		for _, text := range []string{details.Design, details.Color, details.Dimensions} {
			write(strings.TrimSpace(text))
		}
		optional(details.ReferenceCost)
		write(details.UnitPrice.String())
	}
}
