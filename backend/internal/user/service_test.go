package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// fakeRepo is an in-memory Repository.
type fakeRepo struct {
	byEmail   map[string]User
	created   []User
	createErr error
	existsErr error
}

func newFakeRepo(existing ...string) *fakeRepo {
	r := &fakeRepo{byEmail: map[string]User{}}
	for _, e := range existing {
		r.byEmail[e] = User{Email: e, Role: RoleRoot}
	}
	return r
}

func (r *fakeRepo) EmailExists(_ context.Context, email string) (bool, error) {
	if r.existsErr != nil {
		return false, r.existsErr
	}
	_, ok := r.byEmail[email]
	return ok, nil
}

func (r *fakeRepo) Create(_ context.Context, u User) (User, error) {
	r.created = append(r.created, u)
	if r.createErr != nil {
		return User{}, r.createErr
	}
	u.ID = int64(len(r.created))
	u.CreatedAt = time.Now()
	r.byEmail[u.Email] = u
	return u, nil
}

func TestCreateRootValidInput(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	const password = "senhaSegura1"

	got, err := svc.CreateRoot(context.Background(), CreateRootInput{
		Name:     "  Administrador ",
		Email:    " Admin@Youxs.com ",
		Password: password,
	})
	if err != nil {
		t.Fatalf("CreateRoot: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("Create called %d times, want 1", len(repo.created))
	}
	stored := repo.created[0]
	if stored.Role != RoleRoot || got.Role != RoleRoot {
		t.Errorf("role = %q, want %q (FR-009)", stored.Role, RoleRoot)
	}
	if stored.Name != "Administrador" {
		t.Errorf("name = %q, want trimmed %q", stored.Name, "Administrador")
	}
	if stored.Email != "admin@youxs.com" {
		t.Errorf("email = %q, want trimmed and lowercased %q", stored.Email, "admin@youxs.com")
	}
	if stored.PasswordHash == password || strings.Contains(stored.PasswordHash, password) {
		t.Error("the stored hash reveals the password (FR-008)")
	}
	if !strings.HasPrefix(stored.PasswordHash, "$argon2id$") {
		t.Errorf("hash %q is not Argon2id", stored.PasswordHash)
	}
	if got.ID == 0 {
		t.Error("returned user has no ID")
	}
}

func TestCreateRootInvalidInputNeverCallsCreate(t *testing.T) {
	tests := []struct {
		name  string
		in    CreateRootInput
		field string
	}{
		{"empty name", CreateRootInput{Name: " ", Email: "a@b.com", Password: "12345678"}, FieldName},
		{"name too long", CreateRootInput{Name: strings.Repeat("x", 81), Email: "a@b.com", Password: "12345678"}, FieldName},
		{"bad e-mail", CreateRootInput{Name: "Ana", Email: "admin@", Password: "12345678"}, FieldEmail},
		{"short password", CreateRootInput{Name: "Ana", Email: "a@b.com", Password: "1234567"}, FieldPassword},
		{"long password", CreateRootInput{Name: "Ana", Email: "a@b.com", Password: strings.Repeat("1", 21)}, FieldPassword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepo()
			_, err := NewService(repo).CreateRoot(context.Background(), tt.in)
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected *ValidationError, got %v", err)
			}
			if ve.Field != tt.field {
				t.Errorf("Field = %q, want %q", ve.Field, tt.field)
			}
			if len(repo.created) != 0 {
				t.Error("Create was called with invalid input (FR-010)")
			}
		})
	}
}

func TestCheckEmailAvailable(t *testing.T) {
	ctx := context.Background()

	t.Run("taken, compared case-insensitively", func(t *testing.T) {
		svc := NewService(newFakeRepo("admin@youxs.com"))
		if err := svc.CheckEmailAvailable(ctx, " ADMIN@youxs.com "); !errors.Is(err, ErrEmailAlreadyRegistered) {
			t.Fatalf("err = %v, want ErrEmailAlreadyRegistered (FR-007)", err)
		}
	})
	t.Run("free", func(t *testing.T) {
		svc := NewService(newFakeRepo("admin@youxs.com"))
		if err := svc.CheckEmailAvailable(ctx, "ops@youxs.com"); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})
	t.Run("repository error is propagated", func(t *testing.T) {
		repo := newFakeRepo()
		repo.existsErr = errors.New("connection refused")
		err := NewService(repo).CheckEmailAvailable(ctx, "ops@youxs.com")
		if !errors.Is(err, repo.existsErr) {
			t.Fatalf("err = %v, want the repository error", err)
		}
		if errors.Is(err, ErrEmailAlreadyRegistered) {
			t.Fatal("a repository error must not look like a duplicate")
		}
	})
	t.Run("invalid e-mail", func(t *testing.T) {
		err := NewService(newFakeRepo()).CheckEmailAvailable(ctx, "admin@")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != FieldEmail {
			t.Fatalf("err = %v, want an e-mail *ValidationError", err)
		}
	})
}

func TestCreateRootDuplicateFromRepository(t *testing.T) {
	repo := newFakeRepo()
	repo.createErr = fmt.Errorf("insert user: %w", ErrEmailAlreadyRegistered)
	_, err := NewService(repo).CreateRoot(context.Background(), CreateRootInput{
		Name: "Ana", Email: "ana@youxs.com", Password: "senhaSegura1",
	})
	if !errors.Is(err, ErrEmailAlreadyRegistered) {
		t.Fatalf("err = %v, want ErrEmailAlreadyRegistered", err)
	}
}

func TestCreateRootPropagatesRepositoryError(t *testing.T) {
	repo := newFakeRepo()
	repo.createErr = errors.New("connection refused")
	_, err := NewService(repo).CreateRoot(context.Background(), CreateRootInput{
		Name: "Ana", Email: "ana@youxs.com", Password: "senhaSegura1",
	})
	if !errors.Is(err, repo.createErr) {
		t.Fatalf("err = %v, want the repository error", err)
	}
}
