package http_test

import (
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gumunova/backend/internal/domain/sale"
)

const saleKey = "sale-key-0001"

func (s testServer) registerSale(t *testing.T, key, body string) response {
	t.Helper()
	request := httptest.NewRequest(nethttp.MethodPost, "/api/v1/sales", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	return s.serve(request)
}

// stockForSales gives the in-memory repository a roll of 19.80 m (cost 289.54) and 10 tablecloths.
func stockForSales(server testServer) {
	server.sales.Inventory.Rolls["roll-1"] = &sale.RollStock{
		ID: "roll-1", Code: "ROL-000001", ProductID: "pvc-1", SKU: "PVC-01", Name: "PVC floral", ProductActive: true,
		InitialM: dec("19.80"), AvailableM: dec("19.80"), Attributable: dec("289.54"),
		ReferencePricePerMeter: dec("15.50"), ReferencePricePerRoll: dec("280"),
	}
	server.sales.Inventory.Tablecloths["cloth-1"] = &sale.TableclothStock{
		ProductID: "cloth-1", SKU: "MAN-01", Name: "Mantel", ProductActive: true, Units: 10, Value: dec("310.23"), ReferencePrice: dec("60"),
	}
}

const metersSale = `{"sale_date":"2026-11-10","notes":" cliente de paso ","lines":[
  {"kind":"roll_meters","roll_id":"roll-1","meters":"2.50","unit_price":"18"}]}`

func TestRegisterSaleOfMetersFromARoll(t *testing.T) {
	server := newServer()
	stockForSales(server)

	result := server.registerSale(t, saleKey, metersSale)

	if result.status != nethttp.StatusCreated {
		t.Fatalf("status %d body %s", result.status, result.raw)
	}
	body := result.body
	if body["status"] != "confirmed" || body["notes"] != "cliente de paso" || body["sale_date"] != "2026-11-10" {
		t.Errorf("body = %s", result.raw)
	}
	if body["total_amount"] != "45.00" || body["total_cost"] != "36.56" || body["gross_profit"] != "8.44" {
		t.Errorf("amounts = %s", result.raw)
	}
	line := body["lines"].([]any)[0].(map[string]any)
	if line["kind"] != "roll_meters" || line["unit"] != "m" || line["quantity"] != "2.50" ||
		line["unit_price"] != "18.00" || line["reference_price"] != "15.50" {
		t.Errorf("line = %v", line)
	}
	if line["roll"].(map[string]any)["code"] != "ROL-000001" || line["product"].(map[string]any)["sku"] != "PVC-01" {
		t.Errorf("line refs = %v", line)
	}
}

func TestRegisterSaleOfAWholeRollAndTablecloths(t *testing.T) {
	server := newServer()
	stockForSales(server)

	whole := server.registerSale(t, "sale-key-whole", `{"sale_date":"2026-11-10","lines":[{"kind":"full_roll","roll_id":"roll-1","unit_price":"350"}]}`)
	if whole.status != nethttp.StatusCreated || whole.body["total_amount"] != "350.00" || whole.body["total_cost"] != "289.54" {
		t.Fatalf("whole roll: %d %s", whole.status, whole.raw)
	}
	line := whole.body["lines"].([]any)[0].(map[string]any)
	if line["unit"] != "roll" || line["quantity"] != "1.00" || line["reference_price"] != "280.00" {
		t.Errorf("whole roll line = %v", line)
	}

	cloth := server.registerSale(t, "sale-key-cloth", `{"sale_date":"2026-11-10","lines":[{"kind":"tablecloth_units","product_id":"cloth-1","quantity":4,"unit_price":"65"}]}`)
	if cloth.status != nethttp.StatusCreated || cloth.body["total_amount"] != "260.00" || cloth.body["total_cost"] != "124.09" {
		t.Fatalf("tablecloths: %d %s", cloth.status, cloth.raw)
	}
	clothLine := cloth.body["lines"].([]any)[0].(map[string]any)
	if clothLine["unit"] != "unit" || clothLine["quantity"] != "4.00" || clothLine["roll"] != nil {
		t.Errorf("tablecloth line = %v", clothLine)
	}
}

func TestRetryingASaleDoesNotSellTwice(t *testing.T) {
	server := newServer()
	stockForSales(server)
	server.registerSale(t, saleKey, metersSale)

	again := server.registerSale(t, saleKey, metersSale)

	if again.status != nethttp.StatusOK || again.header.Get("Idempotent-Replayed") != "true" {
		t.Errorf("replay status %d headers %v", again.status, again.header)
	}
	if server.sales.RegisterCalls != 1 || server.sales.Inventory.Rolls["roll-1"].AvailableM.String() != "17.30" {
		t.Errorf("calls %d, available %s", server.sales.RegisterCalls, server.sales.Inventory.Rolls["roll-1"].AvailableM)
	}

	different := server.registerSale(t, saleKey, strings.Replace(metersSale, `"2.50"`, `"3.00"`, 1))
	if different.status != nethttp.StatusConflict || errorCode(different) != "idempotency_key_reused" {
		t.Errorf("reused key: %d %q", different.status, errorCode(different))
	}
}

func TestTheIdempotencyKeyIsRequired(t *testing.T) {
	server := newServer()
	stockForSales(server)

	for _, key := range []string{"", "short"} {
		result := server.registerSale(t, key, metersSale)
		if result.status != nethttp.StatusBadRequest || errorCode(result) != "idempotency_key_invalid" {
			t.Errorf("key %q: %d %q", key, result.status, errorCode(result))
		}
	}
}

func TestStockProblemsAreReportedOnTheLine(t *testing.T) {
	server := newServer()
	stockForSales(server)
	body := `{"sale_date":"2026-11-10","lines":[
	  {"kind":"roll_meters","roll_id":"roll-1","meters":"25","unit_price":"18"},
	  {"kind":"tablecloth_units","product_id":"cloth-1","quantity":11,"unit_price":"65"},
	  {"kind":"full_roll","roll_id":"missing","unit_price":"350"}]}`

	result := server.registerSale(t, saleKey, body)

	codes := fieldCodes(result)
	if result.status != 422 || codes["lines[0].meters"] != "insufficient_stock" ||
		codes["lines[1].quantity"] != "insufficient_stock" || codes["lines[2].roll_id"] != "not_found" {
		t.Errorf("status %d codes %v", result.status, codes)
	}
	if server.sales.RegisterCalls != 0 {
		t.Error("nothing may be saved")
	}
}

func TestACutRollIsNotSoldWhole(t *testing.T) {
	server := newServer()
	stockForSales(server)
	server.registerSale(t, saleKey, metersSale)

	result := server.registerSale(t, "sale-key-whole", `{"sale_date":"2026-11-10","lines":[{"kind":"full_roll","roll_id":"roll-1","unit_price":"350"}]}`)

	if codes := fieldCodes(result); result.status != 422 || codes["lines[0].roll_id"] != "roll_not_intact" {
		t.Errorf("status %d codes %v", result.status, codes)
	}
}

func TestSaleInputIsValidated(t *testing.T) {
	server := newServer()
	stockForSales(server)

	unreadable := server.registerSale(t, saleKey, `{"sale_date":"2026-11-10","lines":[{"kind":"roll_meters","roll_id":"roll-1","meters":"abc","unit_price":"1.234"}]}`)
	if codes := fieldCodes(unreadable); unreadable.status != 422 || codes["lines[0].meters"] != "invalid_decimal" || codes["lines[0].unit_price"] != "too_many_decimals" {
		t.Errorf("unreadable: %d %v", unreadable.status, codes)
	}

	invalid := server.registerSale(t, saleKey, `{"sale_date":"2030-01-01","lines":[{"kind":"gift","unit_price":"5"},{"kind":"tablecloth_units","product_id":"cloth-1","quantity":0,"unit_price":"0"}]}`)
	codes := fieldCodes(invalid)
	if invalid.status != 422 || codes["sale_date"] != "in_the_future" || codes["lines[0].kind"] != "invalid_value" ||
		codes["lines[1].quantity"] != "must_be_positive" || codes["lines[1].unit_price"] != "must_be_positive" {
		t.Errorf("invalid: %d %v", invalid.status, codes)
	}

	for name, body := range map[string]string{
		"not json":      `{`,
		"unknown field": `{"sale_date":"2026-11-10","customer":"x","lines":[]}`,
		"trailing":      metersSale + `{}`,
		"empty":         ``,
	} {
		if result := server.registerSale(t, saleKey, body); result.status != 400 || errorCode(result) != "invalid_json" {
			t.Errorf("%s: %d %q", name, result.status, errorCode(result))
		}
	}
}

func TestListAndGetSales(t *testing.T) {
	server := newServer()
	stockForSales(server)
	created := server.registerSale(t, saleKey, metersSale)
	id := created.body["id"].(string)

	list := server.do(t, "GET", "/api/v1/sales?from=2026-11-01&to=2026-11-30&limit=10", "")
	if list.status != nethttp.StatusOK || list.body["total"] != float64(1) {
		t.Fatalf("list: %d %s", list.status, list.raw)
	}
	item := list.body["items"].([]any)[0].(map[string]any)
	if item["id"] != id || item["total_amount"] != "45.00" || item["gross_profit"] != "8.44" || item["line_count"] != float64(1) {
		t.Errorf("item = %v", item)
	}

	if empty := server.do(t, "GET", "/api/v1/sales?from=2026-12-01", ""); empty.body["total"] != float64(0) {
		t.Errorf("from filter: %s", empty.raw)
	}
	for _, query := range []string{"from=yesterday", "limit=0", "offset=-3"} {
		if bad := server.do(t, "GET", "/api/v1/sales?"+query, ""); bad.status != 400 || errorCode(bad) != "invalid_query" {
			t.Errorf("%s: %d %q", query, bad.status, errorCode(bad))
		}
	}

	detail := server.do(t, "GET", "/api/v1/sales/"+id, "")
	if detail.status != nethttp.StatusOK || detail.body["id"] != id {
		t.Errorf("detail: %d %s", detail.status, detail.raw)
	}
	missing := server.do(t, "GET", "/api/v1/sales/nope", "")
	if missing.status != nethttp.StatusNotFound || errorCode(missing) != "sale_not_found" {
		t.Errorf("missing: %d %q", missing.status, errorCode(missing))
	}
}

func TestASaleCannotBeEditedOrDeleted(t *testing.T) {
	server := newServer()
	for _, method := range []string{"PUT", "PATCH", "DELETE"} {
		if result := server.do(t, method, "/api/v1/sales/sale-1", "{}"); result.status != nethttp.StatusMethodNotAllowed {
			t.Errorf("%s = %d, want 405: a confirmed sale is final", method, result.status)
		}
	}
}
