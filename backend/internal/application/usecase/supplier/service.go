// Package supplier contains the use cases for managing suppliers.
package supplier

import (
	"context"

	"gumunova/backend/internal/application/ports"
	domain "gumunova/backend/internal/domain/supplier"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

type ListResult struct {
	Items  []domain.Supplier
	Total  int
	Limit  int
	Offset int
}

type Service struct {
	suppliers ports.SupplierRepository
}

func NewService(suppliers ports.SupplierRepository) *Service {
	return &Service{suppliers: suppliers}
}

func (s *Service) CreateSupplier(ctx context.Context, draft domain.Draft) (domain.Supplier, error) {
	created, err := domain.New(draft)
	if err != nil {
		return domain.Supplier{}, err
	}
	return s.suppliers.Create(ctx, created)
}

func (s *Service) UpdateSupplier(ctx context.Context, id string, draft domain.Draft) (domain.Supplier, error) {
	existing, err := s.suppliers.GetByID(ctx, id)
	if err != nil {
		return domain.Supplier{}, err
	}
	if err := existing.Revise(draft); err != nil {
		return domain.Supplier{}, err
	}
	return s.suppliers.Update(ctx, existing)
}

func (s *Service) GetSupplier(ctx context.Context, id string) (domain.Supplier, error) {
	return s.suppliers.GetByID(ctx, id)
}

func (s *Service) ListSuppliers(ctx context.Context, filter ports.SupplierFilter) (ListResult, error) {
	if filter.Limit <= 0 {
		filter.Limit = defaultPageSize
	}
	if filter.Limit > maxPageSize {
		filter.Limit = maxPageSize
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	page, err := s.suppliers.List(ctx, filter)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Items: page.Items, Total: page.Total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

// DeactivateSupplier hides a supplier from new purchases while keeping its history.
func (s *Service) DeactivateSupplier(ctx context.Context, id string) (domain.Supplier, error) {
	return s.changeActive(ctx, id, (*domain.Supplier).Deactivate)
}

func (s *Service) ActivateSupplier(ctx context.Context, id string) (domain.Supplier, error) {
	return s.changeActive(ctx, id, (*domain.Supplier).Activate)
}

func (s *Service) changeActive(ctx context.Context, id string, change func(*domain.Supplier)) (domain.Supplier, error) {
	existing, err := s.suppliers.GetByID(ctx, id)
	if err != nil {
		return domain.Supplier{}, err
	}
	change(&existing)
	return s.suppliers.Update(ctx, existing)
}
