package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/inventory"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/shared/decimal"
)

type InventoryRepository struct {
	pool *pgxpool.Pool
}

var _ ports.InventoryRepository = (*InventoryRepository)(nil)

func NewInventoryRepository(pool *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{pool: pool}
}

// conditions collects WHERE conditions with numbered placeholders.
type conditions struct {
	parts []string
	args  []any
}

func (c *conditions) add(format string, value any) {
	c.args = append(c.args, value)
	c.parts = append(c.parts, fmt.Sprintf(format, len(c.args)))
}

func (c *conditions) addRaw(condition string) { c.parts = append(c.parts, condition) }

func (c *conditions) where() string {
	if len(c.parts) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(c.parts, " AND ")
}

// The stock of every variant, PVC from its rolls and tablecloths from their stock row.
// Numbers marked "display" are rounded here for reading only.
const stockSource = `
SELECT * FROM (
  SELECT p.id, p.sku, p.name, p.product_type, p.is_active,
         COALESCE(SUM(r.available_length_m), 0) AS quantity,
         v.minimum_stock_m AS minimum,
         COUNT(r.id) FILTER (WHERE r.available_length_m > 0) AS roll_count,
         COALESCE(SUM(round((r.acquisition_cost + r.additional_cost) * r.available_length_m
                            / r.initial_usable_length_m, 2)), 0) AS stock_value,
         NULL::numeric AS average_cost
  FROM products p
  JOIN pvc_product_details v ON v.product_id = p.id
  LEFT JOIN pvc_rolls r ON r.product_id = p.id AND r.superseded_at IS NULL
  GROUP BY p.id, v.minimum_stock_m
  UNION ALL
  SELECT p.id, p.sku, p.name, p.product_type, p.is_active,
         COALESCE(s.quantity, 0)::numeric, t.minimum_units::numeric, 0::bigint,
         COALESCE(s.stock_value, 0),
         CASE WHEN COALESCE(s.quantity, 0) > 0 THEN round(s.stock_value / s.quantity, 4) END
  FROM products p
  JOIN tablecloth_product_details t ON t.product_id = p.id
  LEFT JOIN tablecloth_stock s ON s.product_id = p.id
) stock`

func (r *InventoryRepository) ListStock(ctx context.Context, filter ports.StockFilter) (ports.StockPage, error) {
	var where conditions
	// Inactive variants stay visible only while they still hold stock.
	where.addRaw("(is_active OR quantity > 0)")
	if filter.Type != "" {
		where.add("product_type = $%d", string(filter.Type))
	}
	if query := strings.TrimSpace(filter.Query); query != "" {
		where.add("(sku ILIKE $%[1]d OR name ILIKE $%[1]d)", "%"+escapeLike(query)+"%")
	}
	if filter.OnlyLow {
		where.addRaw("(minimum > 0 AND quantity <= minimum)")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM ("+stockSource+")"+" AS s"+where.where(), where.args...).Scan(&total); err != nil {
		return ports.StockPage{}, fmt.Errorf("count stock: %w", err)
	}

	args := append(append([]any{}, where.args...), filter.Limit, filter.Offset)
	rows, err := r.pool.Query(ctx,
		fmt.Sprintf(`SELECT id::text, sku, name, product_type, is_active, quantity::text, minimum::text,
		        roll_count, stock_value::text, COALESCE(average_cost::text, '')
		 FROM (%s) AS s%s
		 ORDER BY (minimum > 0 AND quantity <= minimum) DESC, sku LIMIT $%d OFFSET $%d`,
			stockSource, where.where(), len(where.args)+1, len(where.args)+2), args...)
	if err != nil {
		return ports.StockPage{}, fmt.Errorf("list stock: %w", err)
	}
	defer rows.Close()

	items := []ports.StockItem{}
	for rows.Next() {
		var (
			item                          ports.StockItem
			productType                   string
			quantity, minimum, stockValue string
		)
		if err := rows.Scan(&item.ProductID, &item.SKU, &item.Name, &productType, &item.IsActive,
			&quantity, &minimum, &item.RollCount, &stockValue, &item.AverageCost); err != nil {
			return ports.StockPage{}, fmt.Errorf("scan stock: %w", err)
		}
		item.Type = product.Type(productType)
		if item.Quantity, err = parseWide(quantity); err != nil {
			return ports.StockPage{}, fmt.Errorf("quantity: %w", err)
		}
		if item.Minimum, err = parseWide(minimum); err != nil {
			return ports.StockPage{}, fmt.Errorf("minimum: %w", err)
		}
		if item.StockValue, err = parseWide(stockValue); err != nil {
			return ports.StockPage{}, fmt.Errorf("stock_value: %w", err)
		}
		item.IsLow = item.Minimum.IsPositive() && item.Quantity.Hundredths() <= item.Minimum.Hundredths()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ports.StockPage{}, fmt.Errorf("read stock: %w", err)
	}
	return ports.StockPage{Items: items, Total: total}, nil
}

