package http_test

import (
	"fmt"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const goodKey = "key-0001-abcdef"

// register sends POST /purchases with an Idempotency-Key header (omitted when key is empty).
func (s testServer) register(t *testing.T, key, body string) response {
	t.Helper()
	request := httptest.NewRequest(nethttp.MethodPost, "/api/v1/purchases", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	recorder := httptest.NewRecorder()
	s.handler.ServeHTTP(recorder, request)
	result := response{status: recorder.Code, raw: recorder.Body.String()}
	result.header = recorder.Header()
	decodeJSON(recorder.Body.Bytes(), &result.body)
	return result
}

type purchaseIDs struct{ supplier, pvc, cloth string }

// seed creates a supplier, a PVC product and a tablecloth product through the API.
func seed(t *testing.T, server testServer) purchaseIDs {
	t.Helper()
	supplier := server.do(t, "POST", "/api/v1/suppliers", `{"name":"Plásticos del Sur"}`)
	pvc := server.do(t, "POST", "/api/v1/products", validPVC)
	cloth := server.do(t, "POST", "/api/v1/products", validTablecloth)
	return purchaseIDs{
		supplier: supplier.body["id"].(string),
		pvc:      pvc.body["id"].(string),
		cloth:    cloth.body["id"].(string),
	}
}

func (ids purchaseIDs) body(extra string) string {
	return fmt.Sprintf(`{
	  "supplier_id": %q, "purchase_date": "2026-11-10", "document_reference": " FAC-001 ",
	  "additional_costs": "30",
	  "lines": [
	    {"product_id": %q, "rolls": [{"length_m": "19,80", "cost": "280"}]},
	    {"product_id": %q, "quantity": 10, "unit_cost": "30"}
	  ]%s}`, ids.supplier, ids.pvc, ids.cloth, extra)
}

// The first roll amount uses a comma on purpose: the API only accepts dots.
func (ids purchaseIDs) valid() string {
	return strings.Replace(ids.body(""), "19,80", "19.80", 1)
}

func TestRegisterPurchase(t *testing.T) {
	server := newServer()
	ids := seed(t, server)

	result := server.register(t, goodKey, ids.valid())

	if result.status != nethttp.StatusCreated {
		t.Fatalf("status = %d, body %s", result.status, result.raw)
	}
	if result.header.Get("Idempotent-Replayed") != "" {
		t.Error("a first registration is not a replay")
	}
	body := result.body
	if body["status"] != "confirmed" || body["document_reference"] != "FAC-001" || body["purchase_date"] != "2026-11-10" {
		t.Errorf("unexpected body: %s", result.raw)
	}
	// 280 + 300 of goods + 30 of additional costs.
	if body["total_amount"] != "610.00" || body["additional_costs"] != "30.00" {
		t.Errorf("amounts: %s", result.raw)
	}
	supplier := body["supplier"].(map[string]any)
	if supplier["name"] != "Plásticos del Sur" {
		t.Errorf("supplier = %v", supplier)
	}

	lines := body["lines"].([]any)
	pvcLine := lines[0].(map[string]any)
	if pvcLine["line_number"] != float64(1) || pvcLine["unit_cost"] != nil || pvcLine["quantity"] != float64(1) {
		t.Errorf("pvc line = %v", pvcLine)
	}
	if product := pvcLine["product"].(map[string]any); product["sku"] != "PVC-FLORAL-20" || product["product_type"] != "pvc" {
		t.Errorf("pvc product = %v", product)
	}
	roll := pvcLine["rolls"].([]any)[0].(map[string]any)
	if roll["roll_code"] != "ROL-000001" || roll["length_m"] != "19.80" || roll["acquisition_cost"] != "280.00" ||
		roll["additional_cost"] != "14.48" || roll["attributable_cost"] != "294.48" {
		t.Errorf("roll = %v", roll)
	}
	clothLine := lines[1].(map[string]any)
	if clothLine["unit_cost"] != "30.00" || clothLine["additional_cost"] != "15.52" || clothLine["line_total"] != "315.52" {
		t.Errorf("tablecloth line = %v", clothLine)
	}
	if rolls := clothLine["rolls"].([]any); len(rolls) != 0 {
		t.Errorf("a tablecloth line has no rolls: %v", rolls)
	}
}

func TestRegisterPurchaseRequiresAValidIdempotencyKey(t *testing.T) {
	server := newServer()
	ids := seed(t, server)

	for _, key := range []string{"", "short", "has spaces in it", "ñandú-12345678"} {
		result := server.register(t, key, ids.valid())
		if result.status != 400 || errorCode(result) != "idempotency_key_invalid" {
			t.Errorf("key %q: status %d code %q", key, result.status, errorCode(result))
		}
	}
	if list := server.do(t, "GET", "/api/v1/purchases", ""); list.body["total"] != float64(0) {
		t.Error("nothing may be saved without a valid key")
	}
}

func TestRepeatingARequestIsHarmless(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	first := server.register(t, goodKey, ids.valid())

	retry := server.register(t, goodKey, ids.valid())

	if retry.status != nethttp.StatusOK || retry.header.Get("Idempotent-Replayed") != "true" {
		t.Errorf("retry: status %d replayed %q", retry.status, retry.header.Get("Idempotent-Replayed"))
	}
	if retry.body["id"] != first.body["id"] {
		t.Error("a retry must return the original purchase")
	}
	firstRoll := first.body["lines"].([]any)[0].(map[string]any)["rolls"].([]any)[0].(map[string]any)["roll_code"]
	retryRoll := retry.body["lines"].([]any)[0].(map[string]any)["rolls"].([]any)[0].(map[string]any)["roll_code"]
	if firstRoll != retryRoll {
		t.Errorf("roll codes %v / %v: a retry must not create new rolls", firstRoll, retryRoll)
	}
	if list := server.do(t, "GET", "/api/v1/purchases", ""); list.body["total"] != float64(1) {
		t.Errorf("purchases = %v, want 1", list.body["total"])
	}
}

func TestReusingAKeyForADifferentPurchaseIsAConflict(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	server.register(t, goodKey, ids.valid())

	different := strings.Replace(ids.valid(), "FAC-001", "FAC-002", 1)
	result := server.register(t, goodKey, different)

	if result.status != nethttp.StatusConflict || errorCode(result) != "idempotency_key_reused" {
		t.Errorf("status %d code %q", result.status, errorCode(result))
	}
}

func TestRegisterPurchaseValidationErrors(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	body := fmt.Sprintf(`{
	  "supplier_id": %q, "purchase_date": "2026-11-10",
	  "lines": [
	    {"product_id": %q, "rolls": [{"length_m": "0", "cost": "1"}]},
	    {"product_id": %q, "quantity": 0, "unit_cost": "0"}
	  ]}`, ids.supplier, ids.pvc, ids.cloth)

	result := server.register(t, goodKey, body)

	if result.status != 422 || errorCode(result) != "validation_failed" {
		t.Fatalf("status %d code %q", result.status, errorCode(result))
	}
	codes := fieldCodes(result)
	want := map[string]string{
		"lines[0].rolls[0].length_m": "must_be_positive",
		"lines[1].quantity":          "must_be_positive",
		"lines[1].unit_cost":         "must_be_positive",
	}
	for field, code := range want {
		if codes[field] != code {
			t.Errorf("%s = %q, want %q (all: %v)", field, codes[field], code, codes)
		}
	}
}

func TestRegisterPurchaseReportsUnreadableValues(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	body := fmt.Sprintf(`{
	  "supplier_id": %q, "purchase_date": "10/11/2026", "additional_costs": "1.234",
	  "lines": [
	    {"product_id": %q, "rolls": [{"length_m": "abc", "cost": ""}]},
	    {"product_id": %q, "quantity": 2, "unit_cost": "1,5"}
	  ]}`, ids.supplier, ids.pvc, ids.cloth)

	result := server.register(t, goodKey, body)

	codes := fieldCodes(result)
	want := map[string]string{
		"purchase_date":              "invalid_format",
		"additional_costs":           "too_many_decimals",
		"lines[0].rolls[0].length_m": "invalid_decimal",
		"lines[0].rolls[0].cost":     "required",
		"lines[1].unit_cost":         "invalid_decimal",
	}
	for field, code := range want {
		if codes[field] != code {
			t.Errorf("%s = %q, want %q (all: %v)", field, codes[field], code, codes)
		}
	}
}

func TestRegisterPurchaseWithoutSupplier(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	body := strings.Replace(ids.valid(), fmt.Sprintf(`"supplier_id": %q,`, ids.supplier), "", 1)

	created := server.register(t, goodKey, body)
	if created.status != nethttp.StatusCreated {
		t.Fatalf("status = %d, body %s", created.status, created.raw)
	}
	if value, present := created.body["supplier"]; !present || value != nil {
		t.Errorf("supplier = %v, want null", value)
	}

	list := server.do(t, "GET", "/api/v1/purchases", "")
	items := list.body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["supplier"] != nil {
		t.Errorf("list = %s", list.raw)
	}
}

const newProductLine = `{
  "new_product": {
    "sku": "pvc-nuevo", "name": "PVC nuevo", "product_type": "pvc",
    "pvc": {"width_m": "1.40", "nominal_length_m": "20",
            "reference_price_per_meter": "15", "reference_price_per_roll": "280"}
  },
  "rolls": [{"length_m": "19.80", "cost": "280"}]
}`

func newProductPurchase(line string) string {
	return `{"purchase_date": "2026-11-10", "lines": [` + line + `]}`
}

func TestRegisterPurchaseCreatingANewProduct(t *testing.T) {
	server := newServer()

	created := server.register(t, goodKey, newProductPurchase(newProductLine))
	if created.status != nethttp.StatusCreated {
		t.Fatalf("status = %d, body %s", created.status, created.raw)
	}
	product := created.body["lines"].([]any)[0].(map[string]any)["product"].(map[string]any)
	if product["sku"] != "PVC-NUEVO" {
		t.Errorf("product = %v", product)
	}

	catalog := server.do(t, "GET", "/api/v1/products?q=PVC-NUEVO", "")
	if catalog.body["total"] != float64(1) {
		t.Errorf("catalog = %s", catalog.raw)
	}

	again := server.register(t, goodKey, newProductPurchase(newProductLine))
	if again.status != nethttp.StatusOK || again.header.Get("Idempotent-Replayed") != "true" {
		t.Errorf("replay status %d", again.status)
	}
	if server.do(t, "GET", "/api/v1/products?q=PVC-NUEVO", "").body["total"] != float64(1) {
		t.Error("a retry must not create the product again")
	}
}

func TestNewProductErrorsPointToTheLine(t *testing.T) {
	server := newServer()

	unreadable := strings.Replace(newProductLine, `"width_m": "1.40"`, `"width_m": "abc"`, 1)
	result := server.register(t, goodKey, newProductPurchase(unreadable))
	if codes := fieldCodes(result); result.status != 422 || codes["lines[0].new_product.pvc.width_m"] != "invalid_decimal" {
		t.Errorf("status %d codes %v", result.status, codes)
	}

	badSKU := strings.Replace(newProductLine, `"sku": "pvc-nuevo"`, `"sku": "x"`, 1)
	result = server.register(t, goodKey, newProductPurchase(badSKU))
	if codes := fieldCodes(result); result.status != 422 || codes["lines[0].new_product.sku"] != "invalid_length" {
		t.Errorf("status %d codes %v", result.status, codes)
	}
}

func TestRegisterPurchaseChecksSupplierAndProducts(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	server.do(t, "POST", "/api/v1/suppliers/"+ids.supplier+"/deactivate", "")
	server.do(t, "POST", "/api/v1/products/"+ids.cloth+"/deactivate", "")
	body := strings.Replace(ids.valid(), ids.pvc, "00000000-0000-0000-0000-000000000000", 1)

	result := server.register(t, goodKey, body)

	codes := fieldCodes(result)
	if result.status != 422 || codes["supplier_id"] != "inactive" || codes["lines[0].product_id"] != "not_found" ||
		codes["lines[1].product_id"] != "inactive" {
		t.Errorf("status %d codes %v", result.status, codes)
	}
}

func TestRegisterPurchaseRejectsMalformedBodies(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	tests := map[string]string{
		"not json":         `{`,
		"unknown field":    `{"supplier_id":"x","status":"voided"}`,
		"trailing data":    ids.valid() + `{}`,
		"fractional units": strings.Replace(ids.valid(), `"quantity": 10`, `"quantity": 1.5`, 1),
		"empty":            ``,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			result := server.register(t, goodKey, body)
			if result.status != 400 || errorCode(result) != "invalid_json" {
				t.Errorf("status %d code %q", result.status, errorCode(result))
			}
		})
	}
}

