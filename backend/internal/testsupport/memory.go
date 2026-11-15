// Package testsupport provides in-memory implementations of the application ports.
// They exist only to keep unit tests fast and independent of PostgreSQL and S3.
package testsupport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/inventory"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/purchase"
	"gumunova/backend/internal/domain/sale"
	"gumunova/backend/internal/domain/shared/validation"
	"gumunova/backend/internal/domain/supplier"
)

// MemoryProductRepository is a ports.ProductRepository kept in a map.
type MemoryProductRepository struct {
	mutex  sync.Mutex
	nextID int

	Products        map[string]product.Product
	FailSetImageKey error
}

var _ ports.ProductRepository = (*MemoryProductRepository)(nil)

func NewMemoryProductRepository() *MemoryProductRepository {
	return &MemoryProductRepository{Products: make(map[string]product.Product)}
}

func (r *MemoryProductRepository) Create(_ context.Context, p product.Product) (product.Product, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, existing := range r.Products {
		if existing.SKU == p.SKU {
			return product.Product{}, product.ErrSKUAlreadyExists
		}
	}
	r.nextID++
	p.ID = fmt.Sprintf("id-%d", r.nextID)
	p.LabelCode = fmt.Sprintf("P-%012d", r.nextID)
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	r.Products[p.ID] = p
	return p, nil
}

func (r *MemoryProductRepository) Update(_ context.Context, p product.Product) (product.Product, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	stored, ok := r.Products[p.ID]
	if !ok {
		return product.Product{}, product.ErrNotFound
	}
	p.ImageKey = stored.ImageKey
	p.LabelCode = stored.LabelCode
	p.CreatedAt = stored.CreatedAt
	p.UpdatedAt = time.Now().UTC()
	r.Products[p.ID] = p
	return p, nil
}

func (r *MemoryProductRepository) SetImageKey(_ context.Context, id, key string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.FailSetImageKey != nil {
		return r.FailSetImageKey
	}
	p, ok := r.Products[id]
	if !ok {
		return product.ErrNotFound
	}
	p.ImageKey = key
	r.Products[id] = p
	return nil
}

func (r *MemoryProductRepository) ResetLabelCode(_ context.Context, id string) (product.Product, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	p, ok := r.Products[id]
	if !ok {
		return product.Product{}, product.ErrNotFound
	}
	r.nextID++
	p.LabelCode = fmt.Sprintf("P-%012d", r.nextID)
	r.Products[id] = p
	return p, nil
}

func (r *MemoryProductRepository) GetByLabelCode(_ context.Context, code string) (product.Product, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, p := range r.Products {
		if p.LabelCode == code {
			return p, nil
		}
	}
	return product.Product{}, product.ErrNotFound
}

func (r *MemoryProductRepository) GetByID(_ context.Context, id string) (product.Product, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	p, ok := r.Products[id]
	if !ok {
		return product.Product{}, product.ErrNotFound
	}
	return p, nil
}

func (r *MemoryProductRepository) List(_ context.Context, filter ports.ProductFilter) (ports.ProductPage, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	query := strings.ToLower(strings.TrimSpace(filter.Query))
	var matches []product.Product
	for _, p := range r.Products {
		if filter.Type != "" && p.Type != filter.Type {
			continue
		}
		if filter.IsActive != nil && p.IsActive != *filter.IsActive {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(p.SKU), query) && !strings.Contains(strings.ToLower(p.Name), query) {
			continue
		}
		matches = append(matches, p)
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Name != matches[j].Name {
			return matches[i].Name < matches[j].Name
		}
		return matches[i].SKU < matches[j].SKU
	})

	total := len(matches)
	start := min(filter.Offset, total)
	end := total
	if filter.Limit > 0 {
		end = min(start+filter.Limit, total)
	}
	return ports.ProductPage{Items: matches[start:end], Total: total}, nil
}

// MemoryImageStorage is a ports.ImageStorage kept in a map (key -> content type).
type MemoryImageStorage struct {
	mutex sync.Mutex

	Objects map[string]string
	Deleted []string
	FailPut error
}

var _ ports.ImageStorage = (*MemoryImageStorage)(nil)

