// Package postgres implements the application ports on top of PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/product"
	"gumunova/backend/internal/domain/shared/decimal"
)

const (
	pgUniqueViolation = "23505"
	skuUniqueIndex    = "products_sku_key"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Numeric values travel as text (`$n::text::numeric` and `col::text`) so they never
// pass through a floating point type.
const selectProduct = `
SELECT p.id::text, p.sku, p.name, p.product_type, p.is_active, COALESCE(p.image_key, ''), p.label_code,
       p.created_at, p.updated_at,
       v.material, v.design, v.color, v.texture,
       v.width_m::text, v.nominal_length_m::text, v.thickness_mm::text,
       v.reference_price_per_meter::text, v.reference_price_per_roll::text,
       t.design, t.color, t.dimensions, t.reference_cost::text, t.unit_price::text,
       v.minimum_stock_m::text, t.minimum_units
FROM products p
LEFT JOIN pvc_product_details v ON v.product_id = p.id
LEFT JOIN tablecloth_product_details t ON t.product_id = p.id`

type ProductRepository struct {
	pool *pgxpool.Pool
}

var _ ports.ProductRepository = (*ProductRepository)(nil)

func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{pool: pool}
}

type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *ProductRepository) Create(ctx context.Context, p product.Product) (product.Product, error) {
	transaction, err := r.pool.Begin(ctx)
	if err != nil {
		return product.Product{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer transaction.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	id, err := insertProduct(ctx, transaction, p)
	if err != nil {
		return product.Product{}, err
	}
	created, err := getByID(ctx, transaction, id)
	if err != nil {
		return product.Product{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return product.Product{}, fmt.Errorf("commit transaction: %w", err)
	}
	return created, nil
}

// insertProduct writes a product and its details inside the caller's transaction. It returns
// product.ErrSKUAlreadyExists when the SKU is taken.
func insertProduct(ctx context.Context, transaction pgx.Tx, p product.Product) (string, error) {
	var id string
	err := transaction.QueryRow(ctx,
		`INSERT INTO products (sku, name, product_type, is_active)
		 VALUES ($1, $2, $3, $4) RETURNING id::text`,
		p.SKU, p.Name, string(p.Type), p.IsActive,
	).Scan(&id)
	if err != nil {
		if isSKUConflict(err) {
			return "", product.ErrSKUAlreadyExists
		}
		return "", fmt.Errorf("insert product: %w", err)
	}
	if err := insertDetails(ctx, transaction, id, p); err != nil {
		return "", err
	}
	return id, nil
}

func (r *ProductRepository) Update(ctx context.Context, p product.Product) (product.Product, error) {
	if !uuidPattern.MatchString(p.ID) {
		return product.Product{}, product.ErrNotFound
	}
	transaction, err := r.pool.Begin(ctx)
	if err != nil {
		return product.Product{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer transaction.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	tag, err := transaction.Exec(ctx,
		`UPDATE products SET name = $2, is_active = $3, updated_at = now() WHERE id = $1`,
		p.ID, p.Name, p.IsActive)
	if err != nil {
		return product.Product{}, fmt.Errorf("update product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return product.Product{}, product.ErrNotFound
	}
	if err := updateDetails(ctx, transaction, p); err != nil {
		return product.Product{}, err
	}
	updated, err := getByID(ctx, transaction, p.ID)
	if err != nil {
		return product.Product{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return product.Product{}, fmt.Errorf("commit transaction: %w", err)
	}
	return updated, nil
}

func (r *ProductRepository) SetImageKey(ctx context.Context, id, imageKey string) error {
	if !uuidPattern.MatchString(id) {
		return product.ErrNotFound
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE products SET image_key = NULLIF($2, ''), updated_at = now() WHERE id = $1`,
		id, imageKey)
	if err != nil {
		return fmt.Errorf("set image key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return product.ErrNotFound
	}
	return nil
}

// ResetLabelCode generates a new unique label code for the product. A clash with another code
// is practically impossible (48 random bits) but the unique constraint is the authority, so a
// clash just draws a new code.
func (r *ProductRepository) ResetLabelCode(ctx context.Context, id string) (product.Product, error) {
	if !uuidPattern.MatchString(id) {
		return product.Product{}, product.ErrNotFound
	}
	err := resetLabelCode(ctx, r.pool, "products", "P", id)
	if errors.Is(err, errNoRow) {
		return product.Product{}, product.ErrNotFound
	}
	if err != nil {
		return product.Product{}, err
	}
	return getByID(ctx, r.pool, id)
}

func (r *ProductRepository) GetByLabelCode(ctx context.Context, code string) (product.Product, error) {
	found, err := scanProduct(r.pool.QueryRow(ctx, selectProduct+" WHERE p.label_code = $1", code))
	if errors.Is(err, pgx.ErrNoRows) {
		return product.Product{}, product.ErrNotFound
	}
	return found, err
}

func (r *ProductRepository) GetByID(ctx context.Context, id string) (product.Product, error) {
	if !uuidPattern.MatchString(id) {
		return product.Product{}, product.ErrNotFound
	}
	return getByID(ctx, r.pool, id)
}

func (r *ProductRepository) List(ctx context.Context, filter ports.ProductFilter) (ports.ProductPage, error) {
	var conditions []string
	var args []any
	addCondition := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}

	if filter.Type != "" {
		addCondition("p.product_type = $%d", string(filter.Type))
	}
	if filter.IsActive != nil {
		addCondition("p.is_active = $%d", *filter.IsActive)
	}
	if query := strings.TrimSpace(filter.Query); query != "" {
		addCondition("(p.sku ILIKE $%[1]d OR p.name ILIKE $%[1]d)", "%"+escapeLike(query)+"%")
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM products p"+where, args...).Scan(&total); err != nil {
		return ports.ProductPage{}, fmt.Errorf("count products: %w", err)
	}

	pageArgs := append(append([]any{}, args...), filter.Limit, filter.Offset)
	listSQL := fmt.Sprintf("%s%s ORDER BY p.name, p.sku LIMIT $%d OFFSET $%d",
		selectProduct, where, len(args)+1, len(args)+2)
	rows, err := r.pool.Query(ctx, listSQL, pageArgs...)
	if err != nil {
		return ports.ProductPage{}, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	items := []product.Product{}
	for rows.Next() {
		item, err := scanProduct(rows)
		if err != nil {
			return ports.ProductPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ports.ProductPage{}, fmt.Errorf("read products: %w", err)
	}
	return ports.ProductPage{Items: items, Total: total}, nil
}

func getByID(ctx context.Context, q queryer, id string) (product.Product, error) {
	found, err := scanProduct(q.QueryRow(ctx, selectProduct+" WHERE p.id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return product.Product{}, product.ErrNotFound
	}
	return found, err
}

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func insertDetails(ctx context.Context, e execer, id string, p product.Product) error {
	switch p.Type {
	case product.TypePVC:
		d := p.PVC
		_, err := e.Exec(ctx,
			`INSERT INTO pvc_product_details
			   (product_id, material, design, color, texture, width_m, nominal_length_m,
			    thickness_mm, reference_price_per_meter, reference_price_per_roll, minimum_stock_m)
			 VALUES ($1, $2, $3, $4, $5, $6::text::numeric, $7::text::numeric,
			         $8::text::numeric, $9::text::numeric, $10::text::numeric, $11::text::numeric)`,
			id, d.Material, d.Design, d.Color, d.Texture, d.WidthM.String(), d.NominalLengthM.String(),
			optionalDecimal(d.ThicknessMM), d.ReferencePricePerMeter.String(), d.ReferencePricePerRoll.String(),
			d.MinimumStockM.String())
		if err != nil {
			return fmt.Errorf("insert pvc details: %w", err)
		}
	case product.TypeTablecloth:
		d := p.Tablecloth
		_, err := e.Exec(ctx,
			`INSERT INTO tablecloth_product_details
			   (product_id, design, color, dimensions, reference_cost, unit_price, minimum_units)
			 VALUES ($1, $2, $3, $4, $5::text::numeric, $6::text::numeric, $7)`,
			id, d.Design, d.Color, d.Dimensions, optionalDecimal(d.ReferenceCost), d.UnitPrice.String(), d.MinimumUnits)
		if err != nil {
			return fmt.Errorf("insert tablecloth details: %w", err)
		}
	default:
		return fmt.Errorf("unsupported product type %q", p.Type)
	}
	return nil
}

func updateDetails(ctx context.Context, e execer, p product.Product) error {
	switch p.Type {
	case product.TypePVC:
		d := p.PVC
		_, err := e.Exec(ctx,
			`UPDATE pvc_product_details SET
			   material = $2, design = $3, color = $4, texture = $5,
			   width_m = $6::text::numeric, nominal_length_m = $7::text::numeric,
			   thickness_mm = $8::text::numeric,
			   reference_price_per_meter = $9::text::numeric,
			   reference_price_per_roll = $10::text::numeric,
			   minimum_stock_m = $11::text::numeric
			 WHERE product_id = $1`,
			p.ID, d.Material, d.Design, d.Color, d.Texture, d.WidthM.String(), d.NominalLengthM.String(),
			optionalDecimal(d.ThicknessMM), d.ReferencePricePerMeter.String(), d.ReferencePricePerRoll.String(),
			d.MinimumStockM.String())
		if err != nil {
			return fmt.Errorf("update pvc details: %w", err)
		}
	case product.TypeTablecloth:
		d := p.Tablecloth
		_, err := e.Exec(ctx,
			`UPDATE tablecloth_product_details SET
			   design = $2, color = $3, dimensions = $4,
			   reference_cost = $5::text::numeric, unit_price = $6::text::numeric,
			   minimum_units = $7
			 WHERE product_id = $1`,
			p.ID, d.Design, d.Color, d.Dimensions, optionalDecimal(d.ReferenceCost), d.UnitPrice.String(), d.MinimumUnits)
		if err != nil {
			return fmt.Errorf("update tablecloth details: %w", err)
		}
	default:
		return fmt.Errorf("unsupported product type %q", p.Type)
	}
	return nil
}

// optionalDecimal returns the text form of a decimal or nil (SQL NULL) when absent.
func optionalDecimal(value *decimal.Decimal) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProduct(row rowScanner) (product.Product, error) {
	var (
		p                                   product.Product
		productType                         string
		createdAt, updatedAt                time.Time
		pvcMaterial, pvcDesign, pvcColor    *string
		pvcTexture                          *string
		widthM, nominalLengthM, thicknessMM *string
		pricePerMeter, pricePerRoll         *string
		clothDesign, clothColor, clothDims  *string
		clothReferenceCost, clothUnitPrice  *string
		minimumStockM                       *string
		minimumUnits                        *int
	)
	err := row.Scan(&p.ID, &p.SKU, &p.Name, &productType, &p.IsActive, &p.ImageKey, &p.LabelCode,
		&createdAt, &updatedAt,
		&pvcMaterial, &pvcDesign, &pvcColor, &pvcTexture,
		&widthM, &nominalLengthM, &thicknessMM, &pricePerMeter, &pricePerRoll,
		&clothDesign, &clothColor, &clothDims, &clothReferenceCost, &clothUnitPrice,
		&minimumStockM, &minimumUnits)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return product.Product{}, err
		}
		return product.Product{}, fmt.Errorf("scan product: %w", err)
	}
	p.Type = product.Type(productType)
	p.CreatedAt = createdAt.UTC()
	p.UpdatedAt = updatedAt.UTC()

	switch p.Type {
	case product.TypePVC:
		details := &product.PVCDetails{
			Material: deref(pvcMaterial), Design: deref(pvcDesign),
			Color: deref(pvcColor), Texture: deref(pvcTexture),
		}
		if details.WidthM, err = parseRequired(widthM); err != nil {
			return product.Product{}, fmt.Errorf("width_m: %w", err)
		}
		if details.NominalLengthM, err = parseRequired(nominalLengthM); err != nil {
			return product.Product{}, fmt.Errorf("nominal_length_m: %w", err)
		}
		if details.ThicknessMM, err = parseOptional(thicknessMM); err != nil {
			return product.Product{}, fmt.Errorf("thickness_mm: %w", err)
		}
		if details.ReferencePricePerMeter, err = parseRequired(pricePerMeter); err != nil {
			return product.Product{}, fmt.Errorf("reference_price_per_meter: %w", err)
		}
		if details.ReferencePricePerRoll, err = parseRequired(pricePerRoll); err != nil {
			return product.Product{}, fmt.Errorf("reference_price_per_roll: %w", err)
		}
		if details.MinimumStockM, err = parseRequired(minimumStockM); err != nil {
			return product.Product{}, fmt.Errorf("minimum_stock_m: %w", err)
		}
		p.PVC = details
	case product.TypeTablecloth:
		details := &product.TableclothDetails{
			Design: deref(clothDesign), Color: deref(clothColor), Dimensions: deref(clothDims),
		}
		if details.ReferenceCost, err = parseOptional(clothReferenceCost); err != nil {
			return product.Product{}, fmt.Errorf("reference_cost: %w", err)
		}
		if details.UnitPrice, err = parseRequired(clothUnitPrice); err != nil {
			return product.Product{}, fmt.Errorf("unit_price: %w", err)
		}
		if minimumUnits != nil {
			details.MinimumUnits = *minimumUnits
		}
		p.Tablecloth = details
	default:
		return product.Product{}, fmt.Errorf("unknown product type %q in database", productType)
	}
	return p, nil
}

func parseRequired(text *string) (decimal.Decimal, error) {
	if text == nil {
		return decimal.Decimal{}, errors.New("missing value")
	}
	return decimal.Parse(*text)
}

func parseOptional(text *string) (*decimal.Decimal, error) {
	if text == nil {
		return nil, nil
	}
	value, err := decimal.Parse(*text)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func deref(text *string) string {
	if text == nil {
		return ""
	}
	return *text
}

func isSKUConflict(err error) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) && pgError.Code == pgUniqueViolation && pgError.ConstraintName == skuUniqueIndex
}

// escapeLike escapes the wildcard characters of a LIKE pattern.
func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}
