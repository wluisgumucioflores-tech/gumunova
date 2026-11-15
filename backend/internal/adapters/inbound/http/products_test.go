package http_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apihttp "gumunova/backend/internal/adapters/inbound/http"
	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/application/usecase/catalog"
	"gumunova/backend/internal/application/usecase/inventoryquery"
	"gumunova/backend/internal/application/usecase/purchasing"
	"gumunova/backend/internal/application/usecase/selling"
	suppliers "gumunova/backend/internal/application/usecase/supplier"
	"gumunova/backend/internal/testsupport"
)

type testServer struct {
	handler   nethttp.Handler
	storage   *testsupport.MemoryImageStorage
	inventory *testsupport.MemoryInventoryRepository
	sales     *testsupport.MemorySaleRepository
	purchases *testsupport.MemoryPurchaseRepository
}

func newServer() testServer {
	storage := testsupport.NewMemoryImageStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	productRepository := testsupport.NewMemoryProductRepository()
	supplierRepository := testsupport.NewMemorySupplierRepository()
	service := catalog.NewService(productRepository, storage, logger)
	supplierService := suppliers.NewService(supplierRepository)
	purchaseRepository := testsupport.NewMemoryPurchaseRepository(supplierRepository)
	purchaseRepository.Products = productRepository
	purchaseService := purchasing.NewService(
		purchaseRepository, supplierRepository, productRepository,
		func() time.Time { return time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC) })
	inventoryRepository := &testsupport.MemoryInventoryRepository{}
	inventoryService := inventoryquery.NewService(inventoryRepository)
	saleRepository := testsupport.NewMemorySaleRepository()
	saleService := selling.NewService(saleRepository, func() time.Time { return time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC) })
	return testServer{
		handler:   apihttp.NewHandler(service, supplierService, purchaseService, inventoryService, saleService, logger),
		storage:   storage,
		inventory: inventoryRepository,
		sales:     saleRepository,
		purchases: purchaseRepository,
	}
}

type response struct {
	status int
	header nethttp.Header
	body   map[string]any
	raw    string
}

func decodeJSON(data []byte, into *map[string]any) { _ = json.Unmarshal(data, into) }

func (s testServer) do(t *testing.T, method, path, body string) response {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return s.serve(request)
}

func (s testServer) serve(request *nethttp.Request) response {
	recorder := httptest.NewRecorder()
	s.handler.ServeHTTP(recorder, request)
	result := response{status: recorder.Code, header: recorder.Header(), raw: recorder.Body.String()}
	_ = json.Unmarshal(recorder.Body.Bytes(), &result.body)
	return result
}

func (s testServer) upload(t *testing.T, path, field string, content []byte) response {
	t.Helper()
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	part, err := writer.CreateFormFile(field, "photo.bin")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(content)
	writer.Close()

	request := httptest.NewRequest(nethttp.MethodPut, path, &buffer)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return s.serve(request)
}

const validPVC = `{
  "sku": "pvc-floral-20", "name": "PVC floral 20 m", "product_type": "pvc",
  "pvc": {
    "design": "Floral", "color": "Rojo",
    "width_m": "1.40", "nominal_length_m": "20", "thickness_mm": "0.30",
    "reference_price_per_meter": "15.5", "reference_price_per_roll": "280"
  }
}`

const validTablecloth = `{
  "sku": "man-rojo-01", "name": "Mantel rojo", "product_type": "tablecloth",
  "tablecloth": {"dimensions": "1.40 x 2.00 m", "unit_price": "60"}
}`

func errorCode(r response) string {
	detail, _ := r.body["error"].(map[string]any)
	code, _ := detail["code"].(string)
	return code
}

func fieldCodes(r response) map[string]string {
	codes := map[string]string{}
	detail, _ := r.body["error"].(map[string]any)
	fields, _ := detail["fields"].([]any)
	for _, item := range fields {
		entry := item.(map[string]any)
		codes[entry["field"].(string)] = entry["code"].(string)
	}
	return codes
}

var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)

func TestHealth(t *testing.T) {
	if got := newServer().do(t, "GET", "/healthz", ""); got.status != 200 {
		t.Errorf("status = %d", got.status)
	}
}