func TestAClientCannotChooseTheLineKind(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	// Rolls sent for a tablecloth product: the catalog says it is a tablecloth, so they are refused.
	body := fmt.Sprintf(`{"supplier_id": %q, "purchase_date": "2026-11-10", "lines": [
	  {"product_id": %q, "rolls": [{"length_m": "20", "cost": "100"}]}]}`, ids.supplier, ids.cloth)

	result := server.register(t, goodKey, body)

	if result.status != 422 || fieldCodes(result)["lines[0].rolls"] != "not_allowed" {
		t.Errorf("status %d codes %v", result.status, fieldCodes(result))
	}
}

func TestGetPurchase(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	id := server.register(t, goodKey, ids.valid()).body["id"].(string)

	got := server.do(t, "GET", "/api/v1/purchases/"+id, "")
	if got.status != 200 || got.body["id"] != id || len(got.body["lines"].([]any)) != 2 {
		t.Errorf("get: %d %s", got.status, got.raw)
	}

	missing := server.do(t, "GET", "/api/v1/purchases/missing", "")
	if missing.status != 404 || errorCode(missing) != "purchase_not_found" {
		t.Errorf("missing: %d %q", missing.status, errorCode(missing))
	}
}

func TestListPurchases(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	server.register(t, "key-0001-aaaaaa", ids.valid())
	later := strings.Replace(ids.valid(), "2026-11-10", "2026-11-11", 1)
	later = strings.Replace(later, "FAC-001", "FAC-002", 1)
	server.register(t, "key-0002-bbbbbb", later)

	count := func(query string) (int, float64) {
		result := server.do(t, "GET", "/api/v1/purchases"+query, "")
		if result.status != 200 {
			t.Fatalf("%s: status %d body %s", query, result.status, result.raw)
		}
		return len(result.body["items"].([]any)), result.body["total"].(float64)
	}

	if items, total := count(""); items != 2 || total != 2 {
		t.Errorf("all: %d/%v", items, total)
	}
	if items, _ := count("?from=2026-11-11"); items != 1 {
		t.Errorf("from: %d", items)
	}
	if items, _ := count("?to=2026-11-10"); items != 1 {
		t.Errorf("to: %d", items)
	}
	if items, _ := count("?q=fac-002"); items != 1 {
		t.Errorf("q: %d", items)
	}
	if items, _ := count("?supplier_id=" + ids.supplier); items != 2 {
		t.Errorf("supplier: %d", items)
	}
	if items, total := count("?limit=1&offset=1"); items != 1 || total != 2 {
		t.Errorf("paging: %d/%v", items, total)
	}

	first := server.do(t, "GET", "/api/v1/purchases", "").body["items"].([]any)[0].(map[string]any)
	if first["document_reference"] != "FAC-002" || first["line_count"] != float64(2) || first["total_amount"] != "610.00" {
		t.Errorf("newest first / summary: %v", first)
	}
	if first["supplier"].(map[string]any)["name"] != "Plásticos del Sur" {
		t.Errorf("supplier = %v", first["supplier"])
	}
}

