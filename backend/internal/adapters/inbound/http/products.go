package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	nethttp "net/http"
	"strconv"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/application/usecase/catalog"
	"gumunova/backend/internal/domain/product"
)

const (
	maxJSONBodyBytes = 1 << 20
	// The multipart envelope adds a little to the image itself.
	maxUploadBodyBytes = catalog.MaxImageBytes + 1<<20
	imageFormField     = "image"
)

type productHandlers struct {
	catalog *catalog.Service
	logger  *slog.Logger
}

func (h productHandlers) create(w nethttp.ResponseWriter, r *nethttp.Request) {
	draft, ok := h.readDraft(w, r)
	if !ok {
		return
	}
	view, err := h.catalog.CreateProduct(r.Context(), draft)
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusCreated, toProductResponse(view))
}

func (h productHandlers) update(w nethttp.ResponseWriter, r *nethttp.Request) {
	draft, ok := h.readDraft(w, r)
	if !ok {
		return
	}
	view, err := h.catalog.UpdateProduct(r.Context(), r.PathValue("id"), draft)
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toProductResponse(view))
}

func (h productHandlers) get(w nethttp.ResponseWriter, r *nethttp.Request) {
	view, err := h.catalog.GetProduct(r.Context(), r.PathValue("id"))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toProductResponse(view))
}

func (h productHandlers) list(w nethttp.ResponseWriter, r *nethttp.Request) {
	filter, ok := parseFilter(r)
	if !ok {
		writeError(w, nethttp.StatusBadRequest, codeInvalidQuery)
		return
	}
	result, err := h.catalog.ListProducts(r.Context(), filter)
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	items := make([]productResponse, len(result.Items))
	for i, view := range result.Items {
		items[i] = toProductResponse(view)
	}
	writeJSON(w, nethttp.StatusOK, productListResponse{
		Items: items, Total: result.Total, Limit: result.Limit, Offset: result.Offset,
	})
}

func (h productHandlers) activate(w nethttp.ResponseWriter, r *nethttp.Request) {
	view, err := h.catalog.ActivateProduct(r.Context(), r.PathValue("id"))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toProductResponse(view))
}

func (h productHandlers) resetLabelCode(w nethttp.ResponseWriter, r *nethttp.Request) {
	view, err := h.catalog.ResetProductLabelCode(r.Context(), r.PathValue("id"))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toProductResponse(view))
}

func (h productHandlers) deactivate(w nethttp.ResponseWriter, r *nethttp.Request) {
	view, err := h.catalog.DeactivateProduct(r.Context(), r.PathValue("id"))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toProductResponse(view))
}

func (h productHandlers) setImage(w nethttp.ResponseWriter, r *nethttp.Request) {
	r.Body = nethttp.MaxBytesReader(w, r.Body, maxUploadBodyBytes)
	file, _, err := r.FormFile(imageFormField)
	if err != nil {
		var tooLarge *nethttp.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, nethttp.StatusRequestEntityTooLarge, codeImageTooLarge)
			return
		}
		writeError(w, nethttp.StatusBadRequest, codeInvalidUpload)
		return
	}
	defer file.Close()

	view, err := h.catalog.SetProductImage(r.Context(), r.PathValue("id"), file)
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toProductResponse(view))
}

func (h productHandlers) removeImage(w nethttp.ResponseWriter, r *nethttp.Request) {
	view, err := h.catalog.RemoveProductImage(r.Context(), r.PathValue("id"))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toProductResponse(view))
}

// readDraft decodes the JSON body strictly and maps it to a domain draft. When it returns
// false the error response has already been written.
func (h productHandlers) readDraft(w nethttp.ResponseWriter, r *nethttp.Request) (product.Draft, bool) {
	r.Body = nethttp.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request productRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, nethttp.StatusBadRequest, codeInvalidJSON)
		return product.Draft{}, false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		writeError(w, nethttp.StatusBadRequest, codeInvalidJSON)
		return product.Draft{}, false
	}

	draft, problems := request.toDraft()
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return product.Draft{}, false
	}
	return draft, true
}

func parseFilter(r *nethttp.Request) (ports.ProductFilter, bool) {
	query := r.URL.Query()
	filter := ports.ProductFilter{Query: query.Get("q")}

	switch value := query.Get("type"); value {
	case "":
	case string(product.TypePVC), string(product.TypeTablecloth):
		filter.Type = product.Type(value)
	default:
		return ports.ProductFilter{}, false
	}

	if value := query.Get("is_active"); value != "" {
		active, err := strconv.ParseBool(value)
		if err != nil {
			return ports.ProductFilter{}, false
		}
		filter.IsActive = &active
	}
	if value := query.Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 {
			return ports.ProductFilter{}, false
		}
		filter.Limit = limit
	}
	if value := query.Get("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil || offset < 0 {
			return ports.ProductFilter{}, false
		}
		filter.Offset = offset
	}
	return filter, true
}
