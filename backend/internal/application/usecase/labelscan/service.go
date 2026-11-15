// Package labelscan resolves the code read from a printed label (QR or barcode) into what it
// identifies: a product, or one physical PVC roll together with its product.
package labelscan

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/application/usecase/catalog"
	"gumunova/backend/internal/application/usecase/inventoryquery"
	"gumunova/backend/internal/domain/inventory"
	"gumunova/backend/internal/domain/product"
)

// ErrNotFound means the code identifies nothing: it is malformed, unknown, or it was
// regenerated (so the label that carries it is out of date).
var ErrNotFound = errors.New("label code not found")

type Kind string

const (
	KindProduct Kind = "product"
	KindRoll    Kind = "roll"
)

// Codes are P- (product) or R- (roll) followed by 12 hexadecimal characters.
var codePattern = regexp.MustCompile(`^([PR])-[0-9A-F]{12}$`)

// Result is what a label identifies. A roll always comes with its product, which holds the prices.
type Result struct {
	Kind    Kind
	Product catalog.ProductView
	Roll    *ports.RollItem
}

type Service struct {
	catalog   *catalog.Service
	inventory *inventoryquery.Service
}

func NewService(catalog *catalog.Service, inventory *inventoryquery.Service) *Service {
	return &Service{catalog: catalog, inventory: inventory}
}

// Resolve reads a scanned code. Whitespace around it and letter case are ignored, because
// scanners and keyboards add line breaks and may change case.
func (s *Service) Resolve(ctx context.Context, code string) (Result, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	match := codePattern.FindStringSubmatch(code)
	if match == nil {
		return Result{}, ErrNotFound
	}

	if match[1] == "P" {
		view, err := s.catalog.GetProductByLabelCode(ctx, code)
		if errors.Is(err, product.ErrNotFound) {
			return Result{}, ErrNotFound
		}
		if err != nil {
			return Result{}, err
		}
		return Result{Kind: KindProduct, Product: view}, nil
	}

	roll, err := s.inventory.GetRollByLabelCode(ctx, code)
	if errors.Is(err, inventory.ErrRollNotFound) {
		return Result{}, ErrNotFound
	}
	if err != nil {
		return Result{}, err
	}
	view, err := s.catalog.GetProduct(ctx, roll.ProductID)
	if err != nil {
		return Result{}, err
	}
	return Result{Kind: KindRoll, Product: view, Roll: &roll}, nil
}
