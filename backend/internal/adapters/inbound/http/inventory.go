package http

import (
	"log/slog"
	nethttp "net/http"
	"strconv"
	"time"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/application/usecase/inventoryquery"
	"gumunova/backend/internal/domain/inventory"
	"gumunova/backend/internal/domain/product"
)

type inventoryHandlers struct {
	inventory *inventoryquery.Service
	logger    *slog.Logger
}

// Money and measures travel as decimal strings. Values documented as "display" are rounded for
// reading (cost per meter keeps four decimals) and are not meant for calculations.

type stockItemResponse struct {
	ProductID   string  `json:"product_id"`
	SKU         string  `json:"sku"`
	Name        string  `json:"name"`
	ProductType string  `json:"product_type"`
	IsActive    bool    `json:"is_active"`
	Quantity    string  `json:"quantity"`
	Unit        string  `json:"unit"`
	Minimum     string  `json:"minimum"`
	IsLow       bool    `json:"is_low"`
	RollCount   *int    `json:"roll_count"`
	StockValue  string  `json:"stock_value"`
	AverageCost *string `json:"average_cost"`
}

type rollResponse struct {
	ID               string     `json:"id"`
	Code             string     `json:"code"`
	LabelCode        string     `json:"label_code"`
	Product          productRef `json:"product"`
	PurchaseID       string     `json:"purchase_id"`
	ReceivedAt       string     `json:"received_at"`
	InitialLengthM   string     `json:"initial_length_m"`
	AvailableLengthM string     `json:"available_length_m"`
	AcquisitionCost  string     `json:"acquisition_cost"`
	AdditionalCost   string     `json:"additional_cost"`
	AttributableCost string     `json:"attributable_cost"`
	CostPerMeter     string     `json:"cost_per_meter"`
	RemainingValue   string     `json:"remaining_value"`
	IsIntact         bool       `json:"is_intact"`
}

type rollRef struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}