const selectRoll = `
SELECT r.id::text, r.roll_code, r.label_code, p.id::text, p.sku, p.name, r.purchase_id::text, r.received_at,
       r.initial_usable_length_m::text, r.available_length_m::text,
       r.acquisition_cost::text, r.additional_cost::text,
       (r.acquisition_cost + r.additional_cost)::text,
       round((r.acquisition_cost + r.additional_cost) / r.initial_usable_length_m, 4)::text,
       round((r.acquisition_cost + r.additional_cost) * r.available_length_m / r.initial_usable_length_m, 2)::text
FROM pvc_rolls r JOIN products p ON p.id = r.product_id`

func scanRoll(row pgx.Row) (ports.RollItem, error) {
	var (
		item                                                          ports.RollItem
		receivedAt                                                    time.Time
		initial, available, acquisition, additional, total, remaining string
	)
	if err := row.Scan(&item.ID, &item.Code, &item.LabelCode, &item.ProductID, &item.SKU, &item.Name, &item.PurchaseID, &receivedAt,
		&initial, &available, &acquisition, &additional, &total, &item.CostPerMeter, &remaining); err != nil {
		return ports.RollItem{}, err
	}
	item.ReceivedAt = receivedAt.UTC()
	var err error
	for _, field := range []struct {
		name   string
		text   string
		target *decimal.Decimal
	}{
		{"initial", initial, &item.InitialM}, {"available", available, &item.AvailableM},
		{"acquisition", acquisition, &item.Acquisition}, {"additional", additional, &item.Additional},
		{"attributable", total, &item.Attributable}, {"remaining", remaining, &item.RemainingValue},
	} {
		if *field.target, err = parseWide(field.text); err != nil {
			return ports.RollItem{}, fmt.Errorf("%s: %w", field.name, err)
		}
	}
	return item, nil
}

