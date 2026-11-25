# Data Model: ROOT User Registration

**Feature**: `001-root-user-registration` | **Date**: 2026-11-24 | **Plan**: [plan.md](plan.md)

## Entity: User

The domain type is `user.User`, defined in `internal/user`. The table is `users`.

| Field          | Go type     | Column          | DB type                                   | Rules |
|----------------|-------------|-----------------|-------------------------------------------|-------|
| ID             | `int64`     | `id`            | `BIGINT GENERATED ALWAYS AS IDENTITY` PK  | Assigned by the database. |
| Name           | `string`    | `name`          | `VARCHAR(80) NOT NULL`                    | Trimmed. Required. 1–80 characters, counted as runes (FR-003). |
| Email          | `string`    | `email`         | `VARCHAR(100) NOT NULL`, `UNIQUE`, `CHECK (email = lower(email))` | Trimmed and lowercased. Required. ≤ 100 characters. Valid format (FR-004, FR-005). Unique across all users (FR-007). |
| PasswordHash   | `string`    | `password_hash` | `TEXT NOT NULL`                           | Argon2id PHC string. The plain-text password is never stored (FR-008). |
| Role           | `user.Role` | `role`          | `VARCHAR(20) NOT NULL`, `CHECK (role IN ('ROOT'))` | Always `ROOT` for this feature (FR-009). |
| CreatedAt      | `time.Time` | `created_at`    | `TIMESTAMPTZ NOT NULL DEFAULT now()`      | Set by the database. |

PostgreSQL's `VARCHAR(n)` counts characters, not bytes, so the database limits match the rune-based validation in the application.

The plain-text password exists only in memory as `CreateRootInput.Password` while the command runs. It is never a field of `User`.

## Value type: Role

```text
type Role string
const RoleRoot Role = "ROOT"
```

This feature defines only `ROOT`. The `CHECK` constraint lists the allowed values. Adding a role later requires a new migration.

## Validation rules

All rules live in `internal/user`. The CLI uses them to re-prompt one field at a time, and the service runs them again before saving.

| Field    | Order of checks | Failure → `ValidationError{Field, Message}` (pt-BR) |
|----------|-----------------|-------------------------------------------------------|
| name     | 1. trim → empty? 2. runes > 80? | `O nome é obrigatório.` / `O nome deve ter no máximo 80 caracteres.` |
| email    | 1. trim → empty? 2. runes > 100? 3. format | `O e-mail é obrigatório.` / `O e-mail deve ter no máximo 100 caracteres.` / `O formato do e-mail é inválido.` |
| password | 1. empty? 2. runes < 8? 3. runes > 20? | `A senha é obrigatória.` / `A senha deve ter no mínimo 8 caracteres.` / `A senha deve ter no máximo 20 caracteres.` |
| password confirmation | differs from password | `As senhas não conferem.` (checked by the CLI, FR-015) |

These are the field-level checks. The e-mail format rule (R4) requires no whitespace, exactly one `@`, a non-empty local part, and a domain that contains a `.` and does not start or end with `.`. It must also be accepted by `net/mail.ParseAddress` with no display name.

## Domain errors

| Error | Meaning | Source |
|-------|---------|--------|
| `*user.ValidationError` | A field broke a rule. It carries `Field` and `Message`. | Validators |
| `user.ErrEmailAlreadyRegistered` | The e-mail is already in use (case-insensitive). | Service pre-check, or the repository mapping SQLSTATE `23505` |
| any other error | Storage failure, timeout or cancellation. | Repository |

## Persistence port

This interface is defined in `internal/user`, where it is used. It is implemented in `internal/user/postgres`.

```text
Repository
  EmailExists(ctx, email string) (bool, error)
  Create(ctx, u User) (User, error)   // runs inside a transaction; returns the user with ID/CreatedAt set
```

## State transitions

None. A user is created once, with role `ROOT`. Editing and deleting users are out of scope.

## Migration `migrations/0001_create_users.sql` (shape)

The file lives at `backend/migrations/0001_create_users.sql` (research R10).

```sql
CREATE TABLE users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          VARCHAR(80)  NOT NULL,
    email         VARCHAR(100) NOT NULL,
    password_hash TEXT         NOT NULL,
    role          VARCHAR(20)  NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT users_email_key       UNIQUE (email),
    CONSTRAINT users_email_lowercase CHECK (email = lower(email)),
    CONSTRAINT users_role_valid      CHECK (role IN ('ROOT'))
);
```
