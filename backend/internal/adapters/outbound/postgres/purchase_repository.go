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
	"gumunova/backend/internal/domain/purchase"
	"gumunova/backend/internal/domain/shared/decimal"
)

type PurchaseRepository struct {
	pool *pgxpool.Pool
}

var _ ports.PurchaseRepository = (*PurchaseRepository)(nil)

func NewPurchaseRepository(pool *pgxpool.Pool) *PurchaseRepository {
	return &PurchaseRepository{pool: pool}
}

// reader is satisfied by both the pool and a transaction.
type reader interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Register saves the whole purchase in one transaction: the purchase, its lines, one physical
// roll per PVC roll (with its generated code), the tablecloth stock increase and every
// inventory movement. If anything fails, nothing is saved.
func (r *PurchaseRepository) Register(ctx context.Context, p purchase.Purchase, key ports.IdempotencyKey) (ports.RegisterResult, error) {
	transaction, err := r.pool.Begin(ctx)
	if err != nil {
		return ports.RegisterResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer transaction.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	// A unique key makes this insert the gate: concurrent requests with the same key queue
	// here, and only one of them inserts.
	var purchaseID string
	err = transaction.QueryRow(ctx,
		`INSERT INTO purchases
		   (supplier_id, purchase_date, document_reference, notes, additional_costs, total_amount,
		    status, idempotency_key, request_fingerprint)
		 VALUES ($1, $2, $3, $4, $5::text::numeric, $6::text::numeric, $7, $8, $9)
		 ON CONFLICT (idempotency_key) DO NOTHING
		 RETURNING id::text`,
		nullableUUID(p.SupplierID), p.PurchaseDate, p.DocumentReference, p.Notes, p.AdditionalCosts.String(),
		p.Total.String(), string(p.Status), key.Key, key.Fingerprint,
	).Scan(&purchaseID)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = transaction.Rollback(ctx)
		return r.replay(ctx, key)
	}
	if err != nil {
		return ports.RegisterResult{}, fmt.Errorf("insert purchase: %w", err)
	}

	if err := insertItems(ctx, transaction, purchaseID, p); err != nil {
		return ports.RegisterResult{}, err
	}

	stored, err := getPurchase(ctx, transaction, purchaseID)
	if err != nil {
		return ports.RegisterResult{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return ports.RegisterResult{}, fmt.Errorf("commit transaction: %w", err)
	}
	return ports.RegisterResult{Purchase: stored}, nil
}

// insertItems writes the lines, rolls, stock increases and movements of a purchase. Products
// created by the purchase come first, in the same transaction: if it fails, no product is left behind.
func insertItems(ctx context.Context, transaction pgx.Tx, purchaseID string, p purchase.Purchase) error {
	lines := make([]purchase.Line, len(p.Lines))
	copy(lines, p.Lines)
	for index := range lines {
		if lines[index].NewProduct == nil {
			continue
		}
		id, err := insertProduct(ctx, transaction, *lines[index].NewProduct)
		if errors.Is(err, product.ErrSKUAlreadyExists) {
			return &purchase.SKUConflictError{Line: index}
		}
		if err != nil {
			return err
		}
		lines[index].ProductID = id
	}
	for _, line := range lines {
		if err := insertLine(ctx, transaction, purchaseID, p.PurchaseDate, line); err != nil {
			return err
		}
	}
	return nil
}

// replay answers a retry: the same key and request returns what was already registered.
func (r *PurchaseRepository) replay(ctx context.Context, key ports.IdempotencyKey) (ports.RegisterResult, error) {
	var existingID, existingFingerprint string
	err := r.pool.QueryRow(ctx,
		`SELECT id::text, request_fingerprint FROM purchases WHERE idempotency_key = $1`, key.Key,
	).Scan(&existingID, &existingFingerprint)
	if err != nil {
		return ports.RegisterResult{}, fmt.Errorf("look up idempotency key: %w", err)
	}
	if existingFingerprint != key.Fingerprint {
		return ports.RegisterResult{}, purchase.ErrIdempotencyKeyReused
	}
	stored, err := getPurchase(ctx, r.pool, existingID)
	if err != nil {
		return ports.RegisterResult{}, err
	}
	return ports.RegisterResult{Purchase: stored, Replayed: true}, nil
}

func insertLine(ctx context.Context, transaction pgx.Tx, purchaseID string, receivedAt time.Time, line purchase.Line) error {
	var unitCost *string
	if line.ProductType == product.TypeTablecloth {
		text := line.UnitCost.String()
		unitCost = &text
	}

	var lineID string
	err := transaction.QueryRow(ctx,
		`INSERT INTO purchase_lines
		   (purchase_id, line_number, product_id, quantity, unit_cost, additional_cost, line_total)
		 VALUES ($1, $2, $3, $4, $5::text::numeric, $6::text::numeric, $7::text::numeric)
		 RETURNING id::text`,
		purchaseID, line.Number, line.ProductID, line.Quantity, unitCost,
		line.AdditionalCost.String(), line.Total.String(),
	).Scan(&lineID)
	if err != nil {
		return fmt.Errorf("insert purchase line %d: %w", line.Number, err)
	}

	if line.ProductType == product.TypePVC {
		for position, roll := range line.Rolls {
			var rollID string
			err := transaction.QueryRow(ctx,
				`INSERT INTO pvc_rolls
				   (roll_code, product_id, purchase_id, purchase_line_id, line_position,
				    initial_usable_length_m, available_length_m, acquisition_cost, additional_cost, received_at)
				 VALUES (next_roll_code(), $1, $2, $3, $4,
				         $5::text::numeric, $5::text::numeric, $6::text::numeric, $7::text::numeric, $8)
				 RETURNING id::text`,
				line.ProductID, purchaseID, lineID, position+1,
				roll.LengthM.String(), roll.AcquisitionCost.String(), roll.AdditionalCost.String(), receivedAt,
			).Scan(&rollID)
			if err != nil {
				return fmt.Errorf("insert roll %d of line %d: %w", position+1, line.Number, err)
			}
			if err := insertMovement(ctx, transaction, line.ProductID, &rollID, roll.LengthM, inventory.UnitMeter, purchaseID); err != nil {
				return err
			}
		}
		return nil
	}

	// Tablecloths: stock and its value go up atomically; the weighted average follows from them.
	_, err = transaction.Exec(ctx,
		`INSERT INTO tablecloth_stock (product_id, quantity, stock_value)
		 VALUES ($1, $2, $3::text::numeric)
		 ON CONFLICT (product_id) DO UPDATE SET
		   quantity    = tablecloth_stock.quantity + EXCLUDED.quantity,
		   stock_value = tablecloth_stock.stock_value + EXCLUDED.stock_value,
		   updated_at  = now()`,
		line.ProductID, line.Quantity, line.Total.String())
	if err != nil {
		return fmt.Errorf("update stock of line %d: %w", line.Number, err)
	}
	quantity := decimal.FromHundredths(int64(line.Quantity) * 100)
	return insertMovement(ctx, transaction, line.ProductID, nil, quantity, inventory.UnitPiece, purchaseID)
}

func insertMovement(ctx context.Context, transaction pgx.Tx, productID string, rollID *string, quantity decimal.Decimal, unit inventory.Unit, purchaseID string) error {
	return insertTypedMovement(ctx, transaction, inventory.MovementPurchaseIn, productID, rollID, quantity, unit, purchaseID)
}

func insertTypedMovement(ctx context.Context, transaction pgx.Tx, movement inventory.MovementType, productID string, rollID *string, quantity decimal.Decimal, unit inventory.Unit, purchaseID string) error {
	_, err := transaction.Exec(ctx,
		`INSERT INTO inventory_movements
		   (product_id, roll_id, movement_type, quantity_delta, unit, reference_type, reference_id)
		 VALUES ($1, $2, $3, $4::text::numeric, $5, $6, $7)`,
		productID, rollID, string(movement), quantity.String(), string(unit),
		inventory.ReferencePurchase, purchaseID)
	if err != nil {
		return fmt.Errorf("insert inventory movement: %w", err)
	}
	return nil
}

func (r *PurchaseRepository) GetByID(ctx context.Context, id string) (purchase.Purchase, error) {
	if !uuidPattern.MatchString(id) {
		return purchase.Purchase{}, purchase.ErrNotFound
	}
	return getPurchase(ctx, r.pool, id)
}

func getPurchase(ctx context.Context, q reader, id string) (purchase.Purchase, error) {
	var (
		p                         purchase.Purchase
		status, additional, total string
		purchaseDate, createdAt   time.Time
	)
	err := q.QueryRow(ctx,
		`SELECT id::text, COALESCE(supplier_id::text, ''), purchase_date, document_reference, notes,
		        additional_costs::text, total_amount::text, status, created_at, revision
		 FROM purchases WHERE id = $1`, id,
	).Scan(&p.ID, &p.SupplierID, &purchaseDate, &p.DocumentReference, &p.Notes, &additional, &total, &status, &createdAt, &p.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return purchase.Purchase{}, purchase.ErrNotFound
	}
	if err != nil {
		return purchase.Purchase{}, fmt.Errorf("get purchase: %w", err)
	}
	p.PurchaseDate = purchaseDate.UTC()
	p.CreatedAt = createdAt.UTC()
	p.Status = purchase.Status(status)
	if p.AdditionalCosts, err = decimal.Parse(additional); err != nil {
		return purchase.Purchase{}, fmt.Errorf("additional_costs: %w", err)
	}
	if p.Total, err = parseWide(total); err != nil {
		return purchase.Purchase{}, fmt.Errorf("total_amount: %w", err)
	}

	if p.Lines, err = getLines(ctx, q, id); err != nil {
		return purchase.Purchase{}, err
	}
	return p, nil
}

func getLines(ctx context.Context, q reader, purchaseID string) ([]purchase.Line, error) {
	rows, err := q.Query(ctx,
		`SELECT l.id::text, l.line_number, l.product_id::text, p.product_type, l.quantity,
		        l.unit_cost::text, l.additional_cost::text, l.line_total::text
		 FROM purchase_lines l JOIN products p ON p.id = l.product_id
		 WHERE l.purchase_id = $1 AND l.superseded_at IS NULL ORDER BY l.line_number`, purchaseID)
	if err != nil {
		return nil, fmt.Errorf("get purchase lines: %w", err)
	}
	defer rows.Close()

	var lines []purchase.Line
	for rows.Next() {
		var (
			line              purchase.Line
			productType       string
			unitCost          *string
			additional, total string
		)
		if err := rows.Scan(&line.ID, &line.Number, &line.ProductID, &productType, &line.Quantity, &unitCost, &additional, &total); err != nil {
			return nil, fmt.Errorf("scan purchase line: %w", err)
		}
		line.ProductType = product.Type(productType)
		if unitCost != nil {
			if line.UnitCost, err = decimal.Parse(*unitCost); err != nil {
				return nil, fmt.Errorf("unit_cost: %w", err)
			}
		}
		if line.AdditionalCost, err = parseWide(additional); err != nil {
			return nil, fmt.Errorf("additional_cost: %w", err)
		}
		if line.Total, err = parseWide(total); err != nil {
			return nil, fmt.Errorf("line_total: %w", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read purchase lines: %w", err)
	}

	rolls, err := getRolls(ctx, q, purchaseID)
	if err != nil {
		return nil, err
	}
	for index := range lines {
		lines[index].Rolls = rolls[lines[index].ID]
	}
	return lines, nil
}

// getRolls returns the rolls of a purchase grouped by purchase line.
func getRolls(ctx context.Context, q reader, purchaseID string) (map[string][]purchase.Roll, error) {
	rows, err := q.Query(ctx,
		`SELECT id::text, roll_code, purchase_line_id::text, product_id::text,
		        initial_usable_length_m::text, acquisition_cost::text, additional_cost::text
		 FROM pvc_rolls WHERE purchase_id = $1 AND superseded_at IS NULL ORDER BY purchase_line_id, line_position`, purchaseID)
	if err != nil {
		return nil, fmt.Errorf("get rolls: %w", err)
	}
	defer rows.Close()

	byLine := make(map[string][]purchase.Roll)
	for rows.Next() {
		var (
			roll                            purchase.Roll
			lineID                          string
			length, acquisition, additional string
		)
		if err := rows.Scan(&roll.ID, &roll.Code, &lineID, &roll.ProductID, &length, &acquisition, &additional); err != nil {
			return nil, fmt.Errorf("scan roll: %w", err)
		}
		if roll.LengthM, err = decimal.Parse(length); err != nil {
			return nil, fmt.Errorf("initial_usable_length_m: %w", err)
		}
		if roll.AcquisitionCost, err = decimal.Parse(acquisition); err != nil {
			return nil, fmt.Errorf("acquisition_cost: %w", err)
		}
		if roll.AdditionalCost, err = decimal.Parse(additional); err != nil {
			return nil, fmt.Errorf("additional_cost: %w", err)
		}
		byLine[lineID] = append(byLine[lineID], roll)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read rolls: %w", err)
	}
	return byLine, nil
}

func (r *PurchaseRepository) List(ctx context.Context, filter ports.PurchaseFilter) (ports.PurchasePage, error) {
	var conditions []string
	var args []any
	addCondition := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}

	if filter.SupplierID != "" {
		if !uuidPattern.MatchString(filter.SupplierID) {
			return ports.PurchasePage{Items: []ports.PurchaseSummary{}}, nil // no supplier has that id
		}
		addCondition("pu.supplier_id = $%d", filter.SupplierID)
	}
	if filter.From != nil {
		addCondition("pu.purchase_date >= $%d", *filter.From)
	}
	if filter.To != nil {
		addCondition("pu.purchase_date <= $%d", *filter.To)
	}
	if query := strings.TrimSpace(filter.Query); query != "" {
		addCondition("(pu.document_reference ILIKE $%[1]d OR s.name ILIKE $%[1]d)", "%"+escapeLike(query)+"%")
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	from := " FROM purchases pu LEFT JOIN suppliers s ON s.id = pu.supplier_id"

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*)"+from+where, args...).Scan(&total); err != nil {
		return ports.PurchasePage{}, fmt.Errorf("count purchases: %w", err)
	}

	pageArgs := append(append([]any{}, args...), filter.Limit, filter.Offset)
	listSQL := fmt.Sprintf(
		`SELECT pu.id::text, COALESCE(pu.supplier_id::text, ''), COALESCE(s.name, ''), pu.purchase_date, pu.document_reference,
		        pu.total_amount::text,
		        (SELECT count(*) FROM purchase_lines l WHERE l.purchase_id = pu.id AND l.superseded_at IS NULL), pu.created_at%s%s
		 ORDER BY pu.purchase_date DESC, pu.created_at DESC, pu.id LIMIT $%d OFFSET $%d`,
		from, where, len(args)+1, len(args)+2)
	rows, err := r.pool.Query(ctx, listSQL, pageArgs...)
	if err != nil {
		return ports.PurchasePage{}, fmt.Errorf("list purchases: %w", err)
	}
	defer rows.Close()

	items := []ports.PurchaseSummary{}
	for rows.Next() {
		var (
			item                    ports.PurchaseSummary
			purchaseDate, createdAt time.Time
			totalText               string
		)
		if err := rows.Scan(&item.ID, &item.SupplierID, &item.SupplierName, &purchaseDate, &item.DocumentReference,
			&totalText, &item.LineCount, &createdAt); err != nil {
			return ports.PurchasePage{}, fmt.Errorf("scan purchase: %w", err)
		}
		item.PurchaseDate = purchaseDate.UTC()
		item.CreatedAt = createdAt.UTC()
		if item.Total, err = parseWide(totalText); err != nil {
			return ports.PurchasePage{}, fmt.Errorf("total_amount: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ports.PurchasePage{}, fmt.Errorf("read purchases: %w", err)
	}
	return ports.PurchasePage{Items: items, Total: total}, nil
}

// parseWide reads a NUMERIC(18,2) value. Totals and sums can exceed the range accepted for a
// single price, so the digits are read directly instead of through decimal.Parse.
func parseWide(text string) (decimal.Decimal, error) {
	whole, fraction, hasFraction := strings.Cut(text, ".")
	if !hasFraction {
		fraction = "00"
	}
	if len(fraction) != 2 || whole == "" {
		return decimal.Decimal{}, fmt.Errorf("unexpected numeric value %q", text)
	}
	var hundredths int64
	negative := strings.HasPrefix(whole, "-")
	for _, digit := range strings.TrimPrefix(whole, "-") + fraction {
		if digit < '0' || digit > '9' {
			return decimal.Decimal{}, fmt.Errorf("unexpected numeric value %q", text)
		}
		next := hundredths*10 + int64(digit-'0')
		if next < hundredths {
			return decimal.Decimal{}, fmt.Errorf("numeric value %q is out of range", text)
		}
		hundredths = next
	}
	if negative {
		hundredths = -hundredths
	}
	return decimal.FromHundredths(hundredths), nil
}

// nullableUUID stores a purchase without a supplier as NULL instead of an invalid empty uuid.
func nullableUUID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

// Update saves the corrected purchase. Header data is always updated; the items are replaced
// only when asked, and only while nothing they brought in has been sold.
func (r *PurchaseRepository) Update(ctx context.Context, id string, p purchase.Purchase, replaceItems bool) (purchase.Purchase, error) {
	if !uuidPattern.MatchString(id) {
		return purchase.Purchase{}, purchase.ErrNotFound
	}
	transaction, err := r.pool.Begin(ctx)
	if err != nil {
		return purchase.Purchase{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer transaction.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	// The purchase row is the lock for editing it: two edits of the same purchase queue here.
	var locked string
	err = transaction.QueryRow(ctx, `SELECT id::text FROM purchases WHERE id = $1 FOR UPDATE`, id).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return purchase.Purchase{}, purchase.ErrNotFound
	}
	if err != nil {
		return purchase.Purchase{}, fmt.Errorf("lock purchase: %w", err)
	}

	if replaceItems {
		if err := lockPurchaseStock(ctx, transaction, id); err != nil {
			return purchase.Purchase{}, err
		}
		editable, err := itemsEditable(ctx, transaction, id)
		if err != nil {
			return purchase.Purchase{}, err
		}
		if !editable {
			return purchase.Purchase{}, purchase.ErrItemsLocked
		}
		if err := supersedeItems(ctx, transaction, id); err != nil {
			return purchase.Purchase{}, err
		}
	}

	_, err = transaction.Exec(ctx,
		`UPDATE purchases SET supplier_id = $2, purchase_date = $3, document_reference = $4, notes = $5,
		        additional_costs = $6::text::numeric, total_amount = $7::text::numeric,
		        revision = revision + $8, updated_at = now()
		 WHERE id = $1`,
		id, nullableUUID(p.SupplierID), p.PurchaseDate, p.DocumentReference, p.Notes,
		p.AdditionalCosts.String(), p.Total.String(), boolToInt(replaceItems))
	if err != nil {
		return purchase.Purchase{}, fmt.Errorf("update purchase: %w", err)
	}

	if replaceItems {
		if err := insertItems(ctx, transaction, id, p); err != nil {
			return purchase.Purchase{}, err
		}
	} else if _, err := transaction.Exec(ctx,
		`UPDATE pvc_rolls SET received_at = $2 WHERE purchase_id = $1 AND superseded_at IS NULL`, id, p.PurchaseDate); err != nil {
		return purchase.Purchase{}, fmt.Errorf("update roll received date: %w", err)
	}

	stored, err := getPurchase(ctx, transaction, id)
	if err != nil {
		return purchase.Purchase{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return purchase.Purchase{}, fmt.Errorf("commit transaction: %w", err)
	}
	return stored, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (r *PurchaseRepository) ItemsEditable(ctx context.Context, id string) (bool, error) {
	if !uuidPattern.MatchString(id) {
		return false, purchase.ErrNotFound
	}
	return itemsEditable(ctx, r.pool, id)
}

// lockPurchaseStock locks the rolls and tablecloth stock rows of the purchase, in the same
// order sales lock them, so the check that follows cannot race with a sale.
func lockPurchaseStock(ctx context.Context, transaction pgx.Tx, purchaseID string) error {
	if _, err := transaction.Exec(ctx,
		`SELECT 1 FROM pvc_rolls WHERE purchase_id = $1 AND superseded_at IS NULL ORDER BY id FOR UPDATE`, purchaseID); err != nil {
		return fmt.Errorf("lock rolls of purchase: %w", err)
	}
	if _, err := transaction.Exec(ctx,
		`SELECT 1 FROM tablecloth_stock
		 WHERE product_id IN (SELECT product_id FROM purchase_lines
		                      WHERE purchase_id = $1 AND superseded_at IS NULL AND unit_cost IS NOT NULL)
		 ORDER BY product_id FOR UPDATE`, purchaseID); err != nil {
		return fmt.Errorf("lock tablecloth stock of purchase: %w", err)
	}
	return nil
}

// itemsEditable is true while nothing the purchase brought in has been sold: every active roll
// is whole and was never sold, and no tablecloth of the purchase's products has been sold since
// the purchase was registered (tablecloth units are interchangeable, so any later sale of the
// product could have used them).
func itemsEditable(ctx context.Context, q reader, purchaseID string) (bool, error) {
	var editable bool
	err := q.QueryRow(ctx,
		`SELECT NOT EXISTS (
		     SELECT 1 FROM pvc_rolls r
		     WHERE r.purchase_id = $1 AND r.superseded_at IS NULL
		       AND (r.available_length_m <> r.initial_usable_length_m
		            OR EXISTS (SELECT 1 FROM sale_lines sl WHERE sl.roll_id = r.id)))
		   AND NOT EXISTS (
		     SELECT 1 FROM purchase_lines l
		     JOIN purchases pu ON pu.id = l.purchase_id
		     WHERE l.purchase_id = $1 AND l.superseded_at IS NULL AND l.unit_cost IS NOT NULL
		       AND EXISTS (SELECT 1 FROM inventory_movements m
		                   WHERE m.product_id = l.product_id AND m.movement_type = 'sale_out'
		                     AND m.created_at >= pu.created_at))`, purchaseID).Scan(&editable)
	if err != nil {
		return false, fmt.Errorf("check purchase items: %w", err)
	}
	return editable, nil
}

// supersedeItems cancels the stock of the current items with compensating movements and marks
// them superseded. Nothing is deleted: the original lines, rolls and movements stay on record.
func supersedeItems(ctx context.Context, transaction pgx.Tx, purchaseID string) error {
	rows, err := transaction.Query(ctx,
		`SELECT id::text, product_id::text, initial_usable_length_m::text
		 FROM pvc_rolls WHERE purchase_id = $1 AND superseded_at IS NULL ORDER BY id`, purchaseID)
	if err != nil {
		return fmt.Errorf("read rolls to supersede: %w", err)
	}
	type rollRow struct{ id, productID, length string }
	var rolls []rollRow
	for rows.Next() {
		var row rollRow
		if err := rows.Scan(&row.id, &row.productID, &row.length); err != nil {
			rows.Close()
			return fmt.Errorf("scan roll to supersede: %w", err)
		}
		rolls = append(rolls, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read rolls to supersede: %w", err)
	}
	for _, roll := range rolls {
		length, err := decimal.Parse(roll.length)
		if err != nil {
			return fmt.Errorf("roll length: %w", err)
		}
		rollID := roll.id
		if err := insertTypedMovement(ctx, transaction, inventory.MovementPurchaseReversal, roll.productID, &rollID, decimal.FromHundredths(-length.Hundredths()), inventory.UnitMeter, purchaseID); err != nil {
			return err
		}
		if _, err := transaction.Exec(ctx,
			`UPDATE pvc_rolls SET available_length_m = 0, superseded_at = now() WHERE id = $1`, roll.id); err != nil {
			return fmt.Errorf("supersede roll: %w", err)
		}
	}

	lineRows, err := transaction.Query(ctx,
		`SELECT product_id::text, quantity, line_total::text FROM purchase_lines
		 WHERE purchase_id = $1 AND superseded_at IS NULL AND unit_cost IS NOT NULL ORDER BY product_id, line_number`, purchaseID)
	if err != nil {
		return fmt.Errorf("read tablecloth lines to supersede: %w", err)
	}
	type lineRow struct {
		productID string
		quantity  int
		total     string
	}
	var tablecloths []lineRow
	for lineRows.Next() {
		var row lineRow
		if err := lineRows.Scan(&row.productID, &row.quantity, &row.total); err != nil {
			lineRows.Close()
			return fmt.Errorf("scan tablecloth line: %w", err)
		}
		tablecloths = append(tablecloths, row)
	}
	lineRows.Close()
	if err := lineRows.Err(); err != nil {
		return fmt.Errorf("read tablecloth lines to supersede: %w", err)
	}
	for _, line := range tablecloths {
		// The stock check constraints refuse to go below zero, so a miscount fails loudly.
		if _, err := transaction.Exec(ctx,
			`UPDATE tablecloth_stock SET quantity = quantity - $2, stock_value = stock_value - $3::text::numeric, updated_at = now()
			 WHERE product_id = $1`, line.productID, line.quantity, line.total); err != nil {
			return fmt.Errorf("reverse tablecloth stock: %w", err)
		}
		quantity := decimal.FromHundredths(int64(line.quantity) * 100)
		if err := insertTypedMovement(ctx, transaction, inventory.MovementPurchaseReversal, line.productID, nil, decimal.FromHundredths(-quantity.Hundredths()), inventory.UnitPiece, purchaseID); err != nil {
			return err
		}
	}

	if _, err := transaction.Exec(ctx,
		`UPDATE purchase_lines SET superseded_at = now() WHERE purchase_id = $1 AND superseded_at IS NULL`, purchaseID); err != nil {
		return fmt.Errorf("supersede lines: %w", err)
	}
	return nil
}
