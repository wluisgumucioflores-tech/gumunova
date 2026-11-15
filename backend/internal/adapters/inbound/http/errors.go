package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"

	"gumunova/backend/internal/application/usecase/catalog"
	"gumunova/backend/internal/application/usecase/labelscan"
	"gumunova/backend/internal/application/usecase/purchasing"
	"gumunova/backend/internal/application/usecase/selling"
	"gumunova/backend/internal/domain/inventory"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/purchase"
	"gumunova/backend/internal/domain/sale"
	"gumunova/backend/internal/domain/supplier"
)

// Stable API error codes. Clients map them to user-facing messages; the server never
// sends internal error text to the client.
const (
	codeValidationFailed    = "validation_failed"
	codeInvalidJSON         = "invalid_json"
	codeInvalidQuery        = "invalid_query"
	codeInvalidUpload       = "invalid_upload"
	codeRollNotFound        = "roll_not_found"
	codeSaleNotFound        = "sale_not_found"
	codeProductNotFound     = "product_not_found"
	codeSupplierNotFound    = "supplier_not_found"
	codePurchaseNotFound    = "purchase_not_found"
	codePurchaseItemsLocked = "purchase_items_locked"
	codeLabelNotFound       = "label_not_found"
	codeKeyInvalid          = "idempotency_key_invalid"
	codeKeyReused           = "idempotency_key_reused"
	codeSKUAlreadyExists    = "sku_already_exists"
	codeImageTooLarge       = "image_too_large"
	codeImageNotAllowed     = "image_type_not_supported"
	codeImageEmpty          = "image_empty"
	codeInternal            = "internal_error"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code   string       `json:"code"`
	Fields []fieldError `json:"fields,omitempty"`
}

type fieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

func writeJSON(w nethttp.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w nethttp.ResponseWriter, status int, code string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code}})
}

func writeFieldErrors(w nethttp.ResponseWriter, problems []product.FieldError) {
	fields := make([]fieldError, len(problems))
	for i, problem := range problems {
		fields[i] = fieldError{Field: problem.Field, Code: problem.Code}
	}
	writeJSON(w, nethttp.StatusUnprocessableEntity,
		errorBody{Error: errorDetail{Code: codeValidationFailed, Fields: fields}})
}

// writeUseCaseError translates domain and application errors into HTTP responses.
func writeUseCaseError(w nethttp.ResponseWriter, logger *slog.Logger, err error) {
	var validation *product.ValidationError
	switch {
	case errors.As(err, &validation):
		writeFieldErrors(w, validation.Fields)
	case errors.Is(err, product.ErrNotFound):
		writeError(w, nethttp.StatusNotFound, codeProductNotFound)
	case errors.Is(err, inventory.ErrRollNotFound):
		writeError(w, nethttp.StatusNotFound, codeRollNotFound)
	case errors.Is(err, labelscan.ErrNotFound):
		writeError(w, nethttp.StatusNotFound, codeLabelNotFound)
	case errors.Is(err, purchase.ErrItemsLocked):
		writeError(w, nethttp.StatusConflict, codePurchaseItemsLocked)
	case errors.Is(err, purchase.ErrNotFound):
		writeError(w, nethttp.StatusNotFound, codePurchaseNotFound)
	case errors.Is(err, sale.ErrNotFound):
		writeError(w, nethttp.StatusNotFound, codeSaleNotFound)
	case errors.Is(err, sale.ErrIdempotencyKeyReused):
		writeError(w, nethttp.StatusConflict, codeKeyReused)
	case errors.Is(err, selling.ErrInvalidIdempotencyKey), errors.Is(err, purchasing.ErrInvalidIdempotencyKey):
		writeError(w, nethttp.StatusBadRequest, codeKeyInvalid)
	case errors.Is(err, purchase.ErrIdempotencyKeyReused):
		writeError(w, nethttp.StatusConflict, codeKeyReused)
	case errors.Is(err, supplier.ErrNotFound):
		writeError(w, nethttp.StatusNotFound, codeSupplierNotFound)
	case errors.Is(err, product.ErrSKUAlreadyExists):
		writeError(w, nethttp.StatusConflict, codeSKUAlreadyExists)
	case errors.Is(err, catalog.ErrImageTooLarge):
		writeError(w, nethttp.StatusRequestEntityTooLarge, codeImageTooLarge)
	case errors.Is(err, catalog.ErrImageTypeNotSupported):
		writeError(w, nethttp.StatusUnsupportedMediaType, codeImageNotAllowed)
	case errors.Is(err, catalog.ErrEmptyImage):
		writeError(w, nethttp.StatusBadRequest, codeImageEmpty)
	default:
		logger.Error("unexpected error handling request", "error", err)
		writeError(w, nethttp.StatusInternalServerError, codeInternal)
	}
}
