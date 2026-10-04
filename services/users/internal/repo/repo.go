package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound   = errors.New("user not found")
	ErrEmailTaken = errors.New("email already registered")
)

type User struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	CreatedAt    time.Time
}

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

const cols = "id::text, email, name, password_hash, created_at"

func scan(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repo) Create(ctx context.Context, email, name, passwordHash string) (*User, error) {
	row := r.pool.QueryRow(ctx,
		"INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING "+cols,
		email, name, passwordHash)
	u, err := scan(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return u, nil
}

func (r *Repo) GetByEmail(ctx context.Context, email string) (*User, error) {
	return scan(r.pool.QueryRow(ctx, "SELECT "+cols+" FROM users WHERE email = $1", email))
}

func (r *Repo) GetByID(ctx context.Context, id string) (*User, error) {
	return scan(r.pool.QueryRow(ctx, "SELECT "+cols+" FROM users WHERE id = $1::uuid", id))
}

func (r *Repo) GetByIDs(ctx context.Context, ids []string) ([]*User, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+cols+" FROM users WHERE id = ANY($1::uuid[])", ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, rows.Err()
}
