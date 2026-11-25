// Package postgres is the PostgreSQL adapter of the user module. It is the only
// place that contains SQL or imports pgx (Constitution III).
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver

	"youxs/internal/user"
)

// Repository implements user.Repository on top of database/sql.
type Repository struct {
	db *sql.DB
}

var _ user.Repository = (*Repository)(nil)

// New returns a Repository that uses db.
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// EmailExists reports whether a user with this e-mail exists. E-mails are
// always stored lowercased, so the caller passes the normalized e-mail.
func (r *Repository) EmailExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check e-mail: %w", err)
	}
	return exists, nil
}

// Create inserts the user inside a transaction (research R8) and returns it
// with ID and CreatedAt set by the database. Any error or context
// cancellation rolls the transaction back.
func (r *Repository) Create(ctx context.Context, u user.User) (user.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return user.User{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after a successful Commit

	err = tx.QueryRowContext(ctx,
		`INSERT INTO users (name, email, password_hash, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		u.Name, u.Email, u.PasswordHash, string(u.Role),
	).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		if isEmailUniqueViolation(err) {
			return user.User{}, user.ErrEmailAlreadyRegistered
		}
		return user.User{}, fmt.Errorf("insert user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		if isEmailUniqueViolation(err) {
			return user.User{}, user.ErrEmailAlreadyRegistered
		}
		return user.User{}, fmt.Errorf("commit: %w", err)
	}
	return u, nil
}

// uniqueViolation is the PostgreSQL SQLSTATE for unique_violation.
const uniqueViolation = "23505"

// isEmailUniqueViolation reports whether err is a unique violation on the
// users_email_key constraint (research R7).
func isEmailUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == uniqueViolation &&
		pgErr.ConstraintName == "users_email_key"
}
