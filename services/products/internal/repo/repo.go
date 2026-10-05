package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("product not found")

type Product struct {
	ID          string
	Name        string
	Description string
	PriceCents  int64
	Currency    string
	CreatedAt   time.Time
}

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

const cols = "id::text, name, description, price_cents, currency, created_at"

type scanner interface {
	Scan(dest ...any) error
}

func scan(row scanner) (*Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.PriceCents, &p.Currency, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repo) Create(ctx context.Context, name, description string, priceCents int64, currency string) (*Product, error) {
	return scan(r.pool.QueryRow(ctx,
		"INSERT INTO products (name, description, price_cents, currency) VALUES ($1, $2, $3, $4) RETURNING "+cols,
		name, description, priceCents, currency))
}

func (r *Repo) GetByID(ctx context.Context, id string) (*Product, error) {
	return scan(r.pool.QueryRow(ctx, "SELECT "+cols+" FROM products WHERE id = $1::uuid", id))
}

func (r *Repo) query(ctx context.Context, sql string, args ...any) ([]*Product, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Product
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) GetByIDs(ctx context.Context, ids []string) ([]*Product, error) {
	return r.query(ctx, "SELECT "+cols+" FROM products WHERE id = ANY($1::uuid[])", ids)
}

func (r *Repo) List(ctx context.Context, limit, offset int32) ([]*Product, error) {
	return r.query(ctx,
		"SELECT "+cols+" FROM products ORDER BY created_at DESC, id LIMIT $1 OFFSET $2",
		limit, offset)
}