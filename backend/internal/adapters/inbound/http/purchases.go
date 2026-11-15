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
	"gumunova/backend/internal/application/usecase/purchasing"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/purchase"
	"gumunova/backend/internal/domain/shared/decimal"
	"gumunova/backend/internal/domain/shared/validation"
)

const (
	dateLayout           = "2006-01-02"
	idempotencyKeyHeader = "Idempotency-Key"
	replayedHeader       = "Idempotent-Replayed"
)

// ---- request ---------------------------------------------------------------

type purchaseRequest struct {
	SupplierID        string                `json:"supplier_id"`
	PurchaseDate      string                `json:"purchase_date"`
	DocumentReference string                `json:"document_reference"`
	Notes             string                `json:"notes"`
	AdditionalCosts   string                `json:"additional_costs"`
	Lines             []purchaseLineRequest `json:"lines"`
}

type purchaseLineRequest struct {
	ProductID string `json:"product_id"`
	// NewProduct creates the product together with the purchase, instead of ProductID.
	NewProduct *productRequest       `json:"new_product"`
	Rolls      []purchaseRollRequest `json:"rolls"`
	Quantity   int                   `json:"quantity"`
	UnitCost   string                `json:"unit_cost"`
}

type purchaseRollRequest struct {
	LengthM string `json:"length_m"`
	Cost    string `json:"cost"`
}

// toDraft converts the request into a domain draft. The kind of each line is not taken from
// the request: the use case reads it from the catalog.
func (r purchaseRequest) toDraft() (purchase.Draft, []validation.FieldError) {
	draft := purchase.Draft{
		SupplierID:        r.SupplierID,
		DocumentReference: r.DocumentReference,
		Notes:             r.Notes,
	}
	var problems []validation.FieldError
	record := func(problem *validation.FieldError) {
		if problem != nil {
			problems = append(problems, *problem)
		}
	}

	if r.PurchaseDate != "" {
		parsed, err := time.Parse(dateLayout, r.PurchaseDate)
		if err != nil {
			problems = append(problems, validation.FieldError{Field: "purchase_date", Code: validation.CodeInvalidFormat})
		} else {
			draft.PurchaseDate = parsed
		}
	}

	var problem *validation.FieldError
	draft.AdditionalCosts, problem = parseDecimal("additional_costs", r.AdditionalCosts, false)
	record(problem)

	for lineIndex, line := range r.Lines {
		prefix := "lines[" + strconv.Itoa(lineIndex) + "]"
		lineDraft := purchase.LineDraft{ProductID: line.ProductID, Quantity: line.Quantity}
		lineDraft.UnitCost, problem = parseDecimal(prefix+".unit_cost", line.UnitCost, false)
		record(problem)

		if line.NewProduct != nil {
			newProduct, productProblems := line.NewProduct.toDraft()
			for _, productProblem := range productProblems {
				problems = append(problems, validation.FieldError{Field: prefix + ".new_product." + productProblem.Field, Code: productProblem.Code})
			}
			lineDraft.NewProduct = &newProduct
		}

		for rollIndex, roll := range line.Rolls {
			rollPrefix := prefix + ".rolls[" + strconv.Itoa(rollIndex) + "]"
			var length, cost decimal.Decimal
			length, problem = parseDecimal(rollPrefix+".length_m", roll.LengthM, true)
			record(problem)
			cost, problem = parseDecimal(rollPrefix+".cost", roll.Cost, true)
			record(problem)
			lineDraft.Rolls = append(lineDraft.Rolls, purchase.RollDraft{LengthM: length, Cost: cost})
		}
		draft.Lines = append(draft.Lines, lineDraft)
	}
	return draft, problems
}

// ---- response --------------------------------------------------------------

