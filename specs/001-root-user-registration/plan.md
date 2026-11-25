# Implementation Plan: ROOT User Registration

**Branch**: `001-root-user-registration` | **Date**: 2026-11-24 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/001-root-user-registration/spec.md`

**Revision 2026-11-24 (repository layout)**: The Go code moves into `backend/`, the plan reserves locations for a future HTTP server, and Docker is reduced to two images (`postgres` and `golang`). The behavior defined in the spec does not change. Details are in [research.md](research.md) R1, R9, R10 and R13.

## Summary

An interactive Go command, `create-root`, asks for a name, an e-mail and a password (entered twice, without echo). It validates each field as it is typed and re-prompts on errors, then creates a user with role `ROOT` in PostgreSQL inside a transaction. The password is stored only as an Argon2id hash. The creation logic (validation, hashing, uniqueness) lives in the `internal/user` domain module. SQL lives only in the `internal/user/postgres` infrastructure package. The `cmd/create-root` entry point handles only terminal I/O and wiring. A `UNIQUE` constraint on the lowercased e-mail guarantees no duplicates, even when two executions run at once.

All Go code lives in `backend/`. Docker Compose provides two services: `db` (PostgreSQL) and `go` (the Go toolchain). The `go` service runs the tests and also runs the command itself through `go run`, so the project needs no image of its own. Details are in [research.md](research.md).

## Technical Context

**Language/Version**: Go 1.26, run in `golang:1.26-alpine` for tests and for the command itself (R1)

**Primary Dependencies**: standard library (`database/sql`, `net/mail`, `unicode/utf8`, `crypto/rand`, `bufio`, `os/signal`, `time/tzdata`), `github.com/jackc/pgx/v5` (driver through `database/sql`, R2), `golang.org/x/crypto/argon2` (R3) and `golang.org/x/term` (hidden input, R5)

**Storage**: PostgreSQL 18 (`postgres:18-alpine`) with one table, `users` ([data-model.md](data-model.md)). The schema comes from plain SQL files in `backend/migrations/` (R10).

**Testing**: the standard `testing` package. Unit tests need no database. Repository integration tests run when `TEST_DATABASE_URL` is set (R12).

**Target Platform**: Linux containers (Docker Compose). The command runs with `docker compose run --rm go go run ./cmd/create-root` (R13).

**Project Type**: CLI tool, the first module of the YOUXS backend

**Performance Goals**: interactive use. One run takes under 1 minute including typing (SC-001); `go run` adds a few seconds of compilation, which is cached between runs. Hashing takes about 100 ms or less with 64 MiB of memory.

**Constraints**: interactive only, and stdin must be a TTY (FR-002). No HTTP endpoint. The password is never echoed, logged or stored in plain text. User messages are in pt-BR. Configuration comes only from `DATABASE_URL`. Every date and time uses Brasília time (Constitution VIII): `TZ=America/Sao_Paulo` for the `go` service, and the PostgreSQL server `timezone` set to `America/Sao_Paulo`. `created_at` is `TIMESTAMPTZ`. The command embeds the time zone database by importing `time/tzdata`, so it does not depend on the image having tz files (R1).

**Scale/Scope**: a handful of ROOT users. One table, one command, about six Go source files.

All earlier NEEDS CLARIFICATION items are resolved in [research.md](research.md).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Assessment | Status |
|-----------|------------|--------|
| I. Endpoints delegate to application services | There are no HTTP endpoints (spec restriction). The CLI follows the same rule anyway: `cmd/create-root` only calls `user.Service` and never touches `*sql.DB` beyond constructing the repository. The reserved `internal/user/http` location will follow this rule when a future feature adds endpoints. | ✅ N/A (respected) |
| II. Business rules out of the HTTP layer | All rules (limits, format, uniqueness, role, hashing) live in `internal/user`. The CLI only reads input, shows messages, and compares password and confirmation (an input concern, FR-015). | ✅ |
| III. Isolated persistence layer | SQL and pgx are imported only in `internal/user/postgres`. The domain depends on its own `Repository` interface. SQL is used directly, so the justification is in Complexity Tracking. | ✅ (justified) |
| IV. Mandatory tests | Unit tests cover every validation rule, the service paths and the CLI prompt loop, with no database. There is no endpoint, so the endpoint integration-test rule does not apply. Repository integration tests against PostgreSQL cover the transaction, uniqueness and concurrency (R12). | ✅ |
| V. Standardized validation errors (HTTP 400) | No HTTP surface. The CLI keeps the same idea: every validation error carries the field and a message (`ValidationError{Field, Message}`). | ✅ N/A |
| VI. Code language and messages | Identifiers, packages, files and folders are in English and follow Go conventions. Every operator-facing message is in pt-BR ([contract](contracts/create-root-cli.md#messages-pt-br-fr-012)). | ✅ |
| VII. Modular organization by domain | One business module: `internal/user` (users and roles). `internal/user/postgres` is its infrastructure adapter. `backend/` is a repository folder, not a Go package or module of the application; it is justified in Complexity Tracking. There are no technical-only modules: configuration stays in `cmd/create-root`, and hashing is part of `internal/user`. | ✅ (justified) |
| VIII. Brasília time zone | `users.created_at` is `TIMESTAMPTZ DEFAULT now()`. Compose sets `TZ=America/Sao_Paulo` on both services and starts PostgreSQL with `-c timezone=America/Sao_Paulo`. The command embeds tz data (`import _ "time/tzdata"`). The zone is set by its IANA name, never a fixed offset. | ✅ |
| Commit conventions | Tasks will use the form `feat(user): ... (FR-00X)`. Docker, layout and migration commits reference the spec or task ID. | ✅ |

**Gate result (pre-research)**: PASS. **Gate result (post-design)**: PASS. The layout revision adds no new modules, dependencies or layers; it removes one image.

## Project Structure

### Documentation (this feature)

```text
specs/001-root-user-registration/
├── plan.md              # This file
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   └── create-root-cli.md
├── checklists/
│   └── requirements.md
└── tasks.md             # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
backend/                       # all Go code; the Go module root (module path: youxs)
├── cmd/
│   ├── create-root/
│   │   ├── main.go            # wiring: args/TTY check, DATABASE_URL, sql.Open + ping, signals, exit codes, time/tzdata
│   │   ├── prompt.go          # interactive loop: per-field read → user.Validate* → re-prompt; password + confirmation
│   │   └── prompt_test.go     # fake reader/writer/password reader: re-prompt, mismatch, messages, exit codes
│   └── api/                   # RESERVED for a future HTTP server; not created by this feature
├── internal/
│   └── user/                  # domain module: users and roles
│       ├── user.go            # User, Role, RoleRoot, CreateRootInput
│       ├── validation.go      # ValidateName, ValidateEmail, ValidatePassword, ValidationError (pt-BR messages)
│       ├── validation_test.go
│       ├── password.go        # hashPassword: Argon2id → PHC string
│       ├── password_test.go
│       ├── service.go         # Repository interface, Service{CheckEmailAvailable, CreateRoot}, ErrEmailAlreadyRegistered
│       ├── service_test.go    # in-memory fake repository
│       ├── postgres/          # infrastructure: the only place with SQL and pgx
│       │   ├── repository.go  # EmailExists, Create (BeginTx/INSERT/Commit, 23505 → ErrEmailAlreadyRegistered)
│       │   ├── repository_test.go   # integration; skipped without TEST_DATABASE_URL
│       │   └── helpers_test.go      # openTestDB, countUsers
│       └── http/              # RESERVED for future HTTP handlers of the user module; not created by this feature
├── migrations/
│   └── 0001_create_users.sql
└── go.mod / go.sum

