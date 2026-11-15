package http_test

import (
	nethttp "net/http"
	"testing"
	"time"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/inventory"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/shared/decimal"
)

func dec(value string) decimal.Decimal { return decimal.MustParse(value) }

func TestStockResponseShape(t *testing.T) {
	server := newServer()
	server.inventory.Stock = []ports.StockItem{
		{ProductID: "p1", SKU: "PVC-01", Name: "PVC", Type: product.TypePVC, IsActive: true,
			Quantity: dec("40.30"), Minimum: dec("50"), IsLow: true, RollCount: 2, StockValue: dec("600.50")},
		{ProductID: "p2", SKU: "MAN-01", Name: "Mantel", Type: product.TypeTablecloth, IsActive: true,
			Quantity: dec("10"), Minimum: dec("0"), StockValue: dec("450"), AverageCost: "45.0000"},
	}

	result := server.do(t, "GET", "/api/v1/inventory/stock", "")

	if result.status != nethttp.StatusOK || result.body["total"] != float64(2) || result.body["limit"] != float64(25) {
		t.Fatalf("status %d body %s", result.status, result.raw)
	}
	items := result.body["items"].([]any)
	pvc, cloth := items[0].(map[string]any), items[1].(map[string]any)
	if pvc["unit"] != "m" || pvc["quantity"] != "40.30" || pvc["is_low"] != true || pvc["roll_count"] != float64(2) || pvc["average_cost"] != nil {
		t.Errorf("pvc = %v", pvc)
	}
	if cloth["unit"] != "unit" || cloth["roll_count"] != nil || cloth["average_cost"] != "45.0000" || cloth["stock_value"] != "450.00" {
		t.Errorf("tablecloth = %v", cloth)
	}
}

func TestStockFiltersAreParsedAndValidated(t *testing.T) {
	server := newServer()

	ok := server.do(t, "GET", "/api/v1/inventory/stock?type=pvc&low=true&q=floral&limit=5&offset=10", "")
	if ok.status != nethttp.StatusOK {
		t.Fatalf("status %d", ok.status)
	}
	got := server.inventory.LastStockFilter
	if got.Type != product.TypePVC || !got.OnlyLow || got.Query != "floral" || got.Limit != 5 || got.Offset != 10 {
		t.Errorf("filter = %+v", got)
	}

	for _, query := range []string{"type=glass", "low=maybe", "limit=0", "limit=x", "offset=-1"} {
		if result := server.do(t, "GET", "/api/v1/inventory/stock?"+query, ""); result.status != nethttp.StatusBadRequest || errorCode(result) != "invalid_query" {
			t.Errorf("%s: status %d code %q", query, result.status, errorCode(result))
		}
	}
}

func TestRollsAndTheirDetail(t *testing.T) {
	server := newServer()
	server.inventory.Rolls = []ports.RollItem{{
		ID: "r1", Code: "ROL-000001", ProductID: "p1", SKU: "PVC-01", Name: "PVC", PurchaseID: "pu1",
		ReceivedAt: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
		InitialM:   dec("19.80"), AvailableM: dec("19.80"), Acquisition: dec("280"), Additional: dec("9.54"),
		Attributable: dec("289.54"), CostPerMeter: "14.6232", RemainingValue: dec("289.54"),
	}}

	list := server.do(t, "GET", "/api/v1/inventory/rolls?status=all", "")
	if list.status != nethttp.StatusOK || server.inventory.LastRollFilter.Status != "all" {
		t.Fatalf("status %d filter %+v", list.status, server.inventory.LastRollFilter)
	}
	roll := list.body["items"].([]any)[0].(map[string]any)
	if roll["code"] != "ROL-000001" || roll["received_at"] != "2026-11-10" || roll["cost_per_meter"] != "14.6232" ||
		roll["attributable_cost"] != "289.54" || roll["is_intact"] != true {
		t.Errorf("roll = %v", roll)
	}
	if roll["product"].(map[string]any)["sku"] != "PVC-01" {
		t.Errorf("product = %v", roll["product"])
	}

	detail := server.do(t, "GET", "/api/v1/inventory/rolls/r1", "")
	if detail.status != nethttp.StatusOK || detail.body["id"] != "r1" {
		t.Errorf("detail %d %s", detail.status, detail.raw)
	}
	missing := server.do(t, "GET", "/api/v1/inventory/rolls/nope", "")
	if missing.status != nethttp.StatusNotFound || errorCode(missing) != "roll_not_found" {
		t.Errorf("missing %d %q", missing.status, errorCode(missing))
	}

	if bad := server.do(t, "GET", "/api/v1/inventory/rolls?status=broken", ""); bad.status != nethttp.StatusBadRequest {
		t.Errorf("bad status filter: %d", bad.status)
	}
}

