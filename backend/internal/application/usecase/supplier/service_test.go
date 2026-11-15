package supplier_test

import (
	"context"
	"errors"
	"testing"

	"gumunova/backend/internal/application/ports"
	usecase "gumunova/backend/internal/application/usecase/supplier"
	"gumunova/backend/internal/domain/shared/validation"
	"gumunova/backend/internal/domain/supplier"
	"gumunova/backend/internal/testsupport"
)

func newService() (*usecase.Service, *testsupport.MemorySupplierRepository) {
	repository := testsupport.NewMemorySupplierRepository()
	return usecase.NewService(repository), repository
}

func TestCreateSupplier(t *testing.T) {
	service, _ := newService()

	created, err := service.CreateSupplier(context.Background(), supplier.Draft{Name: " Plásticos del Sur "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID == "" || created.Name != "Plásticos del Sur" || !created.IsActive {
		t.Errorf("unexpected supplier: %+v", created)
	}
}

func TestCreateSupplierRejectsInvalidDraftWithoutPersisting(t *testing.T) {
	service, repository := newService()

	_, err := service.CreateSupplier(context.Background(), supplier.Draft{Name: "", Email: "mal"})

	var problem *validation.ValidationError
	if !errors.As(err, &problem) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if len(repository.Suppliers) != 0 {
		t.Error("an invalid supplier must not be persisted")
	}
}

func TestUpdateSupplier(t *testing.T) {
	service, _ := newService()
	created, _ := service.CreateSupplier(context.Background(), supplier.Draft{Name: "Viejo"})

	updated, err := service.UpdateSupplier(context.Background(), created.ID, supplier.Draft{Name: "Nuevo", Phone: "700"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "Nuevo" || updated.Phone != "700" {
		t.Errorf("unexpected supplier: %+v", updated)
	}
}

func TestUpdateSupplierErrors(t *testing.T) {
	service, _ := newService()
	if _, err := service.UpdateSupplier(context.Background(), "missing", supplier.Draft{Name: "x"}); !errors.Is(err, supplier.ErrNotFound) {
		t.Errorf("missing: error = %v, want ErrNotFound", err)
	}

	created, _ := service.CreateSupplier(context.Background(), supplier.Draft{Name: "P"})
	var problem *validation.ValidationError
	if _, err := service.UpdateSupplier(context.Background(), created.ID, supplier.Draft{Name: ""}); !errors.As(err, &problem) {
		t.Errorf("invalid: error = %v, want validation error", err)
	}
}

func TestDeactivateKeepsTheRecordAndActivateRestoresIt(t *testing.T) {
	service, _ := newService()
	created, _ := service.CreateSupplier(context.Background(), supplier.Draft{Name: "P"})

	off, err := service.DeactivateSupplier(context.Background(), created.ID)
	if err != nil || off.IsActive {
		t.Fatalf("deactivate: active=%v err=%v", off.IsActive, err)
	}
	if _, err := service.GetSupplier(context.Background(), created.ID); err != nil {
		t.Errorf("a deactivated supplier must remain readable: %v", err)
	}
	on, err := service.ActivateSupplier(context.Background(), created.ID)
	if err != nil || !on.IsActive {
		t.Fatalf("activate: active=%v err=%v", on.IsActive, err)
	}
	if _, err := service.DeactivateSupplier(context.Background(), "missing"); !errors.Is(err, supplier.ErrNotFound) {
		t.Errorf("missing: error = %v", err)
	}
}

func TestListSuppliersAppliesPagingDefaultsAndFilters(t *testing.T) {
	service, _ := newService()
	first, _ := service.CreateSupplier(context.Background(), supplier.Draft{Name: "Alfa"})
	service.CreateSupplier(context.Background(), supplier.Draft{Name: "Beta"})
	service.DeactivateSupplier(context.Background(), first.ID)

	all, err := service.ListSuppliers(context.Background(), ports.SupplierFilter{Limit: 10_000, Offset: -3})
	if err != nil {
		t.Fatal(err)
	}
	if all.Limit != 200 || all.Offset != 0 || all.Total != 2 {
		t.Errorf("paging = limit %d offset %d total %d", all.Limit, all.Offset, all.Total)
	}

	active := true
	onlyActive, _ := service.ListSuppliers(context.Background(), ports.SupplierFilter{IsActive: &active})
	if onlyActive.Limit != 50 || len(onlyActive.Items) != 1 || onlyActive.Items[0].Name != "Beta" {
		t.Errorf("active = %+v", onlyActive)
	}
}
