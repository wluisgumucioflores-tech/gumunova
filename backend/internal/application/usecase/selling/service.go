// Package selling contains the use cases for registering and consulting sales.
package selling

import (
	"context"
	"errors"
	"regexp"
	"time"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/sale"
)

const (
	defaultPageSize = 25
	maxPageSize     = 200
)

// ErrInvalidIdempotencyKey is returned when the key is missing or malformed.
var ErrInvalidIdempotencyKey = errors.New("invalid idempotency key")

var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,100}$`)

type Service struct {
	sales ports.SaleRepository
	now   func() time.Time
}

func NewService(sales ports.SaleRepository, now func() time.Time) *Service {
	return &Service{sales: sales, now: now}
}

type RegisterResult struct {
	Sale     sale.Sale
	Replayed bool
}

// RegisterSale validates and confirms a sale. The idempotency key makes retries safe: repeating
// the same request returns the sale already registered. Prices, costs and stock are decided
// inside the repository's transaction, against the stock it locks, never from client values.
func (s *Service) RegisterSale(ctx context.Context, draft sale.Draft, idempotencyKey string) (RegisterResult, error) {
	if !idempotencyKeyPattern.MatchString(idempotencyKey) {
		return RegisterResult{}, ErrInvalidIdempotencyKey
	}
	fingerprint := draft.Fingerprint()

	built, err := sale.New(draft, s.now())
	if err != nil {
		return RegisterResult{}, err
	}
	stored, err := s.sales.Register(ctx, built, ports.IdempotencyKey{Key: idempotencyKey, Fingerprint: fingerprint})
	if err != nil {
		return RegisterResult{}, err
	}
	return RegisterResult{Sale: stored.Sale, Replayed: stored.Replayed}, nil
}

func (s *Service) GetSale(ctx context.Context, id string) (sale.Sale, error) {
	return s.sales.GetByID(ctx, id)
}

type ListResult struct {
	Items  []ports.SaleSummary
	Total  int
	Limit  int
	Offset int
}

func (s *Service) ListSales(ctx context.Context, filter ports.SaleFilter) (ListResult, error) {
	if filter.Limit <= 0 {
		filter.Limit = defaultPageSize
	}
	if filter.Limit > maxPageSize {
		filter.Limit = maxPageSize
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	page, err := s.sales.List(ctx, filter)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Items: page.Items, Total: page.Total, Limit: filter.Limit, Offset: filter.Offset}, nil
}