func TestMovements(t *testing.T) {
	server := newServer()
	server.inventory.Movements = []ports.MovementItem{
		{ID: "m1", CreatedAt: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC), ProductID: "p1", SKU: "PVC-01", Name: "PVC",
			RollID: "r1", RollCode: "ROL-000001", Type: inventory.MovementPurchaseIn, QuantityDelta: dec("19.80"),
			Unit: inventory.UnitMeter, ReferenceType: "purchase", ReferenceID: "pu1"},
		{ID: "m2", ProductID: "p2", SKU: "MAN-01", Name: "Mantel", Type: inventory.MovementPurchaseIn,
			QuantityDelta: dec("10"), Unit: inventory.UnitPiece, ReferenceType: "purchase", ReferenceID: "pu1"},
	}

	result := server.do(t, "GET", "/api/v1/inventory/movements?product_id=p1&roll_id=r1&type=purchase_in", "")

	if result.status != nethttp.StatusOK {
		t.Fatalf("status %d body %s", result.status, result.raw)
	}
	filter := server.inventory.LastMovementFilter
	if filter.ProductID != "p1" || filter.RollID != "r1" || filter.Type != inventory.MovementPurchaseIn {
		t.Errorf("filter = %+v", filter)
	}
	items := result.body["items"].([]any)
	byRoll, byUnits := items[0].(map[string]any), items[1].(map[string]any)
	if byRoll["roll"].(map[string]any)["code"] != "ROL-000001" || byRoll["quantity_delta"] != "19.80" || byRoll["unit"] != "m" {
		t.Errorf("roll movement = %v", byRoll)
	}
	if byUnits["roll"] != nil || byUnits["unit"] != "unit" {
		t.Errorf("unit movement = %v", byUnits)
	}

	if bad := server.do(t, "GET", "/api/v1/inventory/movements?type=teleport", ""); bad.status != nethttp.StatusBadRequest {
		t.Errorf("unknown movement type: %d", bad.status)
	}
}

func TestInventoryHasNoStockWriteRoutes(t *testing.T) {
	server := newServer()
	for _, path := range []string{"/api/v1/inventory/stock", "/api/v1/inventory/rolls", "/api/v1/inventory/movements"} {
		if result := server.do(t, "POST", path, "{}"); result.status != nethttp.StatusMethodNotAllowed {
			t.Errorf("POST %s = %d, want 405: stock only changes through operations that write movements", path, result.status)
		}
	}
}

func TestResetRollLabelCode(t *testing.T) {
	server := newServer()
	server.inventory.Rolls = []ports.RollItem{{
		ID: "r1", Code: "ROL-000001", LabelCode: "R-OLD", ProductID: "p1", SKU: "PVC-01", Name: "PVC",
		ReceivedAt: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
		InitialM:   dec("19.80"), AvailableM: dec("19.80"),
	}}

	before := server.do(t, "GET", "/api/v1/inventory/rolls/r1", "")
	if before.body["label_code"] != "R-OLD" {
		t.Errorf("label_code = %v", before.body["label_code"])
	}
	reset := server.do(t, "POST", "/api/v1/inventory/rolls/r1/label-code/reset", "")
	if reset.status != nethttp.StatusOK || reset.body["label_code"] == "R-OLD" || reset.body["code"] != "ROL-000001" {
		t.Errorf("reset %d %s", reset.status, reset.raw)
	}
	if reset.body["available_length_m"] != "19.80" {
		t.Error("resetting a label code must not change the stock")
	}
	missing := server.do(t, "POST", "/api/v1/inventory/rolls/nope/label-code/reset", "")
	if missing.status != nethttp.StatusNotFound || errorCode(missing) != "roll_not_found" {
		t.Errorf("missing %d %q", missing.status, errorCode(missing))
	}
}