func TestListPurchasesEmptyIsAnArrayAndRejectsBadQueries(t *testing.T) {
	server := newServer()
	if empty := server.do(t, "GET", "/api/v1/purchases", ""); !strings.Contains(empty.raw, `"items":[]`) {
		t.Errorf("body = %s", empty.raw)
	}
	for _, query := range []string{"?from=yesterday", "?to=2026-13-40", "?limit=0", "?offset=-1"} {
		result := server.do(t, "GET", "/api/v1/purchases"+query, "")
		if result.status != 400 || errorCode(result) != "invalid_query" {
			t.Errorf("%s: status %d code %q", query, result.status, errorCode(result))
		}
	}
}

func TestPurchasesCannotBePatchedOrDeletedThroughTheAPI(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	id := server.register(t, goodKey, ids.valid()).body["id"].(string)

	for _, method := range []string{"PATCH", "DELETE"} {
		result := server.do(t, method, "/api/v1/purchases/"+id, `{}`)
		if result.status != nethttp.StatusMethodNotAllowed && result.status != nethttp.StatusNotFound {
			t.Errorf("%s: status %d, a confirmed purchase must not be patched or deleted", method, result.status)
		}
	}
}

func TestEditPurchase(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	created := server.register(t, goodKey, ids.valid())
	id := created.body["id"].(string)
	if created.body["revision"] != float64(1) || created.body["items_editable"] != true {
		t.Errorf("revision/items_editable: %s", created.raw)
	}

	// Same items, new reference and notes: the purchase keeps its revision.
	dataOnly := strings.Replace(ids.valid(), "FAC-001", "FAC-777", 1)
	edited := server.do(t, "PUT", "/api/v1/purchases/"+id, dataOnly)
	if edited.status != nethttp.StatusOK || edited.body["document_reference"] != "FAC-777" || edited.body["revision"] != float64(1) {
		t.Fatalf("data edit: status %d, %s", edited.status, edited.raw)
	}
	if server.purchases.LastReplaceItems {
		t.Error("editing only the data must not replace the items")
	}

	// Different quantity: the items are replaced.
	moreCloth := strings.Replace(ids.valid(), `"quantity": 10`, `"quantity": 12`, 1)
	replaced := server.do(t, "PUT", "/api/v1/purchases/"+id, moreCloth)
	if replaced.status != nethttp.StatusOK || replaced.body["revision"] != float64(2) || replaced.body["total_amount"] != "670.00" {
		t.Fatalf("items edit: status %d, %s", replaced.status, replaced.raw)
	}
	if !server.purchases.LastReplaceItems {
		t.Error("changing the items must replace them")
	}
}

