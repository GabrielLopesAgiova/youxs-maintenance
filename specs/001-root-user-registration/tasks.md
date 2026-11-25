---

description: "Task list for ROOT User Registration"
---

# Tasks: ROOT User Registration

**Input**: Design documents from `/specs/001-root-user-registration/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/create-root-cli.md](contracts/create-root-cli.md), [quickstart.md](quickstart.md)

**Tests**: REQUIRED. Constitution Principle IV (non-negotiable) requires a unit test for every business rule. Repository integration tests cover the transaction and uniqueness guarantees ([research.md](research.md) R12). Within each phase, write the tests first and confirm they fail before implementing.

**Organization**: Tasks are grouped by user story, so each story can be implemented and tested on its own.

**Toolchain**: Go is not installed on the host. Run every `go` command through Docker: `docker compose run --rm go go <args>` (service defined in T002).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: The user story the task belongs to (US1, US2)
- Paths are relative to the repository root, following the layout in [plan.md](plan.md#source-code-repository-root)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Docker environment, Go module and database schema

- [X] T001 Create the directory layout from plan.md (`cmd/create-root/`, `internal/user/postgres/`, `migrations/`, `docker/postgres/`) and add `.dockerignore` excluding `.git`, `.specify`, `.claude` and `specs`
- [X] T002 Create `compose.yaml` with three services. (1) `db`: `postgres:18-alpine`. Set `POSTGRES_USER`, `POSTGRES_PASSWORD` and `POSTGRES_DB` to `youxs` and `TZ=America/Sao_Paulo`. Use `command: ["postgres", "-c", "timezone=America/Sao_Paulo"]`. Add a `pg_isready` healthcheck and a named volume. Mount `./migrations:/migrations:ro` and `./docker/postgres/initdb.sh:/docker-entrypoint-initdb.d/initdb.sh:ro`. (2) `go`: `golang:1.26-alpine`, with the repository mounted at `/src`, `working_dir: /src`, a named volume for the module cache, `TZ=America/Sao_Paulo`, `TEST_DATABASE_URL=postgres://youxs:youxs@db:5432/youxs_test?sslmode=disable`, and `depends_on: db` (condition `service_healthy`). (3) `create-root`: built from `Dockerfile`, with `stdin_open: true`, `tty: true`, `TZ=America/Sao_Paulo`, `DATABASE_URL=postgres://youxs:youxs@db:5432/youxs?sslmode=disable`, and `depends_on: db` (healthy). Satisfies plan Technical Context and Constitution VIII.
- [X] T003 [P] Create a multi-stage `Dockerfile`. Build stage: `golang:1.26-alpine`, running `CGO_ENABLED=0 go build -tags timetzdata -o /out/create-root ./cmd/create-root`. Runtime stage: `gcr.io/distroless/static-debian12:nonroot` with `ENV TZ=America/Sao_Paulo`, `/create-root` as the entrypoint, and a non-root user (Constitution VIII; research R1)
- [X] T004 Initialize the Go module with `docker compose run --rm go go mod init youxs`, then set `go 1.26` in `go.mod`. Add the dependencies with `go get github.com/jackc/pgx/v5 golang.org/x/crypto golang.org/x/term`, which produces `go.mod` and `go.sum` (research R2, R3, R5). Depends on T002.
- [X] T005 [P] Write `migrations/0001_create_users.sql` creating table `users`. Columns: data-model.md User.id, User.name, User.email, User.password_hash, User.role, User.created_at. Constraints: `users_email_key` (User.email unique), `users_email_lowercase` (User.email lowercase CHECK) and `users_role_valid` (User.role CHECK). Follow the shape in [data-model.md](data-model.md#migration-migrations0001_create_userssql-shape).
- [X] T006 [P] Write `docker/postgres/initdb.sh` (POSIX `sh`, `set -eu`). It runs `CREATE DATABASE youxs_test`, then applies every `/migrations/*.sql` file in lexical order to both `youxs` and `youxs_test` with `psql -v ON_ERROR_STOP=1` (research R10)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Domain types, field validation and the persistence port that both stories depend on

**⚠️ CRITICAL**: Do not start user story work until this phase is complete.

- [X] T007 [P] Create `internal/user/user.go` with: type `Role string`, `const RoleRoot Role = "ROOT"` (data-model.md Role, User.role); struct `User` with fields `ID int64`, `Name`, `Email`, `PasswordHash string`, `Role Role`, `CreatedAt time.Time` (data-model.md User.id, User.name, User.email, User.password_hash, User.role, User.created_at); and struct `CreateRootInput{Name, Email, Password string}`. The plain-text password exists only in `CreateRootInput`, never in `User`.
- [X] T008 [P] Write table-driven tests in `internal/user/validation_test.go`. Cover `ValidateName` (data-model.md User.name): empty, spaces only, 80 and 81 runes, `"  João  "` normalized to `"João"`. Cover `ValidateEmail` (data-model.md User.email): empty, 100 and 101 runes, `admin@`, `a@b`, `a b@c.com`, `Nome <a@b.com>`, `a@@b.com`, `a@.com`, `a@b.com.`, and `" Admin@Youxs.com "` normalized to `"admin@youxs.com"`. Cover `ValidatePassword` (data-model.md Validation rules, password): empty, 7, 8, 20 and 21 runes, and a password with leading spaces that must not be trimmed. Each failure must return `*ValidationError` with the expected `Field` and the exact pt-BR `Message` from data-model.md.
- [X] T009 Implement `internal/user/validation.go`: `type ValidationError struct{ Field, Message string }` with an `Error()` method; `ValidateName(string) (string, error)` (data-model.md User.name); `ValidateEmail(string) (string, error)` (data-model.md User.email, with the format rule from research R4); and `ValidatePassword(string) error` (data-model.md Validation rules, password). Use only `strings`, `unicode/utf8` and `net/mail`. Make T008 pass. Depends on T007 and T008.
- [X] T010 Create `internal/user/service.go` containing only the declarations: `var ErrEmailAlreadyRegistered = errors.New(...)`; the `Repository` interface with `EmailExists(ctx context.Context, email string) (bool, error)` and `Create(ctx context.Context, u User) (User, error)` ([data-model.md](data-model.md#persistence-port)); and `type Service struct{ repo Repository }` with `NewService(repo Repository) *Service`. Depends on T007.
- [X] T011 [P] Create the integration test helper in `internal/user/postgres/helpers_test.go`. `openTestDB(t)` reads `TEST_DATABASE_URL` and calls `t.Skip` when it is empty. It opens `sql.Open("pgx", url)`, runs `TRUNCATE users RESTART IDENTITY` before each test, and registers `t.Cleanup` to close the connection. Depends on T004 and T005.

**Checkpoint**: `docker compose run --rm go go test ./internal/user/...` passes, and the validation tests are green.

---

## Phase 3: User Story 1 - Create a ROOT user from the terminal (Priority: P1) 🎯 MVP

**Goal**: An operator runs `create-root`, enters a name, an e-mail and a password (twice, hidden), and gets a ROOT user stored with an Argon2id hash and the message `Usuário ROOT criado com sucesso.`

**Independent Test**: On an empty database, run `docker compose run --rm create-root` with valid data. Check that the success message appears, the exit status is 0, and a `users` row exists with role `ROOT` and a `$argon2id$` hash ([quickstart.md](quickstart.md) step 3).

### Tests for User Story 1 ⚠️

> Write these tests first and make sure they fail before implementing.

- [X] T012 [P] [US1] Write tests in `internal/user/password_test.go` for `hashPassword` (data-model.md User.password_hash). The result starts with `$argon2id$v=19$m=65536,t=3,p=4$` and has 6 `$`-separated parts with base64 (raw std) salt and hash. Two hashes of the same password differ (random salt). The hash does not contain the plain-text password.
- [X] T013 [P] [US1] Write tests in `internal/user/service_test.go` for `Service.CreateRoot`, using an in-memory fake `Repository` that records the `User` passed to `Create`. With valid input, the call returns the created user with `Role == RoleRoot` (FR-009), `Email` trimmed and lowercased (data-model.md User.email), `Name` trimmed (data-model.md User.name), and a `PasswordHash` different from the password with an `$argon2id$` prefix (FR-008). With invalid input, it returns `*ValidationError` and `Create` is never called (FR-010).
- [X] T014 [P] [US1] Write integration tests in `internal/user/postgres/repository_test.go`. `TestCreate`: inserting a user returns `ID > 0` and a non-zero `CreatedAt`, and reading the row back gives the same name, e-mail, hash and role `ROOT`. `TestCreateSecondRootUser`: after one ROOT user exists, `Create` with a different e-mail succeeds and the table holds 2 rows with role `ROOT` (FR-014). `TestEmailExists`: returns false on an empty table and true after a `Create` with the same e-mail. `TestEmailLowercaseConstraint`: a direct `INSERT` with a mixed-case e-mail fails (data-model.md User.email CHECK). `TestSessionTimeZone`: `SHOW timezone` returns `America/Sao_Paulo` (Constitution VIII). Use `openTestDB` from T011.
- [X] T015 [P] [US1] Write the happy-path test in `cmd/create-root/prompt_test.go`. Build a prompter with a `strings.Reader` holding name and e-mail lines, a fake `readPassword` that returns the password on both calls, two `bytes.Buffer`s (`out` for prompts and field messages, `errOut` for fatal messages, per [contracts/create-root-cli.md](contracts/create-root-cli.md#messages-pt-br-fr-012)), and a fake service that implements only `CreateRoot`. Check that `out` contains, in order, `Nome: `, `E-mail: `, `Senha: `, `Confirme a senha: ` and `Usuário ROOT criado com sucesso.`; that `errOut` is empty; that `readPassword` was called exactly twice (FR-013, FR-015); that neither buffer contains the password; that the `ctx` the fake `CreateRoot` receives has a deadline (10 s per-call timeout, research R9); and that the exit code is 0 (FR-011).

### Implementation for User Story 1

- [X] T016 [P] [US1] Implement unexported `hashPassword(password string) (string, error)` in `internal/user/password.go`. Use `golang.org/x/crypto/argon2.IDKey` with memory 64 MiB, iterations 3, parallelism 4, a 16-byte salt from `crypto/rand` and a 32-byte key, and encode the result as a PHC string (research R3; data-model.md User.password_hash). Make T012 pass.
- [X] T017 [US1] Implement `Service.CreateRoot(ctx context.Context, in CreateRootInput) (User, error)` in `internal/user/service.go`. Call `ValidateName`, `ValidateEmail` and `ValidatePassword` again; call `hashPassword`; build a `User` with `Role: RoleRoot` and the normalized values; return `repo.Create(ctx, u)`. Wrap the hashing error without including the password. Make T013 pass. Depends on T009, T010 and T016.
- [X] T018 [P] [US1] Implement `internal/user/postgres/repository.go`: `type Repository struct{ db *sql.DB }`, `New(db *sql.DB) *Repository`, `EmailExists(ctx, email string) (bool, error)` using `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, and `Create(ctx, u user.User) (user.User, error)`. Both methods are needed here so that `*postgres.Repository` satisfies `user.Repository` (T010) and T020 compiles in US1. `Create` runs `db.BeginTx`, then `defer tx.Rollback()`, then `INSERT INTO users (name, email, password_hash, role) VALUES ($1,$2,$3,$4) RETURNING id, created_at`, then `tx.Commit()` (research R8; data-model.md User.id, User.created_at). Add a blank import of `github.com/jackc/pgx/v5/stdlib` in this package. Make T014 pass. Depends on T010 and T011.
- [X] T019 [US1] Implement the prompter in `cmd/create-root/prompt.go`. Define `type prompter struct{ in *bufio.Reader; out io.Writer; errOut io.Writer; readPassword func() ([]byte, error) }` (`out` for prompts and field messages, `errOut` for fatal messages) and a small consumer-side interface `rootCreator` that declares only `CreateRoot` for now (T026 adds `CheckEmailAvailable` once T024 exists). Add `const dbTimeout = 10 * time.Second` (research R9). Add `readLine(label)`, which writes the label and trims the trailing newline, and `readSecret(label)`, which writes the label, calls `readPassword` and then writes a newline. Add `run(ctx, svc) int`, which reads name (`ValidateName`), e-mail (`ValidateEmail`), password (`ValidatePassword`) and confirmation, calls `svc.CreateRoot` with `context.WithTimeout(ctx, dbTimeout)`, prints `Usuário ROOT criado com sucesso.` to `out` and returns 0. Prompts and success message must match [contracts/create-root-cli.md](contracts/create-root-cli.md). Make T015 pass. Depends on T009 and T017.
- [X] T020 [US1] Implement `cmd/create-root/main.go`. If `len(os.Args) > 1`, print `Este comando não aceita argumentos.` to stderr and exit 1. If `term.IsTerminal(int(os.Stdin.Fd()))` is false, print `Este comando deve ser executado em um terminal interativo.` and exit 1. If `DATABASE_URL` is empty, print `Variável de ambiente DATABASE_URL não definida.` and exit 1. Open `sql.Open("pgx", url)` and call `PingContext` with a 5 s timeout; on failure, print `Não foi possível criar o usuário ROOT: o banco de dados não está acessível.` and exit 1. Then wire `postgres.New`, `user.NewService` and the prompter, with `out` set to `os.Stdout`, `errOut` set to `os.Stderr` and `readPassword` set to `term.ReadPassword(int(os.Stdin.Fd()))`, and `os.Exit(p.run(ctx, svc))`. Every startup message is the exact contract text, written to stderr. The per-call 10 s timeouts live in `prompt.go` (T019, T026), not here. Depends on T018 and T019.

**Checkpoint**: Unit and integration tests pass. [quickstart.md](quickstart.md) step 3 succeeds: the ROOT user is created, the password is hidden, and the hash is Argon2id.

---

## Phase 4: User Story 2 - Clear feedback when the data is invalid (Priority: P2)

**Goal**: Each invalid field shows a pt-BR message and is asked again. A password mismatch asks for both passwords again. A duplicate e-mail (case-insensitive, including concurrent runs), a storage failure, an interruption or EOF ends the command with status 1 and creates no user.

**Independent Test**: Run the inputs from [quickstart.md](quickstart.md) step 4 and the failure paths in step 5. Each message must match [contracts/create-root-cli.md](contracts/create-root-cli.md), the same field must be asked again, and the `users` count must not change on failure.

### Tests for User Story 2 ⚠️

- [X] T021 [P] [US2] Add tests to `internal/user/service_test.go`. `CheckEmailAvailable`: when the fake repo already holds `admin@youxs.com`, the input `" ADMIN@youxs.com "` returns `ErrEmailAlreadyRegistered` (FR-007; data-model.md User.email); a free e-mail returns nil; a repo error is propagated. `CreateRoot`: when the fake `Create` returns `ErrEmailAlreadyRegistered`, `errors.Is` holds on the returned error.
- [X] T022 [P] [US2] Add integration tests to `internal/user/postgres/repository_test.go` (`TestEmailExists` already exists from T014). `TestCreateDuplicateEmail`: a second `Create` with the same e-mail returns `user.ErrEmailAlreadyRegistered` and the row count stays 1. `TestCreateConcurrentSameEmail`: two goroutines call `Create` with the same e-mail at the same time; exactly one succeeds and the other returns `ErrEmailAlreadyRegistered` (SC-004). `TestCreateCanceledContext`: a canceled context returns an error and leaves no row.
- [X] T023 [P] [US2] Add table-driven tests to `cmd/create-root/prompt_test.go`. Use separate `out` and `errOut` buffers, as in T015, and assert each message on the stream that [contracts/create-root-cli.md](contracts/create-root-cli.md#messages-pt-br-fr-012) assigns to it. The fake service now also implements `CheckEmailAvailable`. Each case feeds one invalid value, then a valid one, and checks that the exact message from data-model.md appears on `out` followed by the same prompt again (spec US2 scenarios 1–5, 7 and 9). Also cover:
  - Confirmation mismatch: `As senhas não conferem.` appears on `out`, then `Senha: ` is asked again (scenario 8, FR-015).
  - E-mail already registered: `O e-mail informado já está cadastrado. Nenhum usuário foi criado.` appears on `errOut`, the exit code is 1, and `readPassword` is never called (scenario 6).
  - `CheckEmailAvailable` returns any other error: `Não foi possível criar o usuário ROOT: o banco de dados não está acessível.` appears on `errOut`, the exit code is 1, `readPassword` and `CreateRoot` are never called, and the underlying error text is not printed (FR-010).
  - The `ctx` the fake `CheckEmailAvailable` receives has a deadline (research R9).
  - `CreateRoot` returns `ErrEmailAlreadyRegistered`: the duplicate message on `errOut`, exit code 1.
  - `CreateRoot` returns any other error: `Não foi possível criar o usuário ROOT: falha ao gravar os dados.` on `errOut`, exit code 1, and the underlying error text is not printed.
  - `CreateRoot` returns an error wrapping `context.Canceled` while the parent `ctx` is canceled: `Operação cancelada. Nenhum usuário foi criado.` on `errOut`, exit code 1.
  - The parent `ctx` is canceled before `CreateRoot` would be called: the cancel message on `errOut`, exit code 1, and `CreateRoot` is never called.
  - `CreateRoot` succeeds although the parent `ctx` is canceled during the call: the success message on `out` and exit code 0 (the result is reported as it happened).
  - EOF at any prompt: `Operação cancelada. Nenhum usuário foi criado.` on `errOut`, exit code 1.
  - In every case, neither buffer contains the password.

### Implementation for User Story 2

- [X] T024 [US2] Implement `Service.CheckEmailAvailable(ctx, email string) error` in `internal/user/service.go`. Normalize the e-mail with `ValidateEmail`, call `repo.EmailExists`, and return `ErrEmailAlreadyRegistered` when the e-mail exists (research R7). Make T021 pass.
- [X] T025 [P] [US2] In `internal/user/postgres/repository.go` (where `EmailExists` already exists from T018), in `Create`, map a `*pgconn.PgError` with `Code == "23505"` and `ConstraintName == "users_email_key"` to `user.ErrEmailAlreadyRegistered`, using `errors.As` (research R7; data-model.md User.email unique). Make T022 pass.
- [X] T026 [US2] Extend `cmd/create-root/prompt.go`:
  - Add `CheckEmailAvailable(ctx context.Context, email string) error` to the `rootCreator` interface (T024 makes `*user.Service` satisfy it).
  - Wrap each field read in a loop. On `*user.ValidationError`, print `Message` to `out` and ask the same field again (FR-010).
  - After a valid e-mail, call `svc.CheckEmailAvailable` with `context.WithTimeout(ctx, dbTimeout)`. On `ErrEmailAlreadyRegistered`, print the duplicate message to `errOut` and return 1. On any other error (unreachable database, timeout), print `Não foi possível criar o usuário ROOT: o banco de dados não está acessível.` to `errOut` without the underlying error text and return 1 (FR-010, spec edge case "data store unavailable").
  - On a confirmation mismatch, print `As senhas não conferem.` to `out` and ask for the password and its confirmation again.
  - Guard the create step against interruption: add fields `mu sync.Mutex` and `creating bool`. Just before `CreateRoot`, lock `mu`. If `ctx.Err() != nil`, unlock, print the cancel message and return 1 without calling `CreateRoot`. Otherwise set `creating = true` and unlock. Add `canExitOnSignal() bool`, which locks `mu` and returns `!creating`. T027 uses it.
  - Map `CreateRoot` errors to messages on `errOut`, all returning 1: `ErrEmailAlreadyRegistered` gives the duplicate message. `context.Canceled` (checked with `errors.Is`) while the parent `ctx` is done gives `Operação cancelada. Nenhum usuário foi criado.`. Any other error gives the generic save-failure message. When `CreateRoot` returns nil, print the success message and return 0, even if a signal arrived during the call.
  - Map `io.EOF` from any read to `Operação cancelada. Nenhum usuário foi criado.` on `errOut` and return 1.
  - Make T023 pass. Depends on T024.
- [X] T027 [US2] Add interrupt handling to `cmd/create-root/main.go`:
  - Save the terminal state with `term.GetState(fd)` before the first prompt.
  - Create `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`.
  - Start a goroutine that waits on `ctx.Done()`, then calls `term.Restore(fd, state)`. If `p.canExitOnSignal()` is true (the command is at a prompt or in the e-mail check), it prints a newline and `Operação cancelada. Nenhum usuário foi criado.` to stderr and calls `os.Exit(1)`. If it is false, a create is in flight: the goroutine returns, and `run` reports the real outcome from T026. This keeps the contract guarantee that a non-zero exit means no row from this run (research R6).
  - Pass `ctx` into `run`, so an in-flight `Create` is canceled and its transaction rolled back.
  - Depends on T020 and T026.

**Checkpoint**: All tests pass. [quickstart.md](quickstart.md) steps 4 and 5 behave as documented, and the terminal echo is restored after Ctrl+C.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Quality gates and end-to-end validation

- [X] T028 Run `docker compose run --rm go gofmt -l .` (expect no output), `docker compose run --rm go go vet ./...` and `docker compose run --rm go go test -race ./...`. Fix any findings in the affected files.
- [X] T029 [P] Security review of `cmd/create-root/` and `internal/user/`. Check that no `fmt`, `log` or error wrap includes the password, the confirmation or the hash (FR-008); that no HTTP handler or `net/http` import exists (spec restriction); and that SQL and pgx are imported only in `internal/user/postgres` (Constitution III).
- [X] T030 Run every step of `specs/001-root-user-registration/quickstart.md` manually against `docker compose`. Also confirm Constitution VIII: `docker compose exec db psql -U youxs -d youxs -c "SHOW timezone"` returns `America/Sao_Paulo`, and `created_at` is shown with offset `-03`.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies. T004 needs T002 (the `go` service).
- **Foundational (Phase 2)**: depends on Setup and blocks both stories.
- **US1 (Phase 3)**: depends on Foundational.
- **US2 (Phase 4)**: depends on Foundational. It extends files created in US1 (`service.go`, `repository.go`, `prompt.go`, `main.go`), so run it after US1.
- **Compile boundary**: at the end of US1, `*postgres.Repository` implements the full `user.Repository` port (T018), and `rootCreator` declares only `CreateRoot` (T019). `CheckEmailAvailable` joins the interface in T026, after T024 adds it to `*user.Service`. Every checkpoint therefore builds.
- **Polish (Phase 5)**: depends on US1 and US2.

### User Story Dependencies

- **US1 (P1)**: independent. This is the MVP.
- **US2 (P2)**: builds on US1's files, but its behavior is testable on its own through its own tests and quickstart steps 4 and 5.

### Within Each Story

- Write the tests first and make sure they fail.
- Domain (`internal/user`) before infrastructure (`internal/user/postgres`) before the CLI (`cmd/create-root`).
- `prompt.go` before `main.go`.

### Parallel Opportunities

- Setup: T003, T005 and T006 can run together after T001.
- Foundational: T007, T008 and T011 can run together (T011 also needs T004 and T005).
- US1 tests: T012, T013, T014 and T015 can run together. Then T016 and T018 can run in parallel.
- US2 tests: T021, T022 and T023 can run together. Then T024 and T025 can run in parallel.
- Polish: T029 can run alongside T028.

---

## Parallel Example: User Story 1

```bash
# Tests first (different files):
Task: "Tests for hashPassword in internal/user/password_test.go"                 # T012
Task: "Tests for Service.CreateRoot in internal/user/service_test.go"            # T013
Task: "Integration tests for Create in internal/user/postgres/repository_test.go" # T014
Task: "Happy-path prompt test in cmd/create-root/prompt_test.go"                 # T015

# Then independent implementations:
Task: "Implement hashPassword in internal/user/password.go"                       # T016
Task: "Implement Repository.Create and EmailExists in internal/user/postgres/repository.go" # T018
```

## Parallel Example: User Story 2

```bash
Task: "Service tests for CheckEmailAvailable in internal/user/service_test.go"   # T021
Task: "Duplicate/concurrency tests in internal/user/postgres/repository_test.go" # T022
Task: "Re-prompt and failure tests in cmd/create-root/prompt_test.go"            # T023

Task: "Implement CheckEmailAvailable in internal/user/service.go"                 # T024
Task: "Implement the 23505 mapping in internal/user/postgres/repository.go"       # T025
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1: Setup (Docker, module, migration).
2. Phase 2: Foundational (types, validators, repository port).
3. Phase 3: US1. A ROOT user can be created with valid data.
4. **Stop and validate**: run [quickstart.md](quickstart.md) step 3.

### Incremental Delivery

1. Setup + Foundational give a running database and tested validators.
2. US1 lets you create a ROOT user end to end (MVP).
3. US2 adds re-prompting, duplicate and concurrency safety, and clean failure and interrupt handling.
4. Polish runs the quality gates and the full quickstart.

---

## Notes

- Commit messages follow `type(scope): description (FR-00X)`, for example `feat(user): validate root user e-mail (FR-004, FR-005)`. Setup and Docker commits reference the task ID, for example `build(docker): add compose environment (T002)`.
- All operator-facing text must match [contracts/create-root-cli.md](contracts/create-root-cli.md) exactly (pt-BR, FR-012). Identifiers are in English (Constitution VI).
- Never print or log the password, its confirmation or its hash.

---

## Phase 6: Convergence

**Note**: These tasks supersede the layout and Docker parts of T002–T006, T018, T020, T028 and T030 (plan revision 2026-11-24: code in `backend/`, two images, `go run`). Those tasks stay checked as the history of the first implementation.

- [X] T031 Move the Go code into `backend/`: `cmd/`, `internal/`, `migrations/`, `go.mod` and `go.sum` go to `backend/cmd/`, `backend/internal/`, `backend/migrations/`, `backend/go.mod` and `backend/go.sum`, keeping module path `youxs` so no import changes; do not create `backend/cmd/api/` or `backend/internal/user/http/` (reserved only) per plan: Project Structure, research R13 (missing)
- [X] T032 Remove `Dockerfile`, `.dockerignore` and the `create-root` service from `compose.yaml`, leaving only the `db` and `go` services per plan: "Removed by this revision", research R1 (contradicts)
- [X] T033 Update the `go` service in `compose.yaml`: mount `./backend:/src/backend`, set `working_dir: /src/backend`, keep the `go-mod-cache` volume, `TZ=America/Sao_Paulo` and `TEST_DATABASE_URL`, and add `DATABASE_URL=postgres://youxs:youxs@db:5432/youxs?sslmode=disable`, `stdin_open: true` and `tty: true`. Depends on T031 per plan: Target Platform, research R9, R13 (partial)
- [X] T034 Change the `db` service in `compose.yaml` to mount `./backend/migrations:/migrations:ro` (keep `./docker/postgres/initdb.sh` at the root path), then recreate the volume with `docker compose down -v && docker compose up -d db` and confirm both `youxs` and `youxs_test` have table `users`. `down -v` deletes all local data, including any ROOT users; development environment only. Depends on T031 per research R10, R13 (partial)
- [X] T035 Add `_ "time/tzdata"` to the imports of `backend/cmd/create-root/main.go` with a comment citing Constitution VIII, so `TZ=America/Sao_Paulo` resolves without the removed `-tags timetzdata` build flag. Depends on T031 per research R1, Constitution VIII (missing)
- [X] T036 Re-run the gates and every step of `specs/001-root-user-registration/quickstart.md` (1–7) with the new layout: `docker compose run --rm go gofmt -l .`, `go vet ./...` and `go test ./...` through the `go` service; the command through `docker compose run --rm go go run ./cmd/create-root`; confirm the R13 behavior under `go run` (prompts and hidden input unchanged, Ctrl+C at `Senha:` gives the cancel message and exit 1, the extra `exit status 1` line appears only on failures) and the security checks of T029 against `backend/`. Record any deviation from R13 or the contract. Depends on T031–T035 and T037 (run T037 first) per quickstart, research R13 (partial)
- [X] T037 Make the race-detector gate of T028 reproducible in the `go` service: run it as `docker compose run --rm -e CGO_ENABLED=1 go sh -c 'apk add -q build-base && go test -race ./...'` and document that exact command in quickstart step 2 (no new image, per research R1). Depends on T033 per T028 (partial)
- [X] T038 Remove the `/create-root` entry from `.gitignore`, since no binary is built in the repository anymore per plan: Project Structure (unrequested)
