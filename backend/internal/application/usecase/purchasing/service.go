// Package purchasing contains the use cases for registering and consulting purchases.
package purchasing

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/purchase"
	"gumunova/backend/internal/domain/shared/validation"
	"gumunova/backend/internal/domain/supplier"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// ErrInvalidIdempotencyKey is returned when the key is missing or malformed.
var ErrInvalidIdempotencyKey = errors.New("invalid idempotency key")

var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,100}$`)

// View is a purchase together with the supplier (nil when it has none) and products it refers to.
type View struct {
	Purchase purchase.Purchase
	Supplier *supplier.Supplier
	Products map[string]product.Product
	// ItemsEditable is true while the lines and rolls can still be replaced.
	ItemsEditable bool
}

type RegisterResult struct {
	View     View
	Replayed bool
}

type ListResult struct {
	Items  []ports.PurchaseSummary
	Total  int
	Limit  int
	Offset int
}

type Service struct {
	purchases ports.PurchaseRepository
	suppliers ports.SupplierRepository
	products  ports.ProductRepository
	now       func() time.Time
}

// NewService builds the service. now returns the current instant (time.Now in production).
func NewService(
	purchases ports.PurchaseRepository,
	suppliers ports.SupplierRepository,
	products ports.ProductRepository,
	now func() time.Time,
) *Service {
	return &Service{purchases: purchases, suppliers: suppliers, products: products, now: now}
}

// RegisterPurchase validates and confirms a purchase. The idempotency key makes retries safe:
// repeating the same request returns the purchase already registered.
func (s *Service) RegisterPurchase(ctx context.Context, draft purchase.Draft, idempotencyKey string) (RegisterResult, error) {
	if !idempotencyKeyPattern.MatchString(idempotencyKey) {
		return RegisterResult{}, ErrInvalidIdempotencyKey
	}
	fingerprint := draft.Fingerprint()

	// Never trust what the client says about products: the catalog decides each line's kind.
	problems, products, err := s.resolveReferences(ctx, &draft, nil)
	if err != nil {
		return RegisterResult{}, err
	}
	if len(problems) > 0 {
		return RegisterResult{}, &validation.ValidationError{Fields: problems}
	}

	built, err := purchase.New(draft, s.now())
	if err != nil {
		return RegisterResult{}, err
	}

	stored, err := s.purchases.Register(ctx, built, ports.IdempotencyKey{Key: idempotencyKey, Fingerprint: fingerprint})
	var skuTaken *purchase.SKUConflictError
	if errors.As(err, &skuTaken) {
		field := fmt.Sprintf("lines[%d].new_product.sku", skuTaken.Line)
		return RegisterResult{}, &validation.ValidationError{Fields: []validation.FieldError{{Field: field, Code: validation.CodeAlreadyExists}}}
	}
	if err != nil {
		return RegisterResult{}, err
	}
	view, err := s.buildView(ctx, stored.Purchase, products)
	if err != nil {
		return RegisterResult{}, err
	}
	return RegisterResult{View: view, Replayed: stored.Replayed}, nil
}

// UpdatePurchase corrects a confirmed purchase. The supplier, date, reference and notes can
// always change. The items (lines, rolls, costs) are replaced only when they differ from what
// is stored, and only while nothing they brought in has been sold: otherwise it returns
// purchase.ErrItemsLocked and changes nothing. Repeating the same request is harmless, because
// an edit that changes nothing is a no-op.
func (s *Service) UpdatePurchase(ctx context.Context, id string, draft purchase.Draft) (View, error) {
	existing, err := s.purchases.GetByID(ctx, id)
	if err != nil {
		return View{}, err
	}
	problems, products, err := s.resolveReferences(ctx, &draft, &existing)
	if err != nil {
		return View{}, err
	}
	if len(problems) > 0 {
		return View{}, &validation.ValidationError{Fields: problems}
	}
	built, err := purchase.New(draft, s.now())
	if err != nil {
		return View{}, err
	}

	stored, err := s.purchases.Update(ctx, id, built, !purchase.SameItems(existing, built))
	var skuTaken *purchase.SKUConflictError
	if errors.As(err, &skuTaken) {
		field := fmt.Sprintf("lines[%d].new_product.sku", skuTaken.Line)
		return View{}, &validation.ValidationError{Fields: []validation.FieldError{{Field: field, Code: validation.CodeAlreadyExists}}}
	}
	if err != nil {
		return View{}, err
	}
	return s.buildView(ctx, stored, products)
}

// resolveReferences checks that the supplier and every product exist and can be used, and
// fills in each line's product type from the catalog. When editing, existing is the stored
// purchase: its supplier and products may have been deactivated since, and keeping them is allowed.
func (s *Service) resolveReferences(ctx context.Context, draft *purchase.Draft, existing *purchase.Purchase) ([]validation.FieldError, map[string]product.Product, error) {
	var problems []validation.FieldError

	if draft.SupplierID != "" {
		found, err := s.suppliers.GetByID(ctx, draft.SupplierID)
		switch {
		case errors.Is(err, supplier.ErrNotFound):
			problems = append(problems, validation.FieldError{Field: "supplier_id", Code: validation.CodeNotFound})
		case err != nil:
			return nil, nil, fmt.Errorf("load supplier: %w", err)
		case !found.IsActive && (existing == nil || existing.SupplierID != draft.SupplierID):
			problems = append(problems, validation.FieldError{Field: "supplier_id", Code: validation.CodeInactive})
		}
	}

	products := make(map[string]product.Product)
	missing := make(map[string]bool)
	for index := range draft.Lines {
		line := &draft.Lines[index]
		field := fmt.Sprintf("lines[%d].product_id", index)
		if line.ProductID == "" {
			continue // the domain reports the missing id
		}
		if missing[line.ProductID] {
			problems = append(problems, validation.FieldError{Field: field, Code: validation.CodeNotFound})
			continue
		}
		found, cached := products[line.ProductID]
		if !cached {
			var err error
			found, err = s.products.GetByID(ctx, line.ProductID)
			if errors.Is(err, product.ErrNotFound) {
				missing[line.ProductID] = true
				problems = append(problems, validation.FieldError{Field: field, Code: validation.CodeNotFound})
				continue
			}
			if err != nil {
				return nil, nil, fmt.Errorf("load product: %w", err)
			}
			products[line.ProductID] = found
		}
		if !found.IsActive && !usesProduct(existing, found.ID) {
			problems = append(problems, validation.FieldError{Field: field, Code: validation.CodeInactive})
		}
		line.ProductType = found.Type
	}
	return problems, products, nil
}

// usesProduct reports whether the stored purchase already has a line for the product.
func usesProduct(existing *purchase.Purchase, productID string) bool {
	if existing == nil {
		return false
	}
	for _, line := range existing.Lines {
		if line.ProductID == productID {
			return true
		}
	}
	return false
}

func (s *Service) GetPurchase(ctx context.Context, id string) (View, error) {
	found, err := s.purchases.GetByID(ctx, id)
	if err != nil {
		return View{}, err
	}
	return s.buildView(ctx, found, nil)
}

func (s *Service) ListPurchases(ctx context.Context, filter ports.PurchaseFilter) (ListResult, error) {
	if filter.Limit <= 0 {
		filter.Limit = defaultPageSize
	}
	if filter.Limit > maxPageSize {
		filter.Limit = maxPageSize
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	page, err := s.purchases.List(ctx, filter)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Items: page.Items, Total: page.Total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

// buildView attaches the supplier and products. known holds products already loaded.
func (s *Service) buildView(ctx context.Context, p purchase.Purchase, known map[string]product.Product) (View, error) {
	var owner *supplier.Supplier
	if p.SupplierID != "" {
		found, err := s.suppliers.GetByID(ctx, p.SupplierID)
		if err != nil {
			return View{}, fmt.Errorf("load supplier of purchase: %w", err)
		}
		owner = &found
	}
	products := make(map[string]product.Product)
	for _, line := range p.Lines {
		if _, done := products[line.ProductID]; done {
			continue
		}
		if cached, ok := known[line.ProductID]; ok {
			products[line.ProductID] = cached
			continue
		}
		found, err := s.products.GetByID(ctx, line.ProductID)
		if err != nil {
			return View{}, fmt.Errorf("load product of purchase: %w", err)
		}
		products[line.ProductID] = found
	}
	editable, err := s.purchases.ItemsEditable(ctx, p.ID)
	if err != nil {
		return View{}, fmt.Errorf("check purchase items: %w", err)
	}
	return View{Purchase: p, Supplier: owner, Products: products, ItemsEditable: editable}, nil
}
