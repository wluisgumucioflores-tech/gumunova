package sale

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// Fingerprint is a stable hash of everything the user entered. Two requests with the same
// idempotency key are the same sale only when their fingerprints match.
func (d Draft) Fingerprint() string {
	var builder strings.Builder
	write := func(value string) {
		// Length-prefixed, so no value can be confused with the field that follows it.
		builder.WriteString(strconv.Itoa(len(value)))
		builder.WriteByte(':')
		builder.WriteString(value)
		builder.WriteByte(';')
	}

	write("v1")
	write(dateOnly(d.SaleDate).Format("2006-01-02"))
	write(strings.TrimSpace(d.Notes))
	for _, line := range d.Lines {
		write("line")
		write(string(line.Kind))
		write(strings.TrimSpace(line.RollID))
		write(strings.TrimSpace(line.ProductID))
		write(line.Meters.String())
		write(strconv.Itoa(line.Quantity))
		write(line.UnitPrice.String())
	}

	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}