func TestCreatePVC(t *testing.T) {
	server := newServer()
	created := server.do(t, "POST", "/api/v1/products", validPVC)

	if created.status != nethttp.StatusCreated {
		t.Fatalf("status = %d, body %s", created.status, created.raw)
	}
	if created.body["sku"] != "PVC-FLORAL-20" || created.body["is_active"] != true || created.body["product_type"] != "pvc" {
		t.Errorf("unexpected body: %s", created.raw)
	}
	details := created.body["pvc"].(map[string]any)
	// Decimals come back canonical with two places, as strings.
	if details["width_m"] != "1.40" || details["nominal_length_m"] != "20.00" ||
		details["reference_price_per_meter"] != "15.50" || details["thickness_mm"] != "0.30" {
		t.Errorf("decimals not canonical: %v", details)
	}
	if created.body["image_url"] != nil {
		t.Error("image_url should be null without an image")
	}
	if _, has := created.body["tablecloth"]; has {
		t.Error("tablecloth must be omitted for a PVC product")
	}
}

func TestCreateTablecloth(t *testing.T) {
	created := newServer().do(t, "POST", "/api/v1/products", validTablecloth)
	if created.status != nethttp.StatusCreated {
		t.Fatalf("status = %d, body %s", created.status, created.raw)
	}
	details := created.body["tablecloth"].(map[string]any)
	if details["unit_price"] != "60.00" || details["reference_cost"] != nil {
		t.Errorf("unexpected details: %v", details)
	}
}

func TestCreateValidationErrors(t *testing.T) {
	invalid := `{"sku":"a","name":"","product_type":"pvc","pvc":{
		"width_m":"0","nominal_length_m":"abc","reference_price_per_meter":"1.234","reference_price_per_roll":""}}`

	result := newServer().do(t, "POST", "/api/v1/products", invalid)

	if result.status != nethttp.StatusUnprocessableEntity || errorCode(result) != "validation_failed" {
		t.Fatalf("status %d code %q", result.status, errorCode(result))
	}
	codes := fieldCodes(result)
	want := map[string]string{
		"pvc.nominal_length_m":          "invalid_decimal",
		"pvc.reference_price_per_meter": "too_many_decimals",
		"pvc.reference_price_per_roll":  "required",
	}
	for field, code := range want {
		if codes[field] != code {
			t.Errorf("%s = %q, want %q (all: %v)", field, codes[field], code, codes)
		}
	}
}

func TestCreateDomainValidationErrors(t *testing.T) {
	invalid := `{"sku":"a","name":"","product_type":"pvc","pvc":{
		"width_m":"0","nominal_length_m":"20","reference_price_per_meter":"1","reference_price_per_roll":"1"}}`

	result := newServer().do(t, "POST", "/api/v1/products", invalid)

	codes := fieldCodes(result)
	if result.status != 422 || codes["sku"] != "invalid_length" || codes["name"] != "required" || codes["pvc.width_m"] != "must_be_positive" {
		t.Errorf("status %d codes %v", result.status, codes)
	}
}

func TestCreateRejectsMalformedBodies(t *testing.T) {
	tests := map[string]string{
		"not json":      `{`,
		"unknown field": `{"sku":"PVC-01","name":"x","product_type":"pvc","price":1}`,
		"trailing data": validPVC + `{"x":1}`,
		"wrong type":    `{"sku": 5}`,
		"empty":         ``,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			result := newServer().do(t, "POST", "/api/v1/products", body)
			if result.status != 400 || errorCode(result) != "invalid_json" {
				t.Errorf("status %d code %q", result.status, errorCode(result))
			}
		})
	}
}

func TestCreateDuplicateSKU(t *testing.T) {
	server := newServer()
	server.do(t, "POST", "/api/v1/products", validPVC)

	result := server.do(t, "POST", "/api/v1/products", validPVC)
	if result.status != nethttp.StatusConflict || errorCode(result) != "sku_already_exists" {
		t.Errorf("status %d code %q", result.status, errorCode(result))
	}
}

func TestGetAndNotFound(t *testing.T) {
	server := newServer()
	created := server.do(t, "POST", "/api/v1/products", validPVC)
	id := created.body["id"].(string)

	if got := server.do(t, "GET", "/api/v1/products/"+id, ""); got.status != 200 || got.body["id"] != id {
		t.Errorf("get: status %d body %s", got.status, got.raw)
	}
	missing := server.do(t, "GET", "/api/v1/products/missing", "")
	if missing.status != 404 || errorCode(missing) != "product_not_found" {
		t.Errorf("missing: status %d code %q", missing.status, errorCode(missing))
	}
}

