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
	"gumunova/backend/internal/application/usecase/selling"
	"gumunova/backend/internal/domain/sale"
	"gumunova/backend/internal/domain/shared/validation"
)

// ---- request ---------------------------------------------------------------

type saleRequest struct {
	SaleDate string            `json:"sale_date"`
	Notes    string            `json:"notes"`
	Lines    []saleLineRequest `json:"lines"`
}

// saleLineRequest says what to sell. Which fields apply depends on kind: roll_meters takes a
// roll_id and meters, full_roll a roll_id, tablecloth_units a product_id and quantity. The
// unit_price is what is charged (per meter, per roll or per unit).
type saleLineRequest struct {
	Kind      string `json:"kind"`
	RollID    string `json:"roll_id"`
	ProductID string `json:"product_id"`
	Meters    string `json:"meters"`
	Quantity  int    `json:"quantity"`
	UnitPrice string `json:"unit_price"`
}

func (r saleRequest) toDraft() (sale.Draft, []validation.FieldError) {
	draft := sale.Draft{Notes: r.Notes}
	var problems []validation.FieldError
	record := func(problem *validation.FieldError) {
		if problem != nil {
			problems = append(problems, *problem)
		}
	}

	if r.SaleDate != "" {
		parsed, err := time.Parse(dateLayout, r.SaleDate)
		if err != nil {
			problems = append(problems, validation.FieldError{Field: "sale_date", Code: validation.CodeInvalidFormat})
		} else {
			draft.SaleDate = parsed
		}
	}

	for index, line := range r.Lines {
		prefix := "lines[" + strconv.Itoa(index) + "]"
		lineDraft := sale.LineDraft{
			Kind: sale.Kind(line.Kind), RollID: line.RollID, ProductID: line.ProductID, Quantity: line.Quantity,
		}
		var problem *validation.FieldError
		lineDraft.Meters, problem = parseDecimal(prefix+".meters", line.Meters, false)
		record(problem)
		lineDraft.UnitPrice, problem = parseDecimal(prefix+".unit_price", line.UnitPrice, false)
		record(problem)
		draft.Lines = append(draft.Lines, lineDraft)
	}
	return draft, problems
}

// ---- response --------------------------------------------------------------

type saleLineResponse struct {
	ID      string     `json:"id"`
	Number  int        `json:"number"`
	Kind    string     `json:"kind"`
	Product productRef `json:"product"`
	Roll    *rollRef   `json:"roll"`
	// Quantity is in the unit of the line: meters, 1 for a whole roll, or units.
	Quantity       string `json:"quantity"`
	Unit           string `json:"unit"`
	UnitPrice      string `json:"unit_price"`
	ReferencePrice string `json:"reference_price"`
	LineTotal      string `json:"line_total"`
	CostOfSale     string `json:"cost_of_sale"`
	GrossProfit    string `json:"gross_profit"`
}

type saleResponse struct {
	ID          string             `json:"id"`
	SaleDate    string             `json:"sale_date"`
	Notes       string             `json:"notes"`
	Status      string             `json:"status"`
	TotalAmount string             `json:"total_amount"`
	TotalCost   string             `json:"total_cost"`
	GrossProfit string             `json:"gross_profit"`
	Lines       []saleLineResponse `json:"lines"`
	CreatedAt   time.Time          `json:"created_at"`
}

type saleSummaryResponse struct {
	ID          string    `json:"id"`
	SaleDate    string    `json:"sale_date"`
	TotalAmount string    `json:"total_amount"`
	TotalCost   string    `json:"total_cost"`
	GrossProfit string    `json:"gross_profit"`
	LineCount   int       `json:"line_count"`
	CreatedAt   time.Time `json:"created_at"`
}

func saleLineUnit(kind sale.Kind) string {
	switch kind {
	case sale.KindRollMeters:
		return "m"
	case sale.KindFullRoll:
		return "roll"
	default:
		return "unit"
	}
}

func toSaleResponse(s sale.Sale) saleResponse {
	response := saleResponse{
		ID: s.ID, SaleDate: s.SaleDate.Format(dateLayout), Notes: s.Notes, Status: string(s.Status),
		TotalAmount: s.Total.String(), TotalCost: s.TotalCost.String(), GrossProfit: s.GrossProfit().String(),
		Lines: make([]saleLineResponse, len(s.Lines)), CreatedAt: s.CreatedAt,
	}
	for index, line := range s.Lines {
		item := saleLineResponse{
			ID: line.ID, Number: line.Number, Kind: string(line.Kind),
			Product:  productRef{ID: line.ProductID, SKU: line.ProductSKU, Name: line.ProductName},
			Quantity: line.Sold().String(), Unit: saleLineUnit(line.Kind),
			UnitPrice: line.UnitPrice.String(), ReferencePrice: line.ReferencePrice.String(),
			LineTotal: line.LineTotal.String(), CostOfSale: line.CostOfSale.String(), GrossProfit: line.GrossProfit().String(),
		}
		if line.RollID != "" {
			item.Roll = &rollRef{ID: line.RollID, Code: line.RollCode}
		}
		response.Lines[index] = item
	}
	return response
}

// ---- handlers --------------------------------------------------------------

type saleHandlers struct {
	sales  *selling.Service
	logger *slog.Logger
}

func (h saleHandlers) register(w nethttp.ResponseWriter, r *nethttp.Request) {
	r.Body = nethttp.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request saleRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, nethttp.StatusBadRequest, codeInvalidJSON)
		return
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		writeError(w, nethttp.StatusBadRequest, codeInvalidJSON)
		return
	}

	draft, problems := request.toDraft()
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	result, err := h.sales.RegisterSale(r.Context(), draft, r.Header.Get(idempotencyKeyHeader))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	status := nethttp.StatusCreated
	if result.Replayed {
		// The sale already existed: say so, so the client knows nothing new was created.
		w.Header().Set(replayedHeader, "true")
		status = nethttp.StatusOK
	}
	writeJSON(w, status, toSaleResponse(result.Sale))
}

func (h saleHandlers) get(w nethttp.ResponseWriter, r *nethttp.Request) {
	found, err := h.sales.GetSale(r.Context(), r.PathValue("id"))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toSaleResponse(found))
}

func (h saleHandlers) list(w nethttp.ResponseWriter, r *nethttp.Request) {
	query := r.URL.Query()
	filter := ports.SaleFilter{Query: query.Get("q")}
	var ok bool
	if filter.Limit, filter.Offset, ok = parsePaging(query.Get("limit"), query.Get("offset")); !ok {
		writeError(w, nethttp.StatusBadRequest, codeInvalidQuery)
		return
	}
	for name, target := range map[string]**time.Time{"from": &filter.From, "to": &filter.To} {
		if value := query.Get(name); value != "" {
			parsed, err := time.Parse(dateLayout, value)
			if err != nil {
				writeError(w, nethttp.StatusBadRequest, codeInvalidQuery)
				return
			}
			*target = &parsed
		}
	}

	result, err := h.sales.ListSales(r.Context(), filter)
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	items := make([]saleSummaryResponse, len(result.Items))
	for i, item := range result.Items {
		items[i] = saleSummaryResponse{
			ID: item.ID, SaleDate: item.SaleDate.Format(dateLayout), TotalAmount: item.Total.String(),
			TotalCost: item.TotalCost.String(), LineCount: item.LineCount, CreatedAt: item.CreatedAt,
			GrossProfit: item.GrossProfit().String(),
		}
	}
	writeJSON(w, nethttp.StatusOK, pagedResponse[saleSummaryResponse]{Items: items, Total: result.Total, Limit: result.Limit, Offset: result.Offset})
}
