package user

import (
	"context"
	"errors"
	"fmt"
)

// ErrEmailAlreadyRegistered means the e-mail already belongs to a user
// (compared case-insensitively, FR-007).
var ErrEmailAlreadyRegistered = errors.New("user: e-mail already registered")

// Repository is the persistence port of the user module. It is implemented in
// internal/user/postgres; the domain never depends on database details.
type Repository interface {
	// EmailExists reports whether a user with this (normalized) e-mail exists.
	EmailExists(ctx context.Context, email string) (bool, error)
	// Create stores the user inside a transaction and returns it with ID and
	// CreatedAt set. It returns ErrEmailAlreadyRegistered on a duplicate.
	Create(ctx context.Context, u User) (User, error)
}

// Service holds the user use cases.
type Service struct {
	repo Repository
}

// NewService returns a Service backed by repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CheckEmailAvailable normalizes the e-mail and returns
// ErrEmailAlreadyRegistered if any user already has it (research R7). This is
// an early convenience check; the database constraint is the real guarantee.
func (s *Service) CheckEmailAvailable(ctx context.Context, email string) error {
	email, err := ValidateEmail(email)
	if err != nil {
		return err
	}
	exists, err := s.repo.EmailExists(ctx, email)
	if err != nil {
		return fmt.Errorf("check e-mail availability: %w", err)
	}
	if exists {
		return ErrEmailAlreadyRegistered
	}
	return nil
}

// CreateRoot validates the input again, hashes the password and stores a new
// user with the ROOT role (FR-008, FR-009). It returns a *ValidationError for
// invalid input, ErrEmailAlreadyRegistered for a duplicate e-mail, or the
// storage error.
func (s *Service) CreateRoot(ctx context.Context, in CreateRootInput) (User, error) {
	name, err := ValidateName(in.Name)
	if err != nil {
		return User{}, err
	}
	email, err := ValidateEmail(in.Email)
	if err != nil {
		return User{}, err
	}
	if err := ValidatePassword(in.Password); err != nil {
		return User{}, err
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	return s.repo.Create(ctx, User{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		Role:         RoleRoot,
	})
}
