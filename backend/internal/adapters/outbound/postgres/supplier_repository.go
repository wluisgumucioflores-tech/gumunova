package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gumunova/backend/internal/application/ports"
	"gumunova/backend/internal/domain/supplier"
)

const selectSupplier = `
SELECT id::text, name, phone, email, notes, is_active, created_at, updated_at
FROM suppliers`

type SupplierRepository struct {
	pool *pgxpool.Pool
}

var _ ports.SupplierRepository = (*SupplierRepository)(nil)

func NewSupplierRepository(pool *pgxpool.Pool) *SupplierRepository {
	return &SupplierRepository{pool: pool}
}

func (r *SupplierRepository) Create(ctx context.Context, s supplier.Supplier) (supplier.Supplier, error) {
	created, err := scanSupplier(r.pool.QueryRow(ctx,
		`WITH inserted AS (
		   INSERT INTO suppliers (name, phone, email, notes, is_active)
		   VALUES ($1, $2, $3, $4, $5)
		   RETURNING id, name, phone, email, notes, is_active, created_at, updated_at)
		 SELECT id::text, name, phone, email, notes, is_active, created_at, updated_at FROM inserted`,
		s.Name, s.Phone, s.Email, s.Notes, s.IsActive))
	if err != nil {
		return supplier.Supplier{}, fmt.Errorf("insert supplier: %w", err)
	}
	return created, nil
}

func (r *SupplierRepository) Update(ctx context.Context, s supplier.Supplier) (supplier.Supplier, error) {
	if !uuidPattern.MatchString(s.ID) {
		return supplier.Supplier{}, supplier.ErrNotFound
	}
	updated, err := scanSupplier(r.pool.QueryRow(ctx,
		`WITH changed AS (
		   UPDATE suppliers
		   SET name = $2, phone = $3, email = $4, notes = $5, is_active = $6, updated_at = now()
		   WHERE id = $1
		   RETURNING id, name, phone, email, notes, is_active, created_at, updated_at)
		 SELECT id::text, name, phone, email, notes, is_active, created_at, updated_at FROM changed`,
		s.ID, s.Name, s.Phone, s.Email, s.Notes, s.IsActive))
	if errors.Is(err, pgx.ErrNoRows) {
		return supplier.Supplier{}, supplier.ErrNotFound
	}
	if err != nil {
		return supplier.Supplier{}, fmt.Errorf("update supplier: %w", err)
	}
	return updated, nil
}

func (r *SupplierRepository) GetByID(ctx context.Context, id string) (supplier.Supplier, error) {
	if !uuidPattern.MatchString(id) {
		return supplier.Supplier{}, supplier.ErrNotFound
	}
	found, err := scanSupplier(r.pool.QueryRow(ctx, selectSupplier+" WHERE id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return supplier.Supplier{}, supplier.ErrNotFound
	}
	if err != nil {
		return supplier.Supplier{}, fmt.Errorf("get supplier: %w", err)
	}
	return found, nil
}

func (r *SupplierRepository) List(ctx context.Context, filter ports.SupplierFilter) (ports.SupplierPage, error) {
	var conditions []string
	var args []any
	addCondition := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}
	if filter.IsActive != nil {
		addCondition("is_active = $%d", *filter.IsActive)
	}
	if query := strings.TrimSpace(filter.Query); query != "" {
		addCondition("(name ILIKE $%[1]d OR phone ILIKE $%[1]d OR email ILIKE $%[1]d)", "%"+escapeLike(query)+"%")
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM suppliers"+where, args...).Scan(&total); err != nil {
		return ports.SupplierPage{}, fmt.Errorf("count suppliers: %w", err)
	}

	pageArgs := append(append([]any{}, args...), filter.Limit, filter.Offset)
	listSQL := fmt.Sprintf("%s%s ORDER BY name, id LIMIT $%d OFFSET $%d",
		selectSupplier, where, len(args)+1, len(args)+2)
	rows, err := r.pool.Query(ctx, listSQL, pageArgs...)
	if err != nil {
		return ports.SupplierPage{}, fmt.Errorf("list suppliers: %w", err)
	}
	defer rows.Close()

	items := []supplier.Supplier{}
	for rows.Next() {
		item, err := scanSupplier(rows)
		if err != nil {
			return ports.SupplierPage{}, fmt.Errorf("scan supplier: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ports.SupplierPage{}, fmt.Errorf("read suppliers: %w", err)
	}
	return ports.SupplierPage{Items: items, Total: total}, nil
}

func scanSupplier(row rowScanner) (supplier.Supplier, error) {
	var s supplier.Supplier
	if err := row.Scan(&s.ID, &s.Name, &s.Phone, &s.Email, &s.Notes, &s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return supplier.Supplier{}, err
	}
	s.CreatedAt = s.CreatedAt.UTC()
	s.UpdatedAt = s.UpdatedAt.UTC()
	return s, nil
}