type movementResponse struct {
	ID            string     `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	Product       productRef `json:"product"`
	Roll          *rollRef   `json:"roll"`
	MovementType  string     `json:"movement_type"`
	QuantityDelta string     `json:"quantity_delta"`
	Unit          string     `json:"unit"`
	ReferenceType string     `json:"reference_type"`
	ReferenceID   string     `json:"reference_id"`
	Reason        string     `json:"reason"`
}

type pagedResponse[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func (h inventoryHandlers) stock(w nethttp.ResponseWriter, r *nethttp.Request) {
	query := r.URL.Query()
	filter := ports.StockFilter{Query: query.Get("q")}
	var ok bool
	if filter.Limit, filter.Offset, ok = parsePaging(query.Get("limit"), query.Get("offset")); !ok {
		writeError(w, nethttp.StatusBadRequest, codeInvalidQuery)
		return
	}
	switch kind := query.Get("type"); kind {
	case "":
	case string(product.TypePVC), string(product.TypeTablecloth):
		filter.Type = product.Type(kind)
	default:
		writeError(w, nethttp.StatusBadRequest, codeInvalidQuery)
		return
	}
	switch query.Get("low") {
	case "":
	case "true":
		filter.OnlyLow = true
	default:
		writeError(w, nethttp.StatusBadRequest, codeInvalidQuery)
		return
	}

	result, err := h.inventory.ListStock(r.Context(), filter)
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	items := make([]stockItemResponse, len(result.Items))
	for i, item := range result.Items {
		response := stockItemResponse{
			ProductID: item.ProductID, SKU: item.SKU, Name: item.Name, ProductType: string(item.Type),
			IsActive: item.IsActive, Quantity: item.Quantity.String(), Minimum: item.Minimum.String(),
			IsLow: item.IsLow, StockValue: item.StockValue.String(),
		}
		if item.Type == product.TypePVC {
			response.Unit = string(inventory.UnitMeter)
			count := item.RollCount
			response.RollCount = &count
		} else {
			response.Unit = string(inventory.UnitPiece)
		}
		if item.AverageCost != "" {
			cost := item.AverageCost
			response.AverageCost = &cost
		}
		items[i] = response
	}
	writeJSON(w, nethttp.StatusOK, pagedResponse[stockItemResponse]{Items: items, Total: result.Total, Limit: result.Limit, Offset: result.Offset})
}

func toRollResponse(item ports.RollItem) rollResponse {
	return rollResponse{
		ID: item.ID, Code: item.Code, LabelCode: item.LabelCode, Product: productRef{ID: item.ProductID, SKU: item.SKU, Name: item.Name},
		PurchaseID: item.PurchaseID, ReceivedAt: item.ReceivedAt.Format(dateLayout),
		InitialLengthM: item.InitialM.String(), AvailableLengthM: item.AvailableM.String(),
		AcquisitionCost: item.Acquisition.String(), AdditionalCost: item.Additional.String(),
		AttributableCost: item.Attributable.String(), CostPerMeter: item.CostPerMeter,
		RemainingValue: item.RemainingValue.String(), IsIntact: item.IsIntact(),
	}
}

func (h inventoryHandlers) rolls(w nethttp.ResponseWriter, r *nethttp.Request) {
	query := r.URL.Query()
	filter := ports.RollFilter{ProductID: query.Get("product_id"), Query: query.Get("q")}
	var ok bool
	if filter.Limit, filter.Offset, ok = parsePaging(query.Get("limit"), query.Get("offset")); !ok {
		writeError(w, nethttp.StatusBadRequest, codeInvalidQuery)
		return
	}
	switch status := query.Get("status"); status {
	case "", ports.RollStatusAvailable, ports.RollStatusDepleted, ports.RollStatusAll:
		filter.Status = status
	default:
		writeError(w, nethttp.StatusBadRequest, codeInvalidQuery)
		return
	}

	result, err := h.inventory.ListRolls(r.Context(), filter)
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	items := make([]rollResponse, len(result.Items))
	for i, item := range result.Items {
		items[i] = toRollResponse(item)
	}
	writeJSON(w, nethttp.StatusOK, pagedResponse[rollResponse]{Items: items, Total: result.Total, Limit: result.Limit, Offset: result.Offset})
}

func (h inventoryHandlers) roll(w nethttp.ResponseWriter, r *nethttp.Request) {
	item, err := h.inventory.GetRoll(r.Context(), r.PathValue("id"))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toRollResponse(item))
}

func (h inventoryHandlers) resetRollLabelCode(w nethttp.ResponseWriter, r *nethttp.Request) {
	item, err := h.inventory.ResetRollLabelCode(r.Context(), r.PathValue("id"))
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	writeJSON(w, nethttp.StatusOK, toRollResponse(item))
}

func (h inventoryHandlers) movements(w nethttp.ResponseWriter, r *nethttp.Request) {
	query := r.URL.Query()
	filter := ports.MovementFilter{ProductID: query.Get("product_id"), RollID: query.Get("roll_id")}
	var ok bool
	if filter.Limit, filter.Offset, ok = parsePaging(query.Get("limit"), query.Get("offset")); !ok {
		writeError(w, nethttp.StatusBadRequest, codeInvalidQuery)
		return
	}
	if kind := inventory.MovementType(query.Get("type")); kind != "" {
		if !kind.IsKnown() {
			writeError(w, nethttp.StatusBadRequest, codeInvalidQuery)
			return
		}
		filter.Type = kind
	}

	result, err := h.inventory.ListMovements(r.Context(), filter)
	if err != nil {
		writeUseCaseError(w, h.logger, err)
		return
	}
	items := make([]movementResponse, len(result.Items))
	for i, item := range result.Items {
		response := movementResponse{
			ID: item.ID, CreatedAt: item.CreatedAt, Product: productRef{ID: item.ProductID, SKU: item.SKU, Name: item.Name},
			MovementType: string(item.Type), QuantityDelta: item.QuantityDelta.String(), Unit: string(item.Unit),
			ReferenceType: item.ReferenceType, ReferenceID: item.ReferenceID, Reason: item.Reason,
		}
		if item.RollID != "" {
			response.Roll = &rollRef{ID: item.RollID, Code: item.RollCode}
		}
		items[i] = response
	}
	writeJSON(w, nethttp.StatusOK, pagedResponse[movementResponse]{Items: items, Total: result.Total, Limit: result.Limit, Offset: result.Offset})
}

// parsePaging reads optional limit and offset; ok is false when either is not a valid number.
func parsePaging(limitText, offsetText string) (limit, offset int, ok bool) {
	if limitText != "" {
		parsed, err := strconv.Atoi(limitText)
		if err != nil || parsed < 1 {
			return 0, 0, false
		}
		limit = parsed
	}
	if offsetText != "" {
		parsed, err := strconv.Atoi(offsetText)
		if err != nil || parsed < 0 {
			return 0, 0, false
		}
		offset = parsed
	}
	return limit, offset, true
}