func TestUpdate(t *testing.T) {
	server := newServer()
	id := server.do(t, "POST", "/api/v1/products", validPVC).body["id"].(string)

	revised := strings.Replace(validPVC, `"name": "PVC floral 20 m"`, `"name": "Renamed"`, 1)
	revised = strings.Replace(revised, `"sku": "pvc-floral-20"`, `"sku": "SOMETHING-ELSE"`, 1)
	result := server.do(t, "PUT", "/api/v1/products/"+id, revised)

	if result.status != 200 || result.body["name"] != "Renamed" {
		t.Fatalf("status %d body %s", result.status, result.raw)
	}
	if result.body["sku"] != "PVC-FLORAL-20" {
		t.Errorf("SKU must be immutable, got %v", result.body["sku"])
	}

	if got := server.do(t, "PUT", "/api/v1/products/missing", validPVC); got.status != 404 {
		t.Errorf("missing: status %d", got.status)
	}
}

func TestActivateAndDeactivate(t *testing.T) {
	server := newServer()
	id := server.do(t, "POST", "/api/v1/products", validPVC).body["id"].(string)

	off := server.do(t, "POST", "/api/v1/products/"+id+"/deactivate", "")
	if off.status != 200 || off.body["is_active"] != false {
		t.Fatalf("deactivate: %d %s", off.status, off.raw)
	}
	if got := server.do(t, "GET", "/api/v1/products/"+id, ""); got.status != 200 {
		t.Error("a deactivated product must remain readable")
	}
	on := server.do(t, "POST", "/api/v1/products/"+id+"/activate", "")
	if on.status != 200 || on.body["is_active"] != true {
		t.Fatalf("activate: %d %s", on.status, on.raw)
	}
	if got := server.do(t, "POST", "/api/v1/products/missing/deactivate", ""); got.status != 404 {
		t.Errorf("missing: status %d", got.status)
	}
}

func TestList(t *testing.T) {
	server := newServer()
	server.do(t, "POST", "/api/v1/products", validPVC)
	server.do(t, "POST", "/api/v1/products", validTablecloth)
	offID := server.do(t, "POST", "/api/v1/products", strings.ReplaceAll(validPVC, "pvc-floral-20", "pvc-azul-20")).body["id"].(string)
	server.do(t, "POST", "/api/v1/products/"+offID+"/deactivate", "")

	count := func(query string) (int, float64) {
		result := server.do(t, "GET", "/api/v1/products"+query, "")
		if result.status != 200 {
			t.Fatalf("%s: status %d body %s", query, result.status, result.raw)
		}
		return len(result.body["items"].([]any)), result.body["total"].(float64)
	}

	if items, total := count(""); items != 3 || total != 3 {
		t.Errorf("all: items %d total %v", items, total)
	}
	if items, _ := count("?type=tablecloth"); items != 1 {
		t.Errorf("tablecloth: %d", items)
	}
	if items, _ := count("?is_active=true"); items != 2 {
		t.Errorf("active: %d", items)
	}
	if items, _ := count("?is_active=false"); items != 1 {
		t.Errorf("inactive: %d", items)
	}
	if items, _ := count("?q=mantel"); items != 1 {
		t.Errorf("search: %d", items)
	}
	if items, total := count("?limit=1&offset=1"); items != 1 || total != 3 {
		t.Errorf("paging: items %d total %v", items, total)
	}
}

func TestListEmptyIsAnArrayNotNull(t *testing.T) {
	result := newServer().do(t, "GET", "/api/v1/products", "")
	if !strings.Contains(result.raw, `"items":[]`) {
		t.Errorf("body = %s", result.raw)
	}
}

func TestListRejectsInvalidQuery(t *testing.T) {
	server := newServer()
	for _, query := range []string{"?type=other", "?is_active=maybe", "?limit=0", "?limit=x", "?offset=-1"} {
		result := server.do(t, "GET", "/api/v1/products"+query, "")
		if result.status != 400 || errorCode(result) != "invalid_query" {
			t.Errorf("%s: status %d code %q", query, result.status, errorCode(result))
		}
	}
}

