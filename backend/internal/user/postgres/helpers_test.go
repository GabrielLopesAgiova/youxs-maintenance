package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

// openTestDB connects to TEST_DATABASE_URL and empties the users table. The
// test is skipped when the variable is not set, so unit runs need no database.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration test")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, "TRUNCATE users RESTART IDENTITY"); err != nil {
		t.Fatalf("truncate users: %v", err)
	}
	return db
}

// countUsers returns the number of rows in users.
func countUsers(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM users").Scan(&n); err != nil {
		t.Fatalf("count users: %v", err)
	}
	return n
}
