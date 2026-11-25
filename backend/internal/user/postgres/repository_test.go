package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"

	"youxs/internal/user"
)

func newRootUser(email string) user.User {
	return user.User{
		Name:         "Administrador",
		Email:        email,
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA",
		Role:         user.RoleRoot,
	}
}

func TestCreate(t *testing.T) {
	db := openTestDB(t)
	repo := New(db)
	ctx := context.Background()

	in := newRootUser("admin@youxs.com")
	got, err := repo.Create(ctx, in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID <= 0 {
		t.Errorf("ID = %d, want > 0", got.ID)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}

	var name, email, hash, role string
	err = db.QueryRowContext(ctx,
		"SELECT name, email, password_hash, role FROM users WHERE id = $1", got.ID,
	).Scan(&name, &email, &hash, &role)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if name != in.Name || email != in.Email || hash != in.PasswordHash || role != "ROOT" {
		t.Errorf("row = (%q, %q, %q, %q), want (%q, %q, %q, ROOT)",
			name, email, hash, role, in.Name, in.Email, in.PasswordHash)
	}
}

func TestCreateSecondRootUser(t *testing.T) {
	db := openTestDB(t)
	repo := New(db)
	ctx := context.Background()

	if _, err := repo.Create(ctx, newRootUser("admin@youxs.com")); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if _, err := repo.Create(ctx, newRootUser("ops@youxs.com")); err != nil {
		t.Fatalf("second Create with a different e-mail: %v (FR-014)", err)
	}
	var roots int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE role = 'ROOT'").Scan(&roots); err != nil {
		t.Fatal(err)
	}
	if roots != 2 {
		t.Errorf("ROOT users = %d, want 2", roots)
	}
}

func TestEmailExists(t *testing.T) {
	db := openTestDB(t)
	repo := New(db)
	ctx := context.Background()

	exists, err := repo.EmailExists(ctx, "admin@youxs.com")
	if err != nil {
		t.Fatalf("EmailExists: %v", err)
	}
	if exists {
		t.Error("EmailExists = true on an empty table")
	}
	if _, err := repo.Create(ctx, newRootUser("admin@youxs.com")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	exists, err = repo.EmailExists(ctx, "admin@youxs.com")
	if err != nil {
		t.Fatalf("EmailExists: %v", err)
	}
	if !exists {
		t.Error("EmailExists = false after Create")
	}
}

func TestCreateDuplicateEmail(t *testing.T) {
	db := openTestDB(t)
	repo := New(db)
	ctx := context.Background()

	if _, err := repo.Create(ctx, newRootUser("admin@youxs.com")); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := repo.Create(ctx, newRootUser("admin@youxs.com"))
	if !errors.Is(err, user.ErrEmailAlreadyRegistered) {
		t.Fatalf("second Create err = %v, want ErrEmailAlreadyRegistered", err)
	}
	if n := countUsers(t, db); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

func TestCreateConcurrentSameEmail(t *testing.T) {
	db := openTestDB(t)
	repo := New(db)

	const workers = 2
	start := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := repo.Create(context.Background(), newRootUser("admin@youxs.com"))
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	var ok, dup int
	for err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, user.ErrEmailAlreadyRegistered):
			dup++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 || dup != 1 {
		t.Errorf("successes = %d, duplicates = %d; want exactly 1 and 1 (SC-004)", ok, dup)
	}
	if n := countUsers(t, db); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

func TestCreateCanceledContext(t *testing.T) {
	db := openTestDB(t)
	repo := New(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.Create(ctx, newRootUser("admin@youxs.com")); err == nil {
		t.Fatal("Create with a canceled context succeeded")
	}
	if n := countUsers(t, db); n != 0 {
		t.Errorf("rows = %d, want 0 (no partial record)", n)
	}
}

func TestEmailLowercaseConstraint(t *testing.T) {
	db := openTestDB(t)
	_, err := db.ExecContext(context.Background(),
		"INSERT INTO users (name, email, password_hash, role) VALUES ('A', 'Admin@Youxs.com', 'h', 'ROOT')")
	if err == nil {
		t.Fatal("INSERT with a mixed-case e-mail succeeded; the CHECK constraint is missing")
	}
}

func TestSessionTimeZone(t *testing.T) {
	db := openTestDB(t)
	var tz string
	if err := db.QueryRowContext(context.Background(), "SHOW timezone").Scan(&tz); err != nil {
		t.Fatal(err)
	}
	if tz != "America/Sao_Paulo" {
		t.Errorf("timezone = %q, want America/Sao_Paulo (Constitution VIII)", tz)
	}
}
