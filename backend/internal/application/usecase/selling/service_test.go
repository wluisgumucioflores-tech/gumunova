package selling_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/application/usecase/selling"
	"gumunova/backend/internal/domain/sale"
	"gumunova/backend/internal/domain/shared/decimal"
	"gumunova/backend/internal/domain/shared/validation"
	"gumunova/backend/internal/testsupport"
)

var fixedNow = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

const key = "key-0001-abcdef"

func newService() (*selling.Service, *testsupport.MemorySaleRepository) {
	repository := testsupport.NewMemorySaleRepository()
	repository.Inventory.Tablecloths["cloth-1"] = &sale.TableclothStock{
		ProductID: "cloth-1", SKU: "MAN-01", Name: "Mantel", ProductActive: true, Units: 10,
		Value: decimal.MustParse("310.23"), ReferencePrice: decimal.MustParse("60"),
	}
	return selling.NewService(repository, func() time.Time { return fixedNow }), repository
}

func clothDraft(quantity int) sale.Draft {
	return sale.Draft{
		SaleDate: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
		Lines:    []sale.LineDraft{{Kind: sale.KindTablecloth, ProductID: "cloth-1", Quantity: quantity, UnitPrice: decimal.MustParse("65")}},
	}
}

func TestRegisterSale(t *testing.T) {
	service, repository := newService()

	result, err := service.RegisterSale(context.Background(), clothDraft(4), key)

	if err != nil || result.Replayed || result.Sale.ID == "" || result.Sale.Total.String() != "260.00" {
		t.Fatalf("result %+v, err %v", result, err)
	}
	if repository.RegisterCalls != 1 {
		t.Errorf("calls = %d", repository.RegisterCalls)
	}
}

func TestTheKeyMustLookValid(t *testing.T) {
	service, repository := newService()

	for _, bad := range []string{"", "short", "has spaces in it!", "x"} {
		if _, err := service.RegisterSale(context.Background(), clothDraft(1), bad); !errors.Is(err, selling.ErrInvalidIdempotencyKey) {
			t.Errorf("key %q: error = %v", bad, err)
		}
	}
	if repository.RegisterCalls != 0 {
		t.Error("nothing may be saved without a valid key")
	}
}

func TestRepeatingTheSameRequestReturnsTheSameSale(t *testing.T) {
	service, repository := newService()
	first, _ := service.RegisterSale(context.Background(), clothDraft(4), key)

	again, err := service.RegisterSale(context.Background(), clothDraft(4), key)

	if err != nil || !again.Replayed || again.Sale.ID != first.Sale.ID || repository.RegisterCalls != 1 {
		t.Errorf("replayed %v, err %v, calls %d", again.Replayed, err, repository.RegisterCalls)
	}
	if units := repository.Inventory.Tablecloths["cloth-1"].Units; units != 6 {
		t.Errorf("units left = %d, a retry must not sell again", units)
	}
}

func TestReusingAKeyForAnotherSaleIsRejected(t *testing.T) {
	service, _ := newService()
	service.RegisterSale(context.Background(), clothDraft(4), key)

	_, err := service.RegisterSale(context.Background(), clothDraft(5), key)

	if !errors.Is(err, sale.ErrIdempotencyKeyReused) {
		t.Errorf("error = %v", err)
	}
}

func TestDomainRulesApplyBeforeAnythingIsSaved(t *testing.T) {
	service, repository := newService()
	draft := clothDraft(1)
	draft.SaleDate = fixedNow.AddDate(0, 0, 5)

	_, err := service.RegisterSale(context.Background(), draft, key)

	var invalid *validation.ValidationError
	if !errors.As(err, &invalid) || invalid.Fields[0].Code != validation.CodeInFuture || repository.RegisterCalls != 0 {
		t.Errorf("error = %v, calls %d", err, repository.RegisterCalls)
	}
}

func TestListingNormalizesThePage(t *testing.T) {
	service, _ := newService()
	for index, quantity := range []int{1, 2, 3} {
		if _, err := service.RegisterSale(context.Background(), clothDraft(quantity), "key-000"+string(rune('a'+index))+"-abcdef"); err != nil {
			t.Fatal(err)
		}
	}

	result, err := service.ListSales(context.Background(), ports.SaleFilter{Limit: -5, Offset: -1})

	if err != nil || result.Limit != 25 || result.Offset != 0 || result.Total != 3 {
		t.Errorf("result %+v, err %v", result, err)
	}
	capped, _ := service.ListSales(context.Background(), ports.SaleFilter{Limit: 5000})
	if capped.Limit != 200 {
		t.Errorf("limit = %d", capped.Limit)
	}
}
