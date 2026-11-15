package catalog_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/application/usecase/catalog"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/shared/decimal"
	"gumunova/backend/internal/testsupport"
)

// ---- helpers --------------------------------------------------------------

func newService() (*catalog.Service, *testsupport.MemoryProductRepository, *testsupport.MemoryImageStorage) {
	repository := testsupport.NewMemoryProductRepository()
	storage := testsupport.NewMemoryImageStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return catalog.NewService(repository, storage, logger), repository, storage
}

func pvcDraft(sku string) product.Draft {
	return product.Draft{
		SKU:  sku,
		Name: "PVC floral",
		Type: product.TypePVC,
		PVC: &product.PVCDetails{
			WidthM:                 decimal.MustParse("1.40"),
			NominalLengthM:         decimal.MustParse("20"),
			ReferencePricePerMeter: decimal.MustParse("15"),
			ReferencePricePerRoll:  decimal.MustParse("280"),
		},
	}
}

func tableclothDraft(sku string) product.Draft {
	return product.Draft{
		SKU:        sku,
		Name:       "Mantel rojo",
		Type:       product.TypeTablecloth,
		Tablecloth: &product.TableclothDetails{UnitPrice: decimal.MustParse("60")},
	}
}

var (
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
	jpegBytes = append([]byte("\xff\xd8\xff\xe0"), bytes.Repeat([]byte{0}, 64)...)
)

// ---- tests ----------------------------------------------------------------

func TestCreateProduct(t *testing.T) {
	service, _, _ := newService()

	view, err := service.CreateProduct(context.Background(), pvcDraft("pvc-01"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.Product.ID == "" || view.Product.SKU != "PVC-01" || !view.Product.IsActive {
		t.Errorf("unexpected product: %+v", view.Product)
	}
	if view.ImageURL != "" {
		t.Error("a new product has no image")
	}
}

func TestCreateProductRejectsInvalidDraft(t *testing.T) {
	service, repository, _ := newService()
	draft := pvcDraft("pvc-01")
	draft.PVC.WidthM = decimal.MustParse("0")

	_, err := service.CreateProduct(context.Background(), draft)

	var validation *product.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if len(repository.Products) != 0 {
		t.Error("an invalid product must not be persisted")
	}
}

func TestCreateProductRejectsDuplicateSKU(t *testing.T) {
	service, _, _ := newService()
	if _, err := service.CreateProduct(context.Background(), pvcDraft("pvc-01")); err != nil {
		t.Fatal(err)
	}

	_, err := service.CreateProduct(context.Background(), tableclothDraft("PVC-01"))
	if !errors.Is(err, product.ErrSKUAlreadyExists) {
		t.Errorf("error = %v, want ErrSKUAlreadyExists", err)
	}
}

func TestUpdateProduct(t *testing.T) {
	service, _, _ := newService()
	created, _ := service.CreateProduct(context.Background(), pvcDraft("pvc-01"))

	revision := pvcDraft("ignored")
	revision.Name = "Renamed"
	updated, err := service.UpdateProduct(context.Background(), created.Product.ID, revision)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Product.Name != "Renamed" || updated.Product.SKU != "PVC-01" {
		t.Errorf("unexpected product: %+v", updated.Product)
	}
}

func TestUpdateProductErrors(t *testing.T) {
	service, _, _ := newService()

	if _, err := service.UpdateProduct(context.Background(), "missing", pvcDraft("x")); !errors.Is(err, product.ErrNotFound) {
		t.Errorf("missing product: error = %v, want ErrNotFound", err)
	}

	created, _ := service.CreateProduct(context.Background(), pvcDraft("pvc-01"))
	invalid := pvcDraft("pvc-01")
	invalid.Name = ""
	var validation *product.ValidationError
	if _, err := service.UpdateProduct(context.Background(), created.Product.ID, invalid); !errors.As(err, &validation) {
		t.Errorf("invalid revision: error = %v, want validation error", err)
	}
}

func TestDeactivateAndActivate(t *testing.T) {
	service, _, _ := newService()
	created, _ := service.CreateProduct(context.Background(), pvcDraft("pvc-01"))

	deactivated, err := service.DeactivateProduct(context.Background(), created.Product.ID)
	if err != nil || deactivated.Product.IsActive {
		t.Fatalf("deactivate: active=%v err=%v", deactivated.Product.IsActive, err)
	}
	// The record is kept, not deleted.
	if _, err := service.GetProduct(context.Background(), created.Product.ID); err != nil {
		t.Errorf("deactivated product must still be readable: %v", err)
	}
	activated, err := service.ActivateProduct(context.Background(), created.Product.ID)
	if err != nil || !activated.Product.IsActive {
		t.Fatalf("activate: active=%v err=%v", activated.Product.IsActive, err)
	}
	if _, err := service.DeactivateProduct(context.Background(), "missing"); !errors.Is(err, product.ErrNotFound) {
		t.Errorf("missing product: error = %v", err)
	}
}

func TestListProductsAppliesPagingDefaultsAndFilters(t *testing.T) {
	service, _, _ := newService()
	service.CreateProduct(context.Background(), pvcDraft("pvc-01"))
	service.CreateProduct(context.Background(), tableclothDraft("man-01"))

	all, err := service.ListProducts(context.Background(), ports.ProductFilter{Limit: 10_000, Offset: -5})
	if err != nil {
		t.Fatal(err)
	}
	if all.Limit != 200 || all.Offset != 0 || all.Total != 2 {
		t.Errorf("paging = limit %d offset %d total %d", all.Limit, all.Offset, all.Total)
	}

	onlyCloths, _ := service.ListProducts(context.Background(), ports.ProductFilter{Type: product.TypeTablecloth})
	if onlyCloths.Limit != 50 || len(onlyCloths.Items) != 1 {
		t.Errorf("filtered = limit %d items %d", onlyCloths.Limit, len(onlyCloths.Items))
	}
}

func TestSetProductImage(t *testing.T) {
	service, repository, storage := newService()
	created, _ := service.CreateProduct(context.Background(), pvcDraft("pvc-01"))

	view, err := service.SetProductImage(context.Background(), created.Product.ID, bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(view.Product.ImageKey, "products/"+created.Product.ID+"/") || !strings.HasSuffix(view.Product.ImageKey, ".png") {
		t.Errorf("unexpected image key %q", view.Product.ImageKey)
	}
	if view.ImageURL == "" {
		t.Error("expected a signed image URL")
	}
	if storage.Objects[view.Product.ImageKey] != "image/png" {
		t.Errorf("stored content type = %q", storage.Objects[view.Product.ImageKey])
	}
	if repository.Products[created.Product.ID].ImageKey != view.Product.ImageKey {
		t.Error("image key must be persisted")
	}
}

func TestSetProductImageReplacesPreviousObject(t *testing.T) {
	service, _, storage := newService()
	created, _ := service.CreateProduct(context.Background(), pvcDraft("pvc-01"))
	first, _ := service.SetProductImage(context.Background(), created.Product.ID, bytes.NewReader(pngBytes))

	second, err := service.SetProductImage(context.Background(), created.Product.ID, bytes.NewReader(jpegBytes))
	if err != nil {
		t.Fatal(err)
	}
	if _, stillThere := storage.Objects[first.Product.ImageKey]; stillThere {
		t.Error("the previous image object must be deleted")
	}
	if _, ok := storage.Objects[second.Product.ImageKey]; !ok {
		t.Error("the new image object must exist")
	}
}

func TestSetProductImageRejectsInvalidContent(t *testing.T) {
	service, _, storage := newService()
	created, _ := service.CreateProduct(context.Background(), pvcDraft("pvc-01"))
	id := created.Product.ID

	tests := []struct {
		name    string
		content []byte
		want    error
	}{
		{"empty", nil, catalog.ErrEmptyImage},
		{"text file renamed as image", []byte("<html>not an image</html>"), catalog.ErrImageTypeNotSupported},
		{"gif", []byte("GIF89a" + strings.Repeat("0", 64)), catalog.ErrImageTypeNotSupported},
		{"too large", append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, catalog.MaxImageBytes)...), catalog.ErrImageTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.SetProductImage(context.Background(), id, bytes.NewReader(test.content))
			if !errors.Is(err, test.want) {
				t.Errorf("error = %v, want %v", err, test.want)
			}
		})
	}
	if len(storage.Objects) != 0 {
		t.Error("rejected images must not reach storage")
	}
}