func NewMemoryImageStorage() *MemoryImageStorage {
	return &MemoryImageStorage{Objects: make(map[string]string)}
}

func (s *MemoryImageStorage) Put(_ context.Context, key, contentType string, body io.Reader, _ int64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.FailPut != nil {
		return s.FailPut
	}
	if _, err := io.Copy(io.Discard, body); err != nil {
		return err
	}
	s.Objects[key] = contentType
	return nil
}

func (s *MemoryImageStorage) Delete(_ context.Context, key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.Objects, key)
	s.Deleted = append(s.Deleted, key)
	return nil
}

func (s *MemoryImageStorage) SignedURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://storage.test/" + key + "?signed=1", nil
}

// MemorySupplierRepository is a ports.SupplierRepository kept in a map.
type MemorySupplierRepository struct {
	mutex  sync.Mutex
	nextID int

	Suppliers map[string]supplier.Supplier
}

var _ ports.SupplierRepository = (*MemorySupplierRepository)(nil)

func NewMemorySupplierRepository() *MemorySupplierRepository {
	return &MemorySupplierRepository{Suppliers: make(map[string]supplier.Supplier)}
}

func (r *MemorySupplierRepository) Create(_ context.Context, s supplier.Supplier) (supplier.Supplier, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.nextID++
	s.ID = fmt.Sprintf("sup-%d", r.nextID)
	s.CreatedAt = time.Now().UTC()
	s.UpdatedAt = s.CreatedAt
	r.Suppliers[s.ID] = s
	return s, nil
}

func (r *MemorySupplierRepository) Update(_ context.Context, s supplier.Supplier) (supplier.Supplier, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	stored, ok := r.Suppliers[s.ID]
	if !ok {
		return supplier.Supplier{}, supplier.ErrNotFound
	}
	s.CreatedAt = stored.CreatedAt
	s.UpdatedAt = time.Now().UTC()
	r.Suppliers[s.ID] = s
	return s, nil
}

func (r *MemorySupplierRepository) GetByID(_ context.Context, id string) (supplier.Supplier, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	s, ok := r.Suppliers[id]
	if !ok {
		return supplier.Supplier{}, supplier.ErrNotFound
	}
	return s, nil
}

func (r *MemorySupplierRepository) List(_ context.Context, filter ports.SupplierFilter) (ports.SupplierPage, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	query := strings.ToLower(strings.TrimSpace(filter.Query))
	var matches []supplier.Supplier
	for _, s := range r.Suppliers {
		if filter.IsActive != nil && s.IsActive != *filter.IsActive {
			continue
		}
		haystack := strings.ToLower(s.Name + " " + s.Phone + " " + s.Email)
		if query != "" && !strings.Contains(haystack, query) {
			continue
		}
		matches = append(matches, s)
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Name != matches[j].Name {
			return matches[i].Name < matches[j].Name
		}
		return matches[i].ID < matches[j].ID
	})

	total := len(matches)
	start := min(filter.Offset, total)
	end := total
	if filter.Limit > 0 {
		end = min(start+filter.Limit, total)
	}
	return ports.SupplierPage{Items: matches[start:end], Total: total}, nil
}

// MemoryPurchaseRepository is a ports.PurchaseRepository kept in a map. It assigns IDs and
// sequential roll codes like the real adapter, and honors idempotency keys.
type MemoryPurchaseRepository struct {
	mutex     sync.Mutex
	nextID    int
	nextRoll  int
	suppliers *MemorySupplierRepository
	// Products is where products created by purchase lines are stored; set it in tests that
	// register such lines.
	Products *MemoryProductRepository

	Purchases map[string]purchase.Purchase
	Keys      map[string]memoryKey
	// RegisterCalls counts how many purchases were really persisted (replays do not count).
	RegisterCalls int
	FailRegister  error

	// ItemsLocked makes the repository behave as if stock from every purchase had been sold.
	ItemsLocked bool
	// UpdateCalls counts persisted edits; LastReplaceItems is what the last one asked for.
	UpdateCalls      int
	LastReplaceItems bool
}

type memoryKey struct {
	fingerprint string
	purchaseID  string
}

var _ ports.PurchaseRepository = (*MemoryPurchaseRepository)(nil)

