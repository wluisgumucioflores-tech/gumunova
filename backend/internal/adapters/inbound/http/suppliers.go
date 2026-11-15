package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	nethttp "net/http"
	"strconv"
	"time"

	"gumunova/backend/internal/application/ports"
	usecase "gumunova/backend/internal/application/usecase/supplier"
	domain "gumunova/backend/internal/domain/supplier"
)

type supplierRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
	Notes string `json:"notes"`
}

type supplierResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	Notes     string    `json:"notes"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type supplierListResponse struct {
	Items  []supplierResponse `json:"items"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

func toSupplierResponse(s domain.Supplier) supplierResponse {
	return supplierResponse{
		ID: s.ID, Name: s.Name, Phone: s.Phone, Email: s.Email, Notes: s.Notes,
		IsActive: s.IsActive, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

type supplierHandlers struct {
	suppliers *usecase.Service
	logger    *slog.Logger
}

func (h supplierHandlers) create(w nethttp.ResponseWriter, r *nethttp.Request) {
	draft, ok := readSupplierDraft(w, r)
	if !ok {
		return
	}
	created, err := h.suppliers.CreateSupplier(r.Context(), draft)
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusCreated, toSupplierResponse(created))
}

func (h supplierHandlers) update(w nethttp.ResponseWriter, r *nethttp.Request) {
	draft, ok := readSupplierDraft(w, r)
	if !ok {
		return
	}
	updated, err := h.suppliers.UpdateSupplier(r.Context(), r.PathValue("id"), draft)
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toSupplierResponse(updated))
}

func (h supplierHandlers) get(w nethttp.ResponseWriter, r *nethttp.Request) {
	found, err := h.suppliers.GetSupplier(r.Context(), r.PathValue("id"))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toSupplierResponse(found))
}

func (h supplierHandlers) list(w nethttp.ResponseWriter, r *nethttp.Request) {
	filter, ok := parseSupplierFilter(r)
	if !ok {
		writeError(w, nethttp.StatusBadRequest, codeInvalidQuery)
		return
	}
	result, err := h.suppliers.ListSuppliers(r.Context(), filter)
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	items := make([]supplierResponse, len(result.Items))
	for i, item := range result.Items {
		items[i] = toSupplierResponse(item)
	}
	writeJSON(w, nethttp.StatusOK, supplierListResponse{
		Items: items, Total: result.Total, Limit: result.Limit, Offset: result.Offset,
	})
}

func (h supplierHandlers) activate(w nethttp.ResponseWriter, r *nethttp.Request) {
	updated, err := h.suppliers.ActivateSupplier(r.Context(), r.PathValue("id"))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toSupplierResponse(updated))
}

func (h supplierHandlers) deactivate(w nethttp.ResponseWriter, r *nethttp.Request) {
	updated, err := h.suppliers.DeactivateSupplier(r.Context(), r.PathValue("id"))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toSupplierResponse(updated))
}

// readSupplierDraft decodes the JSON body strictly. When it returns false the error response
// has already been written.
func readSupplierDraft(w nethttp.ResponseWriter, r *nethttp.Request) (domain.Draft, bool) {
	r.Body = nethttp.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request supplierRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, nethttp.StatusBadRequest, codeInvalidJSON)
		return domain.Draft{}, false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		writeError(w, nethttp.StatusBadRequest, codeInvalidJSON)
		return domain.Draft{}, false
	}
	return domain.Draft{Name: request.Name, Phone: request.Phone, Email: request.Email, Notes: request.Notes}, true
}

func parseSupplierFilter(r *nethttp.Request) (ports.SupplierFilter, bool) {
	query := r.URL.Query()
	filter := ports.SupplierFilter{Query: query.Get("q")}

	if value := query.Get("is_active"); value != "" {
		active, err := strconv.ParseBool(value)
		if err != nil {
			return ports.SupplierFilter{}, false
		}
		filter.IsActive = &active
	}
	if value := query.Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 {
			return ports.SupplierFilter{}, false
		}
		filter.Limit = limit
	}
	if value := query.Get("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil || offset < 0 {
			return ports.SupplierFilter{}, false
		}
		filter.Offset = offset
	}
	return filter, true
}
