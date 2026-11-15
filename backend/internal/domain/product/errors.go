package product

import (
	"errors"

	"gumunova/backend/internal/domain/shared/validation"
)

var (
	ErrNotFound         = errors.New("product not found")
	ErrSKUAlreadyExists = errors.New("product sku already exists")
)

// The validation vocabulary is shared with the other domain packages.
type (
	FieldError      = validation.FieldError
	ValidationError = validation.ValidationError
)

const (
	CodeRequired          = validation.CodeRequired
	CodeTooLong           = validation.CodeTooLong
	CodeInvalidLength     = validation.CodeInvalidLength
	CodeInvalidFormat     = validation.CodeInvalidFormat
	CodeInvalidValue      = validation.CodeInvalidValue
	CodeNotAllowed        = validation.CodeNotAllowed
	CodeMustBePositive    = validation.CodeMustBePositive
	CodeMustNotBeNegative = validation.CodeMustNotBeNegative
)