func TestSetProductImageUnknownProduct(t *testing.T) {
	service, _, _ := newService()
	_, err := service.SetProductImage(context.Background(), "missing", bytes.NewReader(pngBytes))
	if !errors.Is(err, product.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestSetProductImageCleansUpWhenPersistingFails(t *testing.T) {
	service, repository, storage := newService()
	created, _ := service.CreateProduct(context.Background(), pvcDraft("pvc-01"))
	repository.FailSetImageKey = errors.New("database down")

	_, err := service.SetProductImage(context.Background(), created.Product.ID, bytes.NewReader(pngBytes))
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(storage.Objects) != 0 {
		t.Error("the uploaded object must be removed when the key cannot be persisted")
	}
}

func TestSetProductImageStorageFailure(t *testing.T) {
	service, repository, storage := newService()
	created, _ := service.CreateProduct(context.Background(), pvcDraft("pvc-01"))
	storage.FailPut = errors.New("storage down")

	if _, err := service.SetProductImage(context.Background(), created.Product.ID, bytes.NewReader(pngBytes)); err == nil {
		t.Fatal("expected an error")
	}
	if repository.Products[created.Product.ID].ImageKey != "" {
		t.Error("the key must not be persisted when the upload fails")
	}
}

func TestRemoveProductImage(t *testing.T) {
	service, _, storage := newService()
	created, _ := service.CreateProduct(context.Background(), pvcDraft("pvc-01"))
	withImage, _ := service.SetProductImage(context.Background(), created.Product.ID, bytes.NewReader(pngBytes))

	view, err := service.RemoveProductImage(context.Background(), created.Product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Product.ImageKey != "" || view.ImageURL != "" {
		t.Errorf("image should be gone: %+v", view)
	}
	if _, stillThere := storage.Objects[withImage.Product.ImageKey]; stillThere {
		t.Error("the object must be deleted")
	}

	// Removing again is a harmless no-op.
	if _, err := service.RemoveProductImage(context.Background(), created.Product.ID); err != nil {
		t.Errorf("second removal: %v", err)
	}
	if _, err := service.RemoveProductImage(context.Background(), "missing"); !errors.Is(err, product.ErrNotFound) {
		t.Errorf("missing product: error = %v", err)
	}
}