type purchaseResponse struct {
	ID                string       `json:"id"`
	Supplier          *supplierRef `json:"supplier"`
	PurchaseDate      string       `json:"purchase_date"`
	DocumentReference string       `json:"document_reference"`
	Notes             string       `json:"notes"`
	AdditionalCosts   string       `json:"additional_costs"`
	TotalAmount       string       `json:"total_amount"`
	Status            string       `json:"status"`
	// Revision grows each time the items are replaced; ItemsEditable says whether they still can be.
	Revision      int                    `json:"revision"`
	ItemsEditable bool                   `json:"items_editable"`
	Lines         []purchaseLineResponse `json:"lines"`
	CreatedAt     time.Time              `json:"created_at"`
}

type supplierRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// optionalSupplierRef is nil for a purchase registered without a supplier.
func optionalSupplierRef(id, name string) *supplierRef {
	if id == "" {
		return nil
	}
	return &supplierRef{ID: id, Name: name}
}

type productRef struct {
	ID          string `json:"id"`
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	ProductType string `json:"product_type"`
}

type purchaseLineResponse struct {
	ID             string                 `json:"id"`
	LineNumber     int                    `json:"line_number"`
	Product        productRef             `json:"product"`
	Quantity       int                    `json:"quantity"`
	UnitCost       *string                `json:"unit_cost"`
	AdditionalCost string                 `json:"additional_cost"`
	LineTotal      string                 `json:"line_total"`
	Rolls          []purchaseRollResponse `json:"rolls"`
}

type purchaseRollResponse struct {
	ID               string `json:"id"`
	RollCode         string `json:"roll_code"`
	LengthM          string `json:"length_m"`
	AcquisitionCost  string `json:"acquisition_cost"`
	AdditionalCost   string `json:"additional_cost"`
	AttributableCost string `json:"attributable_cost"`
}

type purchaseSummaryResponse struct {
	ID                string       `json:"id"`
	Supplier          *supplierRef `json:"supplier"`
	PurchaseDate      string       `json:"purchase_date"`
	DocumentReference string       `json:"document_reference"`
	TotalAmount       string       `json:"total_amount"`
	LineCount         int          `json:"line_count"`
	CreatedAt         time.Time    `json:"created_at"`
}

type purchaseListResponse struct {
	Items  []purchaseSummaryResponse `json:"items"`
	Total  int                       `json:"total"`
	Limit  int                       `json:"limit"`
	Offset int                       `json:"offset"`
}

func toPurchaseResponse(view purchasing.View) purchaseResponse {
	p := view.Purchase
	response := purchaseResponse{
		ID:                p.ID,
		PurchaseDate:      p.PurchaseDate.Format(dateLayout),
		DocumentReference: p.DocumentReference,
		Notes:             p.Notes,
		AdditionalCosts:   p.AdditionalCosts.String(),
		TotalAmount:       p.Total.String(),
		Status:            string(p.Status),
		Revision:          p.Revision,
		ItemsEditable:     view.ItemsEditable,
		Lines:             make([]purchaseLineResponse, len(p.Lines)),
		CreatedAt:         p.CreatedAt,
	}
	if view.Supplier != nil {
		response.Supplier = &supplierRef{ID: view.Supplier.ID, Name: view.Supplier.Name}
	}
	for index, line := range p.Lines {
		item := view.Products[line.ProductID]
		lineResponse := purchaseLineResponse{
			ID:         line.ID,
			LineNumber: line.Number,
			Product: productRef{
				ID: item.ID, SKU: item.SKU, Name: item.Name, ProductType: string(item.Type),
			},
			Quantity:       line.Quantity,
			AdditionalCost: line.AdditionalCost.String(),
			LineTotal:      line.Total.String(),
			Rolls:          make([]purchaseRollResponse, len(line.Rolls)),
		}
		if line.ProductType == product.TypeTablecloth {
			text := line.UnitCost.String()
			lineResponse.UnitCost = &text
		}
		for rollIndex, roll := range line.Rolls {
			lineResponse.Rolls[rollIndex] = purchaseRollResponse{
				ID:               roll.ID,
				RollCode:         roll.Code,
				LengthM:          roll.LengthM.String(),
				AcquisitionCost:  roll.AcquisitionCost.String(),
				AdditionalCost:   roll.AdditionalCost.String(),
				AttributableCost: roll.AttributableCost().String(),
			}
		}
		response.Lines[index] = lineResponse
	}
	return response
}