func TestImageLifecycle(t *testing.T) {
	server := newServer()
	id := server.do(t, "POST", "/api/v1/products", validPVC).body["id"].(string)
	path := "/api/v1/products/" + id + "/image"

	uploaded := server.upload(t, path, "image", pngBytes)
	if uploaded.status != 200 {
		t.Fatalf("upload: %d %s", uploaded.status, uploaded.raw)
	}
	url, _ := uploaded.body["image_url"].(string)
	if !strings.HasPrefix(url, "https://storage.test/products/"+id+"/") {
		t.Errorf("image_url = %q", url)
	}
	if got := server.do(t, "GET", "/api/v1/products/"+id, ""); got.body["image_url"] == nil {
		t.Error("the image must be visible when reading the product")
	}
	if len(server.storage.Objects) != 1 {
		t.Errorf("objects = %d", len(server.storage.Objects))
	}

	removed := server.do(t, "DELETE", path, "")
	if removed.status != 200 || removed.body["image_url"] != nil {
		t.Errorf("remove: %d %s", removed.status, removed.raw)
	}
	if len(server.storage.Objects) != 0 {
		t.Error("the object must be deleted from storage")
	}
}

func TestImageUploadErrors(t *testing.T) {
	server := newServer()
	id := server.do(t, "POST", "/api/v1/products", validPVC).body["id"].(string)
	path := "/api/v1/products/" + id + "/image"

	tests := []struct {
		name    string
		field   string
		content []byte
		status  int
		code    string
	}{
		{"not an image", "image", []byte("<html>hello</html>"), 415, "image_type_not_supported"},
		{"empty file", "image", nil, 400, "image_empty"},
		{"wrong form field", "file", pngBytes, 400, "invalid_upload"},
		{"too large", "image", append(append([]byte{}, pngBytes[:8]...), make([]byte, catalog.MaxImageBytes)...), 413, "image_too_large"},
		{"far too large", "image", make([]byte, catalog.MaxImageBytes+2<<20), 413, "image_too_large"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := server.upload(t, path, test.field, test.content)
			if result.status != test.status || errorCode(result) != test.code {
				t.Errorf("status %d code %q, want %d %q", result.status, errorCode(result), test.status, test.code)
			}
		})
	}
	if got := server.upload(t, "/api/v1/products/missing/image", "image", pngBytes); got.status != 404 {
		t.Errorf("unknown product: status %d", got.status)
	}
	if len(server.storage.Objects) != 0 {
		t.Error("rejected uploads must not be stored")
	}
}

func TestUnexpectedErrorsDoNotLeakDetails(t *testing.T) {
	server := newServer()
	id := server.do(t, "POST", "/api/v1/products", validPVC).body["id"].(string)
	server.storage.FailPut = io.ErrUnexpectedEOF

	result := server.upload(t, "/api/v1/products/"+id+"/image", "image", pngBytes)
	if result.status != 500 || errorCode(result) != "internal_error" {
		t.Fatalf("status %d code %q", result.status, errorCode(result))
	}
	if strings.Contains(result.raw, "unexpected EOF") {
		t.Errorf("internal error text leaked to the client: %s", result.raw)
	}
}

func TestSecurityHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	newServer().handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/products", nil))
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("headers = %v", recorder.Header())
	}
}

func TestMinimumStockRoundTripsAndDefaultsToZero(t *testing.T) {
	server := newServer()

	withMinimum := strings.Replace(validPVC, `"reference_price_per_roll": "280"`, `"reference_price_per_roll": "280", "minimum_stock_m": "50"`, 1)
	created := server.do(t, "POST", "/api/v1/products", withMinimum)
	if created.status != nethttp.StatusCreated || created.body["pvc"].(map[string]any)["minimum_stock_m"] != "50.00" {
		t.Errorf("pvc: status %d body %s", created.status, created.raw)
	}

	// Left out, the minimum is zero: no alert.
	plain := server.do(t, "POST", "/api/v1/products", strings.Replace(validPVC, "pvc-floral-20", "pvc-floral-21", 1))
	if plain.body["pvc"].(map[string]any)["minimum_stock_m"] != "0.00" {
		t.Errorf("default minimum: %s", plain.raw)
	}

	cloth := server.do(t, "POST", "/api/v1/products", strings.Replace(validTablecloth, `"unit_price": "60"`, `"unit_price": "60", "minimum_units": 5`, 1))
	if cloth.status != nethttp.StatusCreated || cloth.body["tablecloth"].(map[string]any)["minimum_units"] != float64(5) {
		t.Errorf("tablecloth: status %d body %s", cloth.status, cloth.raw)
	}

	invalid := server.do(t, "POST", "/api/v1/products", strings.Replace(validTablecloth, `"unit_price": "60"`, `"unit_price": "60", "minimum_units": -2`, 1))
	if codes := fieldCodes(invalid); invalid.status != 422 || codes["tablecloth.minimum_units"] != "must_not_be_negative" {
		t.Errorf("invalid: status %d codes %v", invalid.status, codes)
	}
}

