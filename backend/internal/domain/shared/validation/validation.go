// Package validation holds the error type shared by every domain package that validates
// user input, so transport adapters can report field errors the same way everywhere.
package validation

import (
	"fmt"
	"strings"
)

// Stable validation codes. Clients translate them into user-facing text.
const (
	CodeRequired          = "required"
	CodeTooLong           = "too_long"
	CodeInvalidLength     = "invalid_length"
	CodeInvalidFormat     = "invalid_format"
	CodeInvalidEmail      = "invalid_email"
	CodeInvalidValue      = "invalid_value"
	CodeNotAllowed        = "not_allowed"
	CodeMustBePositive    = "must_be_positive"
	CodeMustNotBeNegative = "must_not_be_negative"
	CodeNotFound          = "not_found"
	CodeInactive          = "inactive"
	CodeWrongType         = "wrong_type"
	CodeInFuture          = "in_the_future"
	CodeTooMany           = "too_many"
	CodeOutOfRange        = "out_of_range"
	CodeAlreadyExists     = "already_exists"
	CodeInsufficientStock = "insufficient_stock"
	CodeRollNotIntact     = "roll_not_intact"
)

// FieldError points to one invalid field using its API path (for example "pvc.width_m").
type FieldError struct {
	Field string
	Code  string
}

// ValidationError groups every invalid field found in one entity.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, field := range e.Fields {
		parts[i] = fmt.Sprintf("%s: %s", field.Field, field.Code)
	}
	return "validation failed: " + strings.Join(parts, ", ")
}
