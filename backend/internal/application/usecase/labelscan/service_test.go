package labelscan_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/application/usecase/catalog"
	"gumunova/backend/internal/application/usecase/inventoryquery"
	"gumunova/backend/internal/application/usecase/labelscan"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/shared/decimal"
	"gumunova/backend/internal/testsupport"
)

type fixture struct {
	service   *labelscan.Service
	products  *testsupport.MemoryProductRepository
	inventory *testsupport.MemoryInventoryRepository
	pvc       product.Product
	cloth     product.Product
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	products := testsupport.NewMemoryProductRepository()
	inventory := &testsupport.MemoryInventoryRepository{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	catalogService := catalog.NewService(products, testsupport.NewMemoryImageStorage(), logger)

	pvc, err := product.New(product.Draft{SKU: "PVC-01", Name: "PVC floral", Type: product.TypePVC, PVC: &product.PVCDetails{
		WidthM: decimal.MustParse("1.40"), NominalLengthM: decimal.MustParse("20"),
		ReferencePricePerMeter: decimal.MustParse("15"), ReferencePricePerRoll: decimal.MustParse("280"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	pvc, _ = products.Create(context.Background(), pvc)
	cloth, err := product.New(product.Draft{SKU: "MAN-01", Name: "Mantel", Type: product.TypeTablecloth,
		Tablecloth: &product.TableclothDetails{UnitPrice: decimal.MustParse("60")}})
	if err != nil {
		t.Fatal(err)
	}
	cloth, _ = products.Create(context.Background(), cloth)

	inventory.Rolls = []ports.RollItem{{ID: "roll-1", Code: "ROL-000001", LabelCode: "R-0123456789AB", ProductID: pvc.ID, SKU: pvc.SKU, Name: pvc.Name}}
	return fixture{
		service:  labelscan.NewService(catalogService, inventoryquery.NewService(inventory)),
		products: products, inventory: inventory, pvc: pvc, cloth: cloth,
	}
}

func TestAProductCodeResolvesToItsProduct(t *testing.T) {
	f := newFixture(t)

	result, err := f.service.Resolve(context.Background(), f.cloth.LabelCode)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Kind != labelscan.KindProduct || result.Product.Product.SKU != "MAN-01" || result.Roll != nil {
		t.Errorf("result = %+v", result)
	}
}

func TestARollCodeResolvesToTheRollAndItsProduct(t *testing.T) {
	f := newFixture(t)

	result, err := f.service.Resolve(context.Background(), "R-0123456789AB")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Kind != labelscan.KindRoll || result.Roll == nil || result.Roll.Code != "ROL-000001" || result.Product.Product.SKU != "PVC-01" {
		t.Errorf("result = %+v", result)
	}
}

func TestScannerNoiseIsIgnored(t *testing.T) {
	f := newFixture(t)

	// Hardware scanners append a line break; some keyboards change the case.
	for _, code := range []string{"R-0123456789AB\n", "  r-0123456789ab ", "\tR-0123456789AB\r\n"} {
		if _, err := f.service.Resolve(context.Background(), code); err != nil {
			t.Errorf("Resolve(%q): %v", code, err)
		}
	}
}

func TestAnythingElseIsNotFound(t *testing.T) {
	f := newFixture(t)

	for name, code := range map[string]string{
		"empty":              "",
		"a SKU":              "PVC-01",
		"wrong prefix":       "X-0123456789AB",
		"too short":          "R-0123",
		"not hexadecimal":    "R-0123456789AZ",
		"unknown roll":       "R-FFFFFFFFFFFF",
		"unknown product":    "P-FFFFFFFFFFFF",
		"a very long string": string(make([]byte, 5000)),
	} {
		if _, err := f.service.Resolve(context.Background(), code); !errors.Is(err, labelscan.ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
}

func TestARegeneratedCodeNoLongerResolves(t *testing.T) {
	f := newFixture(t)
	old := f.cloth.LabelCode
	if _, err := f.products.ResetLabelCode(context.Background(), f.cloth.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := f.service.Resolve(context.Background(), old); !errors.Is(err, labelscan.ErrNotFound) {
		t.Errorf("an old label must stop working, err = %v", err)
	}
}