func NewMemoryPurchaseRepository(suppliers *MemorySupplierRepository) *MemoryPurchaseRepository {
	return &MemoryPurchaseRepository{
		suppliers: suppliers,
		Purchases: make(map[string]purchase.Purchase),
		Keys:      make(map[string]memoryKey),
	}
}

func (r *MemoryPurchaseRepository) Register(_ context.Context, p purchase.Purchase, key ports.IdempotencyKey) (ports.RegisterResult, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if existing, used := r.Keys[key.Key]; used {
		if existing.fingerprint != key.Fingerprint {
			return ports.RegisterResult{}, purchase.ErrIdempotencyKeyReused
		}
		return ports.RegisterResult{Purchase: r.Purchases[existing.purchaseID], Replayed: true}, nil
	}
	if r.FailRegister != nil {
		return ports.RegisterResult{}, r.FailRegister
	}

	lines := make([]purchase.Line, len(p.Lines))
	copy(lines, p.Lines)
	p.Lines = lines
	for lineIndex := range p.Lines {
		if p.Lines[lineIndex].NewProduct == nil {
			continue
		}
		created, err := r.Products.Create(context.Background(), *p.Lines[lineIndex].NewProduct)
		if errors.Is(err, product.ErrSKUAlreadyExists) {
			return ports.RegisterResult{}, &purchase.SKUConflictError{Line: lineIndex}
		}
		if err != nil {
			return ports.RegisterResult{}, err
		}
		p.Lines[lineIndex].ProductID = created.ID
		p.Lines[lineIndex].NewProduct = nil
	}

	r.nextID++
	p.ID = fmt.Sprintf("pur-%d", r.nextID)
	p.CreatedAt = time.Now().UTC()
	p.Revision = 1
	for lineIndex := range p.Lines {
		p.Lines[lineIndex].ID = fmt.Sprintf("%s-line-%d", p.ID, lineIndex+1)
		for rollIndex := range p.Lines[lineIndex].Rolls {
			r.nextRoll++
			p.Lines[lineIndex].Rolls[rollIndex].ID = fmt.Sprintf("roll-%d", r.nextRoll)
			p.Lines[lineIndex].Rolls[rollIndex].Code = fmt.Sprintf("ROL-%06d", r.nextRoll)
		}
	}
	r.Purchases[p.ID] = p
	r.Keys[key.Key] = memoryKey{fingerprint: key.Fingerprint, purchaseID: p.ID}
	r.RegisterCalls++
	return ports.RegisterResult{Purchase: p}, nil
}

func (r *MemoryPurchaseRepository) Update(_ context.Context, id string, p purchase.Purchase, replaceItems bool) (purchase.Purchase, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	current, ok := r.Purchases[id]
	if !ok {
		return purchase.Purchase{}, purchase.ErrNotFound
	}
	if replaceItems && r.ItemsLocked {
		return purchase.Purchase{}, purchase.ErrItemsLocked
	}

	current.SupplierID, current.PurchaseDate = p.SupplierID, p.PurchaseDate
	current.DocumentReference, current.Notes = p.DocumentReference, p.Notes
	if replaceItems {
		lines := make([]purchase.Line, len(p.Lines))
		copy(lines, p.Lines)
		for lineIndex := range lines {
			if lines[lineIndex].NewProduct != nil {
				created, err := r.Products.Create(context.Background(), *lines[lineIndex].NewProduct)
				if errors.Is(err, product.ErrSKUAlreadyExists) {
					return purchase.Purchase{}, &purchase.SKUConflictError{Line: lineIndex}
				}
				if err != nil {
					return purchase.Purchase{}, err
				}
				lines[lineIndex].ProductID = created.ID
				lines[lineIndex].NewProduct = nil
			}
			lines[lineIndex].ID = fmt.Sprintf("%s-line-%d-r%d", id, lineIndex+1, current.Revision+1)
		}
		current.Lines, current.AdditionalCosts, current.Total = lines, p.AdditionalCosts, p.Total
		current.Revision++
	}
	r.Purchases[id] = current
	r.UpdateCalls++
	r.LastReplaceItems = replaceItems
	return current, nil
}

func (r *MemoryPurchaseRepository) ItemsEditable(_ context.Context, id string) (bool, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if _, ok := r.Purchases[id]; !ok {
		return false, purchase.ErrNotFound
	}
	return !r.ItemsLocked, nil
}

