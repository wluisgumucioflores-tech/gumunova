package postgres_test

import (
	"context"
	"errors"
	"testing"

	"gumunova/backend/internal/adapters/outbound/postgres"
	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/supplier"
)

func newSupplierRepository(t *testing.T) *postgres.SupplierRepository {
	t.Helper()
	_, pool := newRepository(t) // connects, migrates and empties every table
	return postgres.NewSupplierRepository(pool)
}

func newSupplier(name, phone, email string) supplier.Supplier {
	created, err := supplier.New(supplier.Draft{Name: name, Phone: phone, Email: email, Notes: "nota"})
	if err != nil {
		panic(err)
	}
	return created
}

func TestSupplierCreateAndGet(t *testing.T) {
	repository := newSupplierRepository(t)
	ctx := context.Background()

	created, err := repository.Create(ctx, newSupplier("Plásticos del Sur", "+591 700", "ventas@sur.com"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" || created.CreatedAt.IsZero() || !created.IsActive {
		t.Errorf("generated fields missing: %+v", created)
	}

	loaded, err := repository.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded.Name != "Plásticos del Sur" || loaded.Phone != "+591 700" || loaded.Email != "ventas@sur.com" || loaded.Notes != "nota" {
		t.Errorf("unexpected supplier: %+v", loaded)
	}
}

func TestSupplierUpdate(t *testing.T) {
	repository := newSupplierRepository(t)
	ctx := context.Background()
	created, _ := repository.Create(ctx, newSupplier("Viejo", "", ""))

	created.Name = "Nuevo"
	created.Email = "nuevo@x.com"
	created.Deactivate()
	updated, err := repository.Update(ctx, created)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Nuevo" || updated.Email != "nuevo@x.com" || updated.IsActive {
		t.Errorf("unexpected supplier: %+v", updated)
	}
	if updated.UpdatedAt.Before(created.CreatedAt) {
		t.Error("updated_at must not go backwards")
	}
}

func TestSupplierNotFound(t *testing.T) {
	repository := newSupplierRepository(t)
	ctx := context.Background()

	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-a-uuid", ""} {
		if _, err := repository.GetByID(ctx, id); !errors.Is(err, supplier.ErrNotFound) {
			t.Errorf("GetByID(%q) error = %v, want ErrNotFound", id, err)
		}
	}
	missing := newSupplier("x", "", "")
	missing.ID = "00000000-0000-0000-0000-000000000000"
	if _, err := repository.Update(ctx, missing); !errors.Is(err, supplier.ErrNotFound) {
		t.Errorf("Update error = %v, want ErrNotFound", err)
	}
	missing.ID = "not-a-uuid"
	if _, err := repository.Update(ctx, missing); !errors.Is(err, supplier.ErrNotFound) {
		t.Errorf("Update with bad id error = %v, want ErrNotFound", err)
	}
}

func TestSupplierListFiltersAndPaging(t *testing.T) {
	repository := newSupplierRepository(t)
	ctx := context.Background()
	repository.Create(ctx, newSupplier("Alfa SRL", "70011111", "alfa@x.com"))
	beta, _ := repository.Create(ctx, newSupplier("Beta 100%_", "70022222", "beta@y.com"))
	repository.Create(ctx, newSupplier("Gamma", "70033333", "gamma@x.com"))
	beta.Deactivate()
	repository.Update(ctx, beta)

	active := true
	inactive := false
	tests := []struct {
		name   string
		filter ports.SupplierFilter
		want   []string
		total  int
	}{
		{"all ordered by name", ports.SupplierFilter{Limit: 10}, []string{"Alfa SRL", "Beta 100%_", "Gamma"}, 3},
		{"only active", ports.SupplierFilter{IsActive: &active, Limit: 10}, []string{"Alfa SRL", "Gamma"}, 2},
		{"only inactive", ports.SupplierFilter{IsActive: &inactive, Limit: 10}, []string{"Beta 100%_"}, 1},
		{"search by name", ports.SupplierFilter{Query: "gamma", Limit: 10}, []string{"Gamma"}, 1},
		{"search by phone", ports.SupplierFilter{Query: "70022", Limit: 10}, []string{"Beta 100%_"}, 1},
		{"search by email", ports.SupplierFilter{Query: "@x.com", Limit: 10}, []string{"Alfa SRL", "Gamma"}, 2},
		{"wildcards are literal", ports.SupplierFilter{Query: "100%_", Limit: 10}, []string{"Beta 100%_"}, 1},
		{"percent alone", ports.SupplierFilter{Query: "%", Limit: 10}, []string{"Beta 100%_"}, 1},
		{"paging", ports.SupplierFilter{Limit: 1, Offset: 1}, []string{"Beta 100%_"}, 3},
		{"no results", ports.SupplierFilter{Query: "zzz", Limit: 10}, nil, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, err := repository.List(ctx, test.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			var got []string
			for _, item := range page.Items {
				got = append(got, item.Name)
			}
			if len(got) != len(test.want) {
				t.Fatalf("names = %v, want %v", got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("names = %v, want %v", got, test.want)
				}
			}
			if page.Total != test.total {
				t.Errorf("total = %d, want %d", page.Total, test.total)
			}
		})
	}
}