func (r *InventoryRepository) ListRolls(ctx context.Context, filter ports.RollFilter) (ports.RollPage, error) {
	var where conditions
	where.addRaw("r.superseded_at IS NULL") // rolls of a replaced purchase item are history
	if filter.ProductID != "" {
		if !uuidPattern.MatchString(filter.ProductID) {
			return ports.RollPage{Items: []ports.RollItem{}}, nil
		}
		where.add("r.product_id = $%d", filter.ProductID)
	}
	switch filter.Status {
	case ports.RollStatusDepleted:
		where.addRaw("r.available_length_m = 0")
	case ports.RollStatusAll:
	default:
		where.addRaw("r.available_length_m > 0")
	}
	if query := strings.TrimSpace(filter.Query); query != "" {
		where.add("(r.roll_code ILIKE $%[1]d OR p.sku ILIKE $%[1]d OR p.name ILIKE $%[1]d)", "%"+escapeLike(query)+"%")
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT count(*) FROM pvc_rolls r JOIN products p ON p.id = r.product_id"+where.where(), where.args...).Scan(&total); err != nil {
		return ports.RollPage{}, fmt.Errorf("count rolls: %w", err)
	}

	args := append(append([]any{}, where.args...), filter.Limit, filter.Offset)
	rows, err := r.pool.Query(ctx,
		fmt.Sprintf("%s%s ORDER BY r.received_at DESC, r.roll_code DESC LIMIT $%d OFFSET $%d",
			selectRoll, where.where(), len(where.args)+1, len(where.args)+2), args...)
	if err != nil {
		return ports.RollPage{}, fmt.Errorf("list rolls: %w", err)
	}
	defer rows.Close()

	items := []ports.RollItem{}
	for rows.Next() {
		item, err := scanRoll(rows)
		if err != nil {
			return ports.RollPage{}, fmt.Errorf("scan roll: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ports.RollPage{}, fmt.Errorf("read rolls: %w", err)
	}
	return ports.RollPage{Items: items, Total: total}, nil
}

func (r *InventoryRepository) GetRollByLabelCode(ctx context.Context, code string) (ports.RollItem, error) {
	item, err := scanRoll(r.pool.QueryRow(ctx, selectRoll+" WHERE r.label_code = $1 AND r.superseded_at IS NULL", code))
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.RollItem{}, inventory.ErrRollNotFound
	}
	if err != nil {
		return ports.RollItem{}, fmt.Errorf("get roll by label code: %w", err)
	}
	return item, nil
}

// ResetRollLabelCode generates a new unique label code for the roll. It changes no stock.
func (r *InventoryRepository) ResetRollLabelCode(ctx context.Context, id string) (ports.RollItem, error) {
	if !uuidPattern.MatchString(id) {
		return ports.RollItem{}, inventory.ErrRollNotFound
	}
	err := resetLabelCode(ctx, r.pool, "pvc_rolls", "R", id)
	if errors.Is(err, errNoRow) {
		return ports.RollItem{}, inventory.ErrRollNotFound
	}
	if err != nil {
		return ports.RollItem{}, err
	}
	return r.GetRoll(ctx, id)
}

func (r *InventoryRepository) GetRoll(ctx context.Context, id string) (ports.RollItem, error) {
	if !uuidPattern.MatchString(id) {
		return ports.RollItem{}, inventory.ErrRollNotFound
	}
	item, err := scanRoll(r.pool.QueryRow(ctx, selectRoll+" WHERE r.id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.RollItem{}, inventory.ErrRollNotFound
	}
	if err != nil {
		return ports.RollItem{}, fmt.Errorf("get roll: %w", err)
	}
	return item, nil
}

func (r *InventoryRepository) ListMovements(ctx context.Context, filter ports.MovementFilter) (ports.MovementPage, error) {
	var where conditions
	for _, id := range []struct{ value, column string }{{filter.ProductID, "m.product_id"}, {filter.RollID, "m.roll_id"}} {
		if id.value == "" {
			continue
		}
		if !uuidPattern.MatchString(id.value) {
			return ports.MovementPage{Items: []ports.MovementItem{}}, nil
		}
		where.add(id.column+" = $%d", id.value)
	}
	if filter.Type != "" {
		where.add("m.movement_type = $%d", string(filter.Type))
	}

	const from = ` FROM inventory_movements m
	  JOIN products p ON p.id = m.product_id
	  LEFT JOIN pvc_rolls r ON r.id = m.roll_id`
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*)"+from+where.where(), where.args...).Scan(&total); err != nil {
		return ports.MovementPage{}, fmt.Errorf("count movements: %w", err)
	}

	args := append(append([]any{}, where.args...), filter.Limit, filter.Offset)
	rows, err := r.pool.Query(ctx,
		fmt.Sprintf(`SELECT m.id::text, m.created_at, p.id::text, p.sku, p.name,
		        COALESCE(r.id::text, ''), COALESCE(r.roll_code, ''), m.movement_type,
		        m.quantity_delta::text, m.unit, m.reference_type, m.reference_id::text, m.reason%s%s
		 ORDER BY m.created_at DESC, m.id LIMIT $%d OFFSET $%d`,
			from, where.where(), len(where.args)+1, len(where.args)+2), args...)
	if err != nil {
		return ports.MovementPage{}, fmt.Errorf("list movements: %w", err)
	}
	defer rows.Close()

	items := []ports.MovementItem{}
	for rows.Next() {
		var (
			item               ports.MovementItem
			createdAt          time.Time
			movementType, unit string
			delta              string
		)
		if err := rows.Scan(&item.ID, &createdAt, &item.ProductID, &item.SKU, &item.Name, &item.RollID, &item.RollCode,
			&movementType, &delta, &unit, &item.ReferenceType, &item.ReferenceID, &item.Reason); err != nil {
			return ports.MovementPage{}, fmt.Errorf("scan movement: %w", err)
		}
		item.CreatedAt = createdAt.UTC()
		item.Type = inventory.MovementType(movementType)
		item.Unit = inventory.Unit(unit)
		if item.QuantityDelta, err = parseWide(delta); err != nil {
			return ports.MovementPage{}, fmt.Errorf("quantity_delta: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ports.MovementPage{}, fmt.Errorf("read movements: %w", err)
	}
	return ports.MovementPage{Items: items, Total: total}, nil
}