func (r *MemoryPurchaseRepository) GetByID(_ context.Context, id string) (purchase.Purchase, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	p, ok := r.Purchases[id]
	if !ok {
		return purchase.Purchase{}, purchase.ErrNotFound
	}
	return p, nil
}

func (r *MemoryPurchaseRepository) List(ctx context.Context, filter ports.PurchaseFilter) (ports.PurchasePage, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	query := strings.ToLower(strings.TrimSpace(filter.Query))
	var matches []ports.PurchaseSummary
	for _, p := range r.Purchases {
		if filter.SupplierID != "" && p.SupplierID != filter.SupplierID {
			continue
		}
		if filter.From != nil && p.PurchaseDate.Before(*filter.From) {
			continue
		}
		if filter.To != nil && p.PurchaseDate.After(*filter.To) {
			continue
		}
		name := ""
		if owner, err := r.suppliers.GetByID(ctx, p.SupplierID); err == nil {
			name = owner.Name
		}
		if query != "" && !strings.Contains(strings.ToLower(p.DocumentReference), query) && !strings.Contains(strings.ToLower(name), query) {
			continue
		}
		matches = append(matches, ports.PurchaseSummary{
			ID: p.ID, SupplierID: p.SupplierID, SupplierName: name, PurchaseDate: p.PurchaseDate,
			DocumentReference: p.DocumentReference, Total: p.Total, LineCount: len(p.Lines), CreatedAt: p.CreatedAt,
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		if !matches[i].PurchaseDate.Equal(matches[j].PurchaseDate) {
			return matches[i].PurchaseDate.After(matches[j].PurchaseDate)
		}
		return matches[i].ID > matches[j].ID
	})

	total := len(matches)
	start := min(filter.Offset, total)
	end := total
	if filter.Limit > 0 {
		end = min(start+filter.Limit, total)
	}
	return ports.PurchasePage{Items: matches[start:end], Total: total}, nil
}

// MemoryInventoryRepository is a ports.InventoryRepository over fixed data. It only applies
// paging: filtering is the database's job and is tested against PostgreSQL.
type MemoryInventoryRepository struct {
	Stock     []ports.StockItem
	Rolls     []ports.RollItem
	Movements []ports.MovementItem
	// ResetCalls counts the label code regenerations of rolls.
	ResetCalls int

	// Filters received by the last calls, so tests can check what the use case passed down.
	LastStockFilter    ports.StockFilter
	LastRollFilter     ports.RollFilter
	LastMovementFilter ports.MovementFilter
}

var _ ports.InventoryRepository = (*MemoryInventoryRepository)(nil)

func pageBounds(total, limit, offset int) (int, int) {
	start := min(offset, total)
	return start, min(start+limit, total)
}

func (r *MemoryInventoryRepository) ListStock(_ context.Context, filter ports.StockFilter) (ports.StockPage, error) {
	r.LastStockFilter = filter
	start, end := pageBounds(len(r.Stock), filter.Limit, filter.Offset)
	return ports.StockPage{Items: r.Stock[start:end], Total: len(r.Stock)}, nil
}

func (r *MemoryInventoryRepository) ListRolls(_ context.Context, filter ports.RollFilter) (ports.RollPage, error) {
	r.LastRollFilter = filter
	start, end := pageBounds(len(r.Rolls), filter.Limit, filter.Offset)
	return ports.RollPage{Items: r.Rolls[start:end], Total: len(r.Rolls)}, nil
}

func (r *MemoryInventoryRepository) GetRollByLabelCode(_ context.Context, code string) (ports.RollItem, error) {
	for _, roll := range r.Rolls {
		if roll.LabelCode == code {
			return roll, nil
		}
	}
	return ports.RollItem{}, inventory.ErrRollNotFound
}

func (r *MemoryInventoryRepository) ResetRollLabelCode(_ context.Context, id string) (ports.RollItem, error) {
	for index, roll := range r.Rolls {
		if roll.ID == id {
			r.ResetCalls++
			r.Rolls[index].LabelCode = fmt.Sprintf("R-RESET-%d", r.ResetCalls)
			return r.Rolls[index], nil
		}
	}
	return ports.RollItem{}, inventory.ErrRollNotFound
}

func (r *MemoryInventoryRepository) GetRoll(_ context.Context, id string) (ports.RollItem, error) {
	for _, roll := range r.Rolls {
		if roll.ID == id {
			return roll, nil
		}
	}
	return ports.RollItem{}, inventory.ErrRollNotFound
}

func (r *MemoryInventoryRepository) ListMovements(_ context.Context, filter ports.MovementFilter) (ports.MovementPage, error) {
	r.LastMovementFilter = filter
	start, end := pageBounds(len(r.Movements), filter.Limit, filter.Offset)
	return ports.MovementPage{Items: r.Movements[start:end], Total: len(r.Movements)}, nil
}

// MemorySaleRepository is a ports.SaleRepository kept in a map. It prices and checks sales with
// the same domain rules as the real adapter (sale.Sale.Allocate) over the Inventory it holds, and
// honors idempotency keys. It does not lock anything: concurrency is tested against PostgreSQL.
type MemorySaleRepository struct {
	mutex  sync.Mutex
	nextID int

	// Inventory is the stock sales take from; tests fill it in.
	Inventory *sale.Inventory
	Sales     map[string]sale.Sale
	Keys      map[string]memoryKey
	// RegisterCalls counts how many sales were really persisted (replays do not count).
	RegisterCalls int
}

var _ ports.SaleRepository = (*MemorySaleRepository)(nil)

func NewMemorySaleRepository() *MemorySaleRepository {
	return &MemorySaleRepository{
		Inventory: &sale.Inventory{
			Rolls:         make(map[string]*sale.RollStock),
			Tablecloths:   make(map[string]*sale.TableclothStock),
			NotTablecloth: make(map[string]bool),
		},
		Sales: make(map[string]sale.Sale),
		Keys:  make(map[string]memoryKey),
	}
}

func (r *MemorySaleRepository) Register(_ context.Context, s sale.Sale, key ports.IdempotencyKey) (ports.SaleRegisterResult, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if existing, used := r.Keys[key.Key]; used {
		if existing.fingerprint != key.Fingerprint {
			return ports.SaleRegisterResult{}, sale.ErrIdempotencyKeyReused
		}
		return ports.SaleRegisterResult{Sale: r.Sales[existing.purchaseID], Replayed: true}, nil
	}

	lines := make([]sale.Line, len(s.Lines))
	copy(lines, s.Lines)
	s.Lines = lines
	if problems := s.Allocate(r.Inventory); len(problems) > 0 {
		return ports.SaleRegisterResult{}, &validation.ValidationError{Fields: problems}
	}

	r.nextID++
	s.ID = fmt.Sprintf("sale-%d", r.nextID)
	s.CreatedAt = time.Now().UTC()
	for index := range s.Lines {
		s.Lines[index].ID = fmt.Sprintf("%s-line-%d", s.ID, index+1)
	}
	r.Sales[s.ID] = s
	r.Keys[key.Key] = memoryKey{fingerprint: key.Fingerprint, purchaseID: s.ID}
	r.RegisterCalls++
	return ports.SaleRegisterResult{Sale: s}, nil
}

func (r *MemorySaleRepository) GetByID(_ context.Context, id string) (sale.Sale, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	found, ok := r.Sales[id]
	if !ok {
		return sale.Sale{}, sale.ErrNotFound
	}
	return found, nil
}

func (r *MemorySaleRepository) List(_ context.Context, filter ports.SaleFilter) (ports.SalePage, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var matches []ports.SaleSummary
	for _, s := range r.Sales {
		if filter.From != nil && s.SaleDate.Before(*filter.From) {
			continue
		}
		if filter.To != nil && s.SaleDate.After(*filter.To) {
			continue
		}
		matches = append(matches, ports.SaleSummary{
			ID: s.ID, SaleDate: s.SaleDate, Total: s.Total, TotalCost: s.TotalCost, LineCount: len(s.Lines), CreatedAt: s.CreatedAt,
		})
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	start, end := pageBounds(len(matches), filter.Limit, filter.Offset)
	return ports.SalePage{Items: matches[start:end], Total: len(matches)}, nil
}