func TestProductsExposeAndResetTheirLabelCode(t *testing.T) {
	server := newServer()
	first := server.do(t, "POST", "/api/v1/products", validPVC)
	second := server.do(t, "POST", "/api/v1/products", validTablecloth)
	id := first.body["id"].(string)

	code, other := first.body["label_code"].(string), second.body["label_code"].(string)
	if code == "" || code == other {
		t.Fatalf("each product needs its own code: %q and %q", code, other)
	}

	reset := server.do(t, "POST", "/api/v1/products/"+id+"/label-code/reset", "")
	if reset.status != nethttp.StatusOK || reset.body["label_code"] == code || reset.body["sku"] != "PVC-FLORAL-20" {
		t.Fatalf("reset %d %s", reset.status, reset.raw)
	}

	// Editing the product never touches its label code.
	updated := server.do(t, "PUT", "/api/v1/products/"+id, validPVC)
	if updated.body["label_code"] != reset.body["label_code"] {
		t.Errorf("update changed the label code: %v -> %v", reset.body["label_code"], updated.body["label_code"])
	}

	missing := server.do(t, "POST", "/api/v1/products/nope/label-code/reset", "")
	if missing.status != nethttp.StatusNotFound || errorCode(missing) != "product_not_found" {
		t.Errorf("missing %d %q", missing.status, errorCode(missing))
	}
}

func TestResolveALabelCode(t *testing.T) {
	server := newServer()
	created := server.do(t, "POST", "/api/v1/products", validPVC)
	code := created.body["label_code"].(string)

	found := server.do(t, "GET", "/api/v1/labels/"+code, "")
	if found.status != nethttp.StatusOK || found.body["kind"] != "product" || found.body["roll"] != nil {
		t.Fatalf("status %d, %s", found.status, found.raw)
	}
	product := found.body["product"].(map[string]any)
	if product["name"] == "" || product["pvc"].(map[string]any)["reference_price_per_meter"] != "15.50" {
		t.Errorf("product = %v", product)
	}

	server.inventory.Rolls = []ports.RollItem{{
		ID: "r1", Code: "ROL-000001", LabelCode: "R-0123456789AB", ProductID: created.body["id"].(string),
		SKU: "PVC-FLORAL-20", Name: "PVC", InitialM: dec("19.80"), AvailableM: dec("12.00"),
	}}
	roll := server.do(t, "GET", "/api/v1/labels/r-0123456789ab", "")
	if roll.status != nethttp.StatusOK || roll.body["kind"] != "roll" {
		t.Fatalf("roll: status %d, %s", roll.status, roll.raw)
	}
	detail := roll.body["roll"].(map[string]any)
	if detail["code"] != "ROL-000001" || detail["available_length_m"] != "12.00" || detail["is_intact"] != false {
		t.Errorf("roll = %v", detail)
	}
	if roll.body["product"].(map[string]any)["id"] != created.body["id"] {
		t.Error("a roll must come with its product, which holds the prices")
	}

	unknown := server.do(t, "GET", "/api/v1/labels/P-FFFFFFFFFFFF", "")
	if unknown.status != nethttp.StatusNotFound || errorCode(unknown) != "label_not_found" {
		t.Errorf("unknown: %d %q", unknown.status, errorCode(unknown))
	}
	if junk := server.do(t, "GET", "/api/v1/labels/PVC-FLORAL-20", ""); junk.status != nethttp.StatusNotFound {
		t.Errorf("a SKU is not a label code: %d", junk.status)
	}
}