docker/
└── postgres/
    └── initdb.sh              # creates youxs_test and applies /migrations/*.sql to youxs and youxs_test

compose.yaml                   # services: db (postgres:18-alpine, healthcheck), go (golang:1.26-alpine; tests and create-root)
.gitignore
```

Removed by this revision: `Dockerfile`, `.dockerignore` and the `create-root` Compose service.

**Structure Decision**: The repository root holds orchestration (`compose.yaml`, `docker/`) and the spec-kit folders (`specs/`, `.specify/`, `.claude/`). All Go code lives in `backend/`, which is the Go module root and follows the standard `cmd/` plus `internal/` layout. `cmd/create-root` is the delivery mechanism (CLI I/O only). `internal/user` is the domain and application module. Its `postgres` subpackage is the infrastructure adapter. Tests sit next to the code, as is usual in Go. There is no `pkg/`, no shared `utils`, and no separate config package.

**Reserved for HTTP (future feature, out of scope for spec 001)**: a server entry point at `backend/cmd/api/` and the user module's handlers at `backend/internal/user/http/`. Handlers will call `user.Service` (Principles I and II), and validation errors will map to HTTP 400 (Principle V). These folders are not created now: empty folders are not tracked by git and are not Go packages.

## Execution flow (reference for tasks)

1. `main` checks for no arguments and a TTY stdin, reads `DATABASE_URL`, opens `sql.Open("pgx", url)`, calls `PingContext(5s)`, and sets up `signal.NotifyContext` and the saved terminal state.
2. `prompt` loops: name → `user.ValidateName`, then e-mail → `user.ValidateEmail` → `svc.CheckEmailAvailable` (fatal if the e-mail is taken), then password → `user.ValidatePassword`, then confirmation (on mismatch, the password is asked again).
3. `svc.CreateRoot(ctx, CreateRootInput)` validates all fields again, calls `hashPassword`, then `repo.Create`, which runs in a transaction.
4. `main` maps the result to a [contract](contracts/create-root-cli.md) message and an exit code.

## Complexity Tracking

| Violation / Justification required | Why Needed | Simpler Alternative Rejected Because |
|------------------------------------|------------|--------------------------------------|
| Direct SQL in `internal/user/postgres` (required by Principle III) | Two statements: `SELECT EXISTS(...)` and `INSERT ... RETURNING id, created_at`, plus the unique-violation code. Plain SQL through `database/sql` is the idiomatic Go approach and needs no extra dependency. | An ORM or query builder adds a dependency and hides the `23505` handling that the concurrency guarantee depends on. |
| `user.Repository` interface | Principle III forbids the domain from depending on database details. It also makes the service testable without a database. | Calling the postgres package from the service would couple the domain to pgx and `database/sql`. |
| Non-stdlib dependencies (`pgx`, `x/crypto`, `x/term`) | The standard library has no PostgreSQL driver, no Argon2id, and no portable way to turn off echo. | PBKDF2 from the standard library is weaker (R3). Raw `syscall` code for echo is not portable (R5). |
| `backend/` organizational folder (Principle VII asks to justify technical grouping) | Separates the Go module from orchestration and spec-kit files, and leaves room for other top-level parts (for example a frontend) without mixing them with Go code. It is a repository folder, not an application module: the business modules inside it are still organized by domain. | Keeping Go code at the root mixes `go.mod`, `cmd/` and `internal/` with `specs/`, `.specify/` and Docker files, and a future frontend would have no clear home. |
