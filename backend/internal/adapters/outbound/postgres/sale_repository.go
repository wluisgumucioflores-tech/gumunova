package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/inventory"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/sale"
	"gumunova/backend/internal/domain/shared/decimal"
	"gumunova/backend/internal/domain/shared/validation"
)

type SaleRepository struct {
	pool *pgxpool.Pool
}

var _ ports.SaleRepository = (*SaleRepository)(nil)

func NewSaleRepository(pool *pgxpool.Pool) *SaleRepository {
	return &SaleRepository{pool: pool}
}

// Register confirms the sale in one transaction.
func (r *SaleRepository) Register(ctx context.Context, s sale.Sale, key ports.IdempotencyKey) (ports.SaleRegisterResult, error) {
	transaction, err := r.pool.Begin(ctx)
	if err != nil {
		return ports.SaleRegisterResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer transaction.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	// A unique key makes this insert the gate: concurrent requests with the same key queue here,
	// and only one of them goes on. The others then see the stored sale and replay it. The gate
	// comes before the stock checks so a retry of a confirmed sale is never re-checked against
	// the stock that sale already took.
	var saleID string
	err = transaction.QueryRow(ctx,
		`INSERT INTO sales (sale_date, notes, status, idempotency_key, request_fingerprint)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (idempotency_key) DO NOTHING
		 RETURNING id::text`,
		s.SaleDate, s.Notes, string(s.Status), key.Key, key.Fingerprint,
	).Scan(&saleID)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = transaction.Rollback(ctx)
		return r.replay(ctx, key)
	}
	if err != nil {
		return ports.SaleRegisterResult{}, fmt.Errorf("insert sale: %w", err)
	}

	stock, err := lockInventory(ctx, transaction, s.Lines)
	if err != nil {
		return ports.SaleRegisterResult{}, err
	}
	if problems := s.Allocate(stock); len(problems) > 0 {
		return ports.SaleRegisterResult{}, &validation.ValidationError{Fields: problems}
	}

	if err := writeStock(ctx, transaction, stock); err != nil {
		return ports.SaleRegisterResult{}, err
	}
	for _, line := range s.Lines {
		if err := insertSaleLine(ctx, transaction, saleID, stock, line); err != nil {
			return ports.SaleRegisterResult{}, err
		}
	}
	if _, err := transaction.Exec(ctx,
		`UPDATE sales SET total_amount = $2::text::numeric, total_cost = $3::text::numeric WHERE id = $1`,
		saleID, s.Total.String(), s.TotalCost.String()); err != nil {
		return ports.SaleRegisterResult{}, fmt.Errorf("update sale totals: %w", err)
	}

	stored, err := getSale(ctx, transaction, saleID)
	if err != nil {
		return ports.SaleRegisterResult{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return ports.SaleRegisterResult{}, fmt.Errorf("commit transaction: %w", err)
	}
	return ports.SaleRegisterResult{Sale: stored}, nil
}

// replay answers a retry: the same key and request returns what was already registered.
func (r *SaleRepository) replay(ctx context.Context, key ports.IdempotencyKey) (ports.SaleRegisterResult, error) {
	var existingID, existingFingerprint string
	err := r.pool.QueryRow(ctx,
		`SELECT id::text, request_fingerprint FROM sales WHERE idempotency_key = $1`, key.Key,
	).Scan(&existingID, &existingFingerprint)
	if err != nil {
		return ports.SaleRegisterResult{}, fmt.Errorf("look up idempotency key: %w", err)
	}
	if existingFingerprint != key.Fingerprint {
		return ports.SaleRegisterResult{}, sale.ErrIdempotencyKeyReused
	}
	stored, err := getSale(ctx, r.pool, existingID)
	if err != nil {
		return ports.SaleRegisterResult{}, err
	}
	return ports.SaleRegisterResult{Sale: stored, Replayed: true}, nil
}

// lockInventory reads, and locks until the transaction ends, every roll and tablecloth stock row
// the sale takes from. Rows are locked in a fixed order, so two sales that touch the same stock
// in a different order wait for each other instead of deadlocking, and two sales can never both
// take the same meters or units.
func lockInventory(ctx context.Context, transaction pgx.Tx, lines []sale.Line) (*sale.Inventory, error) {
	stock := &sale.Inventory{
		Rolls:         make(map[string]*sale.RollStock),
		Tablecloths:   make(map[string]*sale.TableclothStock),
		NotTablecloth: make(map[string]bool),
	}

	rollIDs, productIDs := map[string]bool{}, map[string]bool{}
	for _, line := range lines {
		switch line.Kind {
		case sale.KindRollMeters, sale.KindFullRoll:
			if uuidPattern.MatchString(line.RollID) {
				rollIDs[line.RollID] = true
			}
		case sale.KindTablecloth:
			if uuidPattern.MatchString(line.ProductID) {
				productIDs[line.ProductID] = true
			}
		}
	}

	if len(rollIDs) > 0 {
		rows, err := transaction.Query(ctx,
			`SELECT r.id::text, r.roll_code, p.id::text, p.sku, p.name, p.is_active,
			        r.initial_usable_length_m::text, r.available_length_m::text,
			        (r.acquisition_cost + r.additional_cost)::text,
			        v.reference_price_per_meter::text, v.reference_price_per_roll::text
			 FROM pvc_rolls r
			 JOIN products p ON p.id = r.product_id
			 JOIN pvc_product_details v ON v.product_id = p.id
			 WHERE r.id = ANY($1::uuid[]) AND r.superseded_at IS NULL
			 ORDER BY r.id
			 FOR UPDATE OF r`, sortedKeys(rollIDs))
		if err != nil {
			return nil, fmt.Errorf("lock rolls: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var (
				roll                                                sale.RollStock
				initial, available, attributable, perMeter, perRoll string
			)
			if err := rows.Scan(&roll.ID, &roll.Code, &roll.ProductID, &roll.SKU, &roll.Name, &roll.ProductActive,
				&initial, &available, &attributable, &perMeter, &perRoll); err != nil {
				return nil, fmt.Errorf("scan roll: %w", err)
			}
			for _, field := range []struct {
				text   string
				target *decimal.Decimal
			}{
				{initial, &roll.InitialM}, {available, &roll.AvailableM}, {attributable, &roll.Attributable},
				{perMeter, &roll.ReferencePricePerMeter}, {perRoll, &roll.ReferencePricePerRoll},
			} {
				if *field.target, err = parseWide(field.text); err != nil {
					return nil, fmt.Errorf("roll %s: %w", roll.ID, err)
				}
			}
			stock.Rolls[roll.ID] = &roll
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("read rolls: %w", err)
		}
		rows.Close()
	}

	if len(productIDs) > 0 {
		// Lock the stock rows first, in order; products without a stock row are read afterwards.
		stockRows, err := transaction.Query(ctx,
			`SELECT product_id::text, quantity, stock_value::text FROM tablecloth_stock
			 WHERE product_id = ANY($1::uuid[]) ORDER BY product_id FOR UPDATE`, sortedKeys(productIDs))
		if err != nil {
			return nil, fmt.Errorf("lock tablecloth stock: %w", err)
		}
		defer stockRows.Close()
		units := map[string]int{}
		values := map[string]decimal.Decimal{}
		for stockRows.Next() {
			var (
				id    string
				count int
				value string
			)
			if err := stockRows.Scan(&id, &count, &value); err != nil {
				return nil, fmt.Errorf("scan tablecloth stock: %w", err)
			}
			parsed, err := parseWide(value)
			if err != nil {
				return nil, fmt.Errorf("stock value of %s: %w", id, err)
			}
			units[id], values[id] = count, parsed
		}
		if err := stockRows.Err(); err != nil {
			return nil, fmt.Errorf("read tablecloth stock: %w", err)
		}
		stockRows.Close()

		products, err := transaction.Query(ctx,
			`SELECT p.id::text, p.sku, p.name, p.product_type, p.is_active, COALESCE(t.unit_price::text, '')
			 FROM products p LEFT JOIN tablecloth_product_details t ON t.product_id = p.id
			 WHERE p.id = ANY($1::uuid[])`, sortedKeys(productIDs))
		if err != nil {
			return nil, fmt.Errorf("read products: %w", err)
		}
		defer products.Close()
		for products.Next() {
			var id, sku, name, kind, price string
			var active bool
			if err := products.Scan(&id, &sku, &name, &kind, &active, &price); err != nil {
				return nil, fmt.Errorf("scan product: %w", err)
			}
			if product.Type(kind) != product.TypeTablecloth {
				stock.NotTablecloth[id] = true
				continue
			}
			reference, err := decimal.Parse(price)
			if err != nil {
				return nil, fmt.Errorf("unit price of %s: %w", id, err)
			}
			stock.Tablecloths[id] = &sale.TableclothStock{
				ProductID: id, SKU: sku, Name: name, ProductActive: active,
				Units: units[id], Value: values[id], ReferencePrice: reference,
			}
		}
		if err := products.Err(); err != nil {
			return nil, fmt.Errorf("read products: %w", err)
		}
	}
	return stock, nil
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// writeStock stores the balances Allocate left in the inventory. The tablecloth stock row may
// not exist yet only for a variant that never received stock, and such a variant cannot be sold.
func writeStock(ctx context.Context, transaction pgx.Tx, stock *sale.Inventory) error {
	for _, id := range sortedRollIDs(stock) {
		roll := stock.Rolls[id]
		if _, err := transaction.Exec(ctx,
			`UPDATE pvc_rolls SET available_length_m = $2::text::numeric WHERE id = $1`,
			roll.ID, roll.AvailableM.String()); err != nil {
			return fmt.Errorf("update roll %s: %w", roll.Code, err)
		}
	}
	for id, cloth := range stock.Tablecloths {
		if _, err := transaction.Exec(ctx,
			`UPDATE tablecloth_stock SET quantity = $2, stock_value = $3::text::numeric, updated_at = now()
			 WHERE product_id = $1`,
			id, cloth.Units, cloth.Value.String()); err != nil {
			return fmt.Errorf("update tablecloth stock of %s: %w", cloth.SKU, err)
		}
	}
	return nil
}

func sortedRollIDs(stock *sale.Inventory) []string {
	ids := make([]string, 0, len(stock.Rolls))
	for id := range stock.Rolls {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func insertSaleLine(ctx context.Context, transaction pgx.Tx, saleID string, stock *sale.Inventory, line sale.Line) error {
	var rollID any
	if line.Kind != sale.KindTablecloth {
		rollID = line.RollID
	}
	var lineID string
	err := transaction.QueryRow(ctx,
		`INSERT INTO sale_lines
		   (sale_id, line_number, sale_kind, product_id, roll_id, quantity, unit_price, reference_price, line_total, cost_of_sale)
		 VALUES ($1, $2, $3, $4, $5, $6::text::numeric, $7::text::numeric, $8::text::numeric, $9::text::numeric, $10::text::numeric)
		 RETURNING id::text`,
		saleID, line.Number, string(line.Kind), line.ProductID, rollID, line.Sold().String(),
		line.UnitPrice.String(), line.ReferencePrice.String(), line.LineTotal.String(), line.CostOfSale.String(),
	).Scan(&lineID)
	if err != nil {
		return fmt.Errorf("insert sale line %d: %w", line.Number, err)
	}

	// One outflow movement per line, in meters for rolls (the whole length for a whole roll)
	// and in units for tablecloths.
	var (
		movementRoll any
		quantity     decimal.Decimal
		unit         inventory.Unit
	)
	switch line.Kind {
	case sale.KindRollMeters:
		movementRoll, quantity, unit = line.RollID, line.Meters, inventory.UnitMeter
	case sale.KindFullRoll:
		movementRoll, unit = line.RollID, inventory.UnitMeter
		// The roll is empty now; what left it is the length it was sold with.
		quantity = fullRollLength(stock, line)
	default:
		quantity, unit = line.Sold(), inventory.UnitPiece
	}
	_, err = transaction.Exec(ctx,
		`INSERT INTO inventory_movements
		   (product_id, roll_id, movement_type, quantity_delta, unit, reference_type, reference_id)
		 VALUES ($1, $2, $3, $4::text::numeric, $5, $6, $7)`,
		line.ProductID, movementRoll, string(inventory.MovementSaleOut), negate(quantity).String(), string(unit),
		inventory.ReferenceSale, saleID)
	if err != nil {
		return fmt.Errorf("insert inventory movement of line %d: %w", line.Number, err)
	}
	return nil
}

// fullRollLength is the length a whole-roll line took out of its roll: the roll's initial length.
func fullRollLength(stock *sale.Inventory, line sale.Line) decimal.Decimal {
	return stock.Rolls[line.RollID].InitialM
}

func negate(value decimal.Decimal) decimal.Decimal {
	return decimal.FromHundredths(-value.Hundredths())
}

func (r *SaleRepository) GetByID(ctx context.Context, id string) (sale.Sale, error) {
	if !uuidPattern.MatchString(id) {
		return sale.Sale{}, sale.ErrNotFound
	}
	return getSale(ctx, r.pool, id)
}

func getSale(ctx context.Context, q reader, id string) (sale.Sale, error) {
	var (
		s                   sale.Sale
		status, total, cost string
		saleDate, createdAt time.Time
	)
	err := q.QueryRow(ctx,
		`SELECT id::text, sale_date, notes, status, total_amount::text, total_cost::text, created_at
		 FROM sales WHERE id = $1`, id,
	).Scan(&s.ID, &saleDate, &s.Notes, &status, &total, &cost, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return sale.Sale{}, sale.ErrNotFound
	}
	if err != nil {
		return sale.Sale{}, fmt.Errorf("get sale: %w", err)
	}
	s.SaleDate, s.CreatedAt, s.Status = saleDate.UTC(), createdAt.UTC(), sale.Status(status)
	if s.Total, err = parseWide(total); err != nil {
		return sale.Sale{}, fmt.Errorf("total_amount: %w", err)
	}
	if s.TotalCost, err = parseWide(cost); err != nil {
		return sale.Sale{}, fmt.Errorf("total_cost: %w", err)
	}

	rows, err := q.Query(ctx,
		`SELECT l.id::text, l.line_number, l.sale_kind, p.id::text, p.sku, p.name,
		        COALESCE(l.roll_id::text, ''), COALESCE(r.roll_code, ''),
		        l.quantity::text, l.unit_price::text, l.reference_price::text, l.line_total::text, l.cost_of_sale::text
		 FROM sale_lines l
		 JOIN products p ON p.id = l.product_id
		 LEFT JOIN pvc_rolls r ON r.id = l.roll_id
		 WHERE l.sale_id = $1 ORDER BY l.line_number`, id)
	if err != nil {
		return sale.Sale{}, fmt.Errorf("get sale lines: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			line                                            sale.Line
			kind                                            string
			quantity, price, reference, lineTotal, lineCost string
		)
		if err := rows.Scan(&line.ID, &line.Number, &kind, &line.ProductID, &line.ProductSKU, &line.ProductName,
			&line.RollID, &line.RollCode, &quantity, &price, &reference, &lineTotal, &lineCost); err != nil {
			return sale.Sale{}, fmt.Errorf("scan sale line: %w", err)
		}
		line.Kind = sale.Kind(kind)
		quantityValue, err := parseWide(quantity)
		if err != nil {
			return sale.Sale{}, fmt.Errorf("quantity: %w", err)
		}
		switch line.Kind {
		case sale.KindRollMeters:
			line.Meters = quantityValue
		case sale.KindTablecloth:
			line.Quantity = int(quantityValue.Hundredths() / 100)
		}
		for _, field := range []struct {
			text   string
			target *decimal.Decimal
		}{{price, &line.UnitPrice}, {reference, &line.ReferencePrice}, {lineTotal, &line.LineTotal}, {lineCost, &line.CostOfSale}} {
			if *field.target, err = parseWide(field.text); err != nil {
				return sale.Sale{}, fmt.Errorf("sale line %d: %w", line.Number, err)
			}
		}
		s.Lines = append(s.Lines, line)
	}
	if err := rows.Err(); err != nil {
		return sale.Sale{}, fmt.Errorf("read sale lines: %w", err)
	}
	return s, nil
}

func (r *SaleRepository) List(ctx context.Context, filter ports.SaleFilter) (ports.SalePage, error) {
	var where conditions
	if filter.From != nil {
		where.add("s.sale_date >= $%d", *filter.From)
	}
	if filter.To != nil {
		where.add("s.sale_date <= $%d", *filter.To)
	}
	if query := strings.TrimSpace(filter.Query); query != "" {
		where.add(`EXISTS (
		    SELECT 1 FROM sale_lines l
		    JOIN products p ON p.id = l.product_id
		    LEFT JOIN pvc_rolls r ON r.id = l.roll_id
		    WHERE l.sale_id = s.id AND (p.sku ILIKE $%[1]d OR p.name ILIKE $%[1]d OR r.roll_code ILIKE $%[1]d))`,
			"%"+escapeLike(query)+"%")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM sales s"+where.where(), where.args...).Scan(&total); err != nil {
		return ports.SalePage{}, fmt.Errorf("count sales: %w", err)
	}

	args := append(append([]any{}, where.args...), filter.Limit, filter.Offset)
	rows, err := r.pool.Query(ctx,
		fmt.Sprintf(`SELECT s.id::text, s.sale_date, s.total_amount::text, s.total_cost::text,
		        (SELECT count(*) FROM sale_lines l WHERE l.sale_id = s.id), s.created_at
		 FROM sales s%s ORDER BY s.sale_date DESC, s.created_at DESC, s.id LIMIT $%d OFFSET $%d`,
			where.where(), len(where.args)+1, len(where.args)+2), args...)
	if err != nil {
		return ports.SalePage{}, fmt.Errorf("list sales: %w", err)
	}
	defer rows.Close()

	items := []ports.SaleSummary{}
	for rows.Next() {
		var (
			item                ports.SaleSummary
			saleDate, createdAt time.Time
			totalText, costText string
		)
		if err := rows.Scan(&item.ID, &saleDate, &totalText, &costText, &item.LineCount, &createdAt); err != nil {
			return ports.SalePage{}, fmt.Errorf("scan sale: %w", err)
		}
		item.SaleDate, item.CreatedAt = saleDate.UTC(), createdAt.UTC()
		if item.Total, err = parseWide(totalText); err != nil {
			return ports.SalePage{}, fmt.Errorf("total_amount: %w", err)
		}
		if item.TotalCost, err = parseWide(costText); err != nil {
			return ports.SalePage{}, fmt.Errorf("total_cost: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ports.SalePage{}, fmt.Errorf("read sales: %w", err)
	}
	return ports.SalePage{Items: items, Total: total}, nil
}
