// Package user is the users-and-roles domain module.
package user

import "time"

// Role is the privilege level of a user.
type Role string

// RoleRoot is the highest privilege level in the system.
const RoleRoot Role = "ROOT"

// User is a person who can access the system. It never holds the plain-text
// password, only its hash.
type User struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
}

// CreateRootInput is the raw data typed by the operator. It is the only place
// where the plain-text password exists, and only while the command runs.
type CreateRootInput struct {
	Name     string
	Email    string
	Password string
}