// ---- handlers --------------------------------------------------------------

type purchaseHandlers struct {
	purchases *purchasing.Service
	logger    *slog.Logger
}

// readDraft decodes and converts the body; it writes the error response itself and returns
// false when the request is not valid.
func readDraft(w nethttp.ResponseWriter, r *nethttp.Request) (purchase.Draft, bool) {
	r.Body = nethttp.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request purchaseRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, nethttp.StatusBadRequest, codeInvalidJSON)
		return purchase.Draft{}, false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		writeError(w, nethttp.StatusBadRequest, codeInvalidJSON)
		return purchase.Draft{}, false
	}

	draft, problems := request.toDraft()
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return purchase.Draft{}, false
	}
	return draft, true
}

func (h purchaseHandlers) update(w nethttp.ResponseWriter, r *nethttp.Request) {
	draft, ok := readDraft(w, r)
	if !ok {
		return
	}
	view, err := h.purchases.UpdatePurchase(r.Context(), r.PathValue("id"), draft)
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toPurchaseResponse(view))
}

func (h purchaseHandlers) register(w nethttp.ResponseWriter, r *nethttp.Request) {
	draft, ok := readDraft(w, r)
	if !ok {
		return
	}

	result, err := h.purchases.RegisterPurchase(r.Context(), draft, r.Header.Get(idempotencyKeyHeader))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	status := nethttp.StatusCreated
	if result.Replayed {
		// The purchase already existed: say so, so the client knows nothing new was created.
		w.Header().Set(replayedHeader, "true")
		status = nethttp.StatusOK
	}
	writeJSON(w, status, toPurchaseResponse(result.View))
}

func (h purchaseHandlers) get(w nethttp.ResponseWriter, r *nethttp.Request) {
	view, err := h.purchases.GetPurchase(r.Context(), r.PathValue("id"))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toPurchaseResponse(view))
}

func (h purchaseHandlers) list(w nethttp.ResponseWriter, r *nethttp.Request) {
	filter, ok := parsePurchaseFilter(r)
	if !ok {
		writeError(w, nethttp.StatusBadRequest, codeInvalidQuery)
		return
	}
	result, err := h.purchases.ListPurchases(r.Context(), filter)
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	items := make([]purchaseSummaryResponse, len(result.Items))
	for i, item := range result.Items {
		items[i] = purchaseSummaryResponse{
			ID:                item.ID,
			Supplier:          optionalSupplierRef(item.SupplierID, item.SupplierName),
			PurchaseDate:      item.PurchaseDate.Format(dateLayout),
			DocumentReference: item.DocumentReference,
			TotalAmount:       item.Total.String(),
			LineCount:         item.LineCount,
			CreatedAt:         item.CreatedAt,
		}
	}
	writeJSON(w, nethttp.StatusOK, purchaseListResponse{Items: items, Total: result.Total, Limit: result.Limit, Offset: result.Offset})
}

func parsePurchaseFilter(r *nethttp.Request) (ports.PurchaseFilter, bool) {
	query := r.URL.Query()
	filter := ports.PurchaseFilter{SupplierID: query.Get("supplier_id"), Query: query.Get("q")}

	for name, target := range map[string]**time.Time{"from": &filter.From, "to": &filter.To} {
		if value := query.Get(name); value != "" {
			parsed, err := time.Parse(dateLayout, value)
			if err != nil {
				return ports.PurchaseFilter{}, false
			}
			*target = &parsed
		}
	}
	if value := query.Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 {
			return ports.PurchaseFilter{}, false
		}
		filter.Limit = limit
	}
	if value := query.Get("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil || offset < 0 {
			return ports.PurchaseFilter{}, false
		}
		filter.Offset = offset
	}
	return filter, true
}
