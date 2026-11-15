// Package inventoryquery holds the inventory use cases that never move stock: stock per variant,
// physical rolls, the movement ledger and the regeneration of a roll's label code.
package inventoryquery

import (
	"context"

	"gumunova/backend/internal/application/ports"
)

const (
	defaultPageSize = 25
	maxPageSize     = 200
)

type Service struct {
	inventory ports.InventoryRepository
}

func NewService(inventory ports.InventoryRepository) *Service {
	return &Service{inventory: inventory}
}

// Page is the paging that was applied, echoed back to the caller.
type Page struct {
	Limit  int
	Offset int
}

func normalizePage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

type StockResult struct {
	ports.StockPage
	Page
}

func (s *Service) ListStock(ctx context.Context, filter ports.StockFilter) (StockResult, error) {
	filter.Limit, filter.Offset = normalizePage(filter.Limit, filter.Offset)
	page, err := s.inventory.ListStock(ctx, filter)
	return StockResult{StockPage: page, Page: Page{filter.Limit, filter.Offset}}, err
}

type RollsResult struct {
	ports.RollPage
	Page
}

func (s *Service) ListRolls(ctx context.Context, filter ports.RollFilter) (RollsResult, error) {
	filter.Limit, filter.Offset = normalizePage(filter.Limit, filter.Offset)
	if filter.Status == "" {
		filter.Status = ports.RollStatusAvailable
	}
	page, err := s.inventory.ListRolls(ctx, filter)
	return RollsResult{RollPage: page, Page: Page{filter.Limit, filter.Offset}}, err
}

func (s *Service) GetRoll(ctx context.Context, id string) (ports.RollItem, error) {
	return s.inventory.GetRoll(ctx, id)
}

// GetRollByLabelCode finds the roll whose label carries the code.
func (s *Service) GetRollByLabelCode(ctx context.Context, code string) (ports.RollItem, error) {
	return s.inventory.GetRollByLabelCode(ctx, code)
}

// ResetRollLabelCode gives the roll a new label code. Labels printed with the old code stop
// identifying it, so they must be printed again. It does not change stock.
func (s *Service) ResetRollLabelCode(ctx context.Context, id string) (ports.RollItem, error) {
	return s.inventory.ResetRollLabelCode(ctx, id)
}

type MovementsResult struct {
	ports.MovementPage
	Page
}

func (s *Service) ListMovements(ctx context.Context, filter ports.MovementFilter) (MovementsResult, error) {
	filter.Limit, filter.Offset = normalizePage(filter.Limit, filter.Offset)
	page, err := s.inventory.ListMovements(ctx, filter)
	return MovementsResult{MovementPage: page, Page: Page{filter.Limit, filter.Offset}}, err
}
