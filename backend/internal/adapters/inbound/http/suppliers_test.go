package http_test

import (
	"strings"
	"testing"
)

const validSupplier = `{"name":"  Plásticos del Sur ","phone":"+591 700 00000","email":"ventas@sur.com","notes":"Entrega en 3 días"}`

func TestCreateSupplier(t *testing.T) {
	created := newServer().do(t, "POST", "/api/v1/suppliers", validSupplier)

	if created.status != 201 {
		t.Fatalf("status = %d, body %s", created.status, created.raw)
	}
	if created.body["name"] != "Plásticos del Sur" || created.body["email"] != "ventas@sur.com" || created.body["is_active"] != true {
		t.Errorf("unexpected body: %s", created.raw)
	}
}

func TestCreateSupplierWithOnlyAName(t *testing.T) {
	created := newServer().do(t, "POST", "/api/v1/suppliers", `{"name":"Proveedor"}`)
	if created.status != 201 || created.body["phone"] != "" {
		t.Errorf("status %d body %s", created.status, created.raw)
	}
}

func TestCreateSupplierValidationErrors(t *testing.T) {
	result := newServer().do(t, "POST", "/api/v1/suppliers", `{"name":"","email":"mal"}`)

	if result.status != 422 || errorCode(result) != "validation_failed" {
		t.Fatalf("status %d code %q", result.status, errorCode(result))
	}
	codes := fieldCodes(result)
	if codes["name"] != "required" || codes["email"] != "invalid_email" {
		t.Errorf("codes = %v", codes)
	}
}

func TestCreateSupplierRejectsMalformedBodies(t *testing.T) {
	tests := map[string]string{
		"not json":      `{`,
		"unknown field": `{"name":"P","address":"x"}`,
		"trailing data": validSupplier + `{}`,
		"empty":         ``,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			result := newServer().do(t, "POST", "/api/v1/suppliers", body)
			if result.status != 400 || errorCode(result) != "invalid_json" {
				t.Errorf("status %d code %q", result.status, errorCode(result))
			}
		})
	}
}

func TestGetUpdateAndNotFoundSupplier(t *testing.T) {
	server := newServer()
	id := server.do(t, "POST", "/api/v1/suppliers", validSupplier).body["id"].(string)

	if got := server.do(t, "GET", "/api/v1/suppliers/"+id, ""); got.status != 200 || got.body["id"] != id {
		t.Errorf("get: %d %s", got.status, got.raw)
	}
	updated := server.do(t, "PUT", "/api/v1/suppliers/"+id, `{"name":"Renombrado"}`)
	if updated.status != 200 || updated.body["name"] != "Renombrado" || updated.body["email"] != "" {
		t.Errorf("update: %d %s", updated.status, updated.raw)
	}
	if bad := server.do(t, "PUT", "/api/v1/suppliers/"+id, `{"name":""}`); bad.status != 422 {
		t.Errorf("invalid update: status %d", bad.status)
	}

	missing := server.do(t, "GET", "/api/v1/suppliers/missing", "")
	if missing.status != 404 || errorCode(missing) != "supplier_not_found" {
		t.Errorf("missing: %d %q", missing.status, errorCode(missing))
	}
	if got := server.do(t, "PUT", "/api/v1/suppliers/missing", validSupplier); got.status != 404 {
		t.Errorf("update missing: status %d", got.status)
	}
}

func TestActivateAndDeactivateSupplier(t *testing.T) {
	server := newServer()
	id := server.do(t, "POST", "/api/v1/suppliers", validSupplier).body["id"].(string)

	off := server.do(t, "POST", "/api/v1/suppliers/"+id+"/deactivate", "")
	if off.status != 200 || off.body["is_active"] != false {
		t.Fatalf("deactivate: %d %s", off.status, off.raw)
	}
	if got := server.do(t, "GET", "/api/v1/suppliers/"+id, ""); got.status != 200 {
		t.Error("a deactivated supplier must remain readable")
	}
	on := server.do(t, "POST", "/api/v1/suppliers/"+id+"/activate", "")
	if on.status != 200 || on.body["is_active"] != true {
		t.Fatalf("activate: %d %s", on.status, on.raw)
	}
	if got := server.do(t, "POST", "/api/v1/suppliers/missing/deactivate", ""); got.status != 404 {
		t.Errorf("missing: status %d", got.status)
	}
}

func TestListSuppliers(t *testing.T) {
	server := newServer()
	server.do(t, "POST", "/api/v1/suppliers", `{"name":"Alfa","email":"alfa@x.com"}`)
	server.do(t, "POST", "/api/v1/suppliers", `{"name":"Beta"}`)
	offID := server.do(t, "POST", "/api/v1/suppliers", `{"name":"Gamma"}`).body["id"].(string)
	server.do(t, "POST", "/api/v1/suppliers/"+offID+"/deactivate", "")

	count := func(query string) (int, float64) {
		result := server.do(t, "GET", "/api/v1/suppliers"+query, "")
		if result.status != 200 {
			t.Fatalf("%s: status %d body %s", query, result.status, result.raw)
		}
		return len(result.body["items"].([]any)), result.body["total"].(float64)
	}

	if items, total := count(""); items != 3 || total != 3 {
		t.Errorf("all: items %d total %v", items, total)
	}
	if items, _ := count("?is_active=true"); items != 2 {
		t.Errorf("active: %d", items)
	}
	if items, _ := count("?is_active=false"); items != 1 {
		t.Errorf("inactive: %d", items)
	}
	if items, _ := count("?q=alfa"); items != 1 {
		t.Errorf("search: %d", items)
	}
	if items, total := count("?limit=1&offset=1"); items != 1 || total != 3 {
		t.Errorf("paging: items %d total %v", items, total)
	}
}

func TestListSuppliersEmptyIsAnArray(t *testing.T) {
	result := newServer().do(t, "GET", "/api/v1/suppliers", "")
	if !strings.Contains(result.raw, `"items":[]`) {
		t.Errorf("body = %s", result.raw)
	}
}

func TestListSuppliersRejectsInvalidQuery(t *testing.T) {
	server := newServer()
	for _, query := range []string{"?is_active=maybe", "?limit=0", "?limit=x", "?offset=-1"} {
		result := server.do(t, "GET", "/api/v1/suppliers"+query, "")
		if result.status != 400 || errorCode(result) != "invalid_query" {
			t.Errorf("%s: status %d code %q", query, result.status, errorCode(result))
		}
	}
}