func TestEditPurchaseRejectsInvalidDataAndUnknownPurchase(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	id := server.register(t, goodKey, ids.valid()).body["id"].(string)

	invalid := server.do(t, "PUT", "/api/v1/purchases/"+id, `{"purchase_date":"2026-11-10","lines":[]}`)
	if invalid.status != 422 || errorCode(invalid) != "validation_failed" {
		t.Errorf("invalid: status %d, %s", invalid.status, invalid.raw)
	}
	missing := server.do(t, "PUT", "/api/v1/purchases/00000000-0000-0000-0000-000000000000", ids.valid())
	if missing.status != 404 || errorCode(missing) != "purchase_not_found" {
		t.Errorf("missing: status %d, %s", missing.status, missing.raw)
	}
}

func TestEditPurchaseItemsAfterASaleIsAConflictButDataStillEdits(t *testing.T) {
	server := newServer()
	ids := seed(t, server)
	id := server.register(t, goodKey, ids.valid()).body["id"].(string)
	server.purchases.ItemsLocked = true

	locked := server.do(t, "PUT", "/api/v1/purchases/"+id, strings.Replace(ids.valid(), `"quantity": 10`, `"quantity": 12`, 1))
	if locked.status != nethttp.StatusConflict || errorCode(locked) != "purchase_items_locked" {
		t.Errorf("status %d, %s", locked.status, locked.raw)
	}
	if got := server.do(t, "GET", "/api/v1/purchases/"+id, ""); got.body["items_editable"] != false {
		t.Errorf("items_editable = %v, want false", got.body["items_editable"])
	}
	dataOnly := server.do(t, "PUT", "/api/v1/purchases/"+id, strings.Replace(ids.valid(), "FAC-001", "FAC-9", 1))
	if dataOnly.status != nethttp.StatusOK {
		t.Errorf("data edit: status %d, %s", dataOnly.status, dataOnly.raw)
	}
}
