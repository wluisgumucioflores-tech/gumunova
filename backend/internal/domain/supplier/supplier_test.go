package supplier_test

import (
	"errors"
	"strings"
	"testing"

	"gumunova/backend/internal/domain/shared/validation"
	"gumunova/backend/internal/domain/supplier"
)

func codes(t *testing.T, err error) map[string]string {
	t.Helper()
	var problem *validation.ValidationError
	if !errors.As(err, &problem) {
		t.Fatalf("expected *ValidationError, got %v", err)
	}
	result := map[string]string{}
	for _, field := range problem.Fields {
		result[field.Field] = field.Code
	}
	return result
}

func TestNewTrimsAndActivates(t *testing.T) {
	created, err := supplier.New(supplier.Draft{
		Name: "  Plásticos del Sur ", Phone: " +591 700 00000 ", Email: " ventas@sur.com ", Notes: " Entrega en 3 días ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Name != "Plásticos del Sur" || created.Phone != "+591 700 00000" ||
		created.Email != "ventas@sur.com" || created.Notes != "Entrega en 3 días" {
		t.Errorf("fields not trimmed: %+v", created)
	}
	if !created.IsActive {
		t.Error("new suppliers must be active")
	}
}

func TestOnlyTheNameIsRequired(t *testing.T) {
	if _, err := supplier.New(supplier.Draft{Name: "Proveedor"}); err != nil {
		t.Errorf("a name alone must be enough: %v", err)
	}
	_, err := supplier.New(supplier.Draft{Name: "   "})
	if got := codes(t, err)["name"]; got != validation.CodeRequired {
		t.Errorf("name = %q, want required", got)
	}
}

func TestEmailValidation(t *testing.T) {
	valid := []string{"a@b.com", "ventas@proveedor.com.bo", "ana.perez+pvc@correo.net"}
	for _, email := range valid {
		if _, err := supplier.New(supplier.Draft{Name: "P", Email: email}); err != nil {
			t.Errorf("%q should be valid: %v", email, err)
		}
	}
	invalid := []string{"sin-arroba", "@dominio.com", "a@", "a@dominio", "a@dominio.", "Ana <ana@x.com>", "a b@x.com", "a@@x.com"}
	for _, email := range invalid {
		_, err := supplier.New(supplier.Draft{Name: "P", Email: email})
		if got := codes(t, err)["email"]; got != validation.CodeInvalidEmail {
			t.Errorf("%q: email code = %q, want invalid_email", email, got)
		}
	}
}

func TestLengthLimits(t *testing.T) {
	_, err := supplier.New(supplier.Draft{
		Name:  strings.Repeat("n", 121),
		Phone: strings.Repeat("1", 31),
		Email: strings.Repeat("a", 250) + "@x.com",
		Notes: strings.Repeat("x", 501),
	})
	got := codes(t, err)
	for _, field := range []string{"name", "phone", "email", "notes"} {
		if got[field] != validation.CodeTooLong {
			t.Errorf("%s = %q, want too_long", field, got[field])
		}
	}
}

func TestReviseReplacesTheData(t *testing.T) {
	created, _ := supplier.New(supplier.Draft{Name: "Viejo", Email: "viejo@x.com"})

	if err := created.Revise(supplier.Draft{Name: " Nuevo ", Phone: "123"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Name != "Nuevo" || created.Phone != "123" || created.Email != "" {
		t.Errorf("unexpected supplier: %+v", created)
	}
}

func TestReviseWithInvalidDataChangesNothing(t *testing.T) {
	created, _ := supplier.New(supplier.Draft{Name: "Original", Email: "o@x.com"})

	if err := created.Revise(supplier.Draft{Name: "", Email: "nueva@x.com"}); err == nil {
		t.Fatal("expected a validation error")
	}
	if created.Name != "Original" || created.Email != "o@x.com" {
		t.Errorf("a failed revision must not modify the supplier: %+v", created)
	}
}

func TestActivateAndDeactivate(t *testing.T) {
	created, _ := supplier.New(supplier.Draft{Name: "P"})
	created.Deactivate()
	created.Deactivate()
	if created.IsActive {
		t.Error("expected inactive")
	}
	created.Activate()
	if !created.IsActive {
		t.Error("expected active")
	}
}
