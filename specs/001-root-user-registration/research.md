# Research: ROOT User Registration

**Feature**: `001-root-user-registration` | **Date**: 2026-11-24 | **Plan**: [plan.md](plan.md)

Each section resolves one open point from the plan's Technical Context.

## R1. Go version and toolchain

- **Decision**: Go 1.26, run in Docker. One image, `golang:1.26-alpine` (Compose service `go`), runs the tests, the quality gates and the command itself (`go run ./cmd/create-root`, R13). There is no project-specific image and no `Dockerfile`. The command embeds the time zone database with `import _ "time/tzdata"` in `cmd/create-root/main.go`, so `TZ=America/Sao_Paulo` works whatever tz files the image has (Constitution VIII). Nothing needs to be installed on the host except Docker.
- **Rationale**: This is the current stable release. Go is not installed on the development host, and the user asked for a Docker environment with only two images: one for PostgreSQL and one for Go. Using one Go image for everything keeps every developer on the same toolchain. Importing `time/tzdata` replaces the earlier `-tags timetzdata` build flag, which only applied to the removed image build.
- **Alternatives considered**:
  - Installing Go on the host: rejected because it adds a manual setup step and versions drift between machines.
  - A multi-stage `Dockerfile` with a minimal runtime image (the earlier decision): rejected by the user in favor of two images. It can come back when a deployable artifact is needed; nothing in the code depends on it.

## R2. PostgreSQL driver

- **Decision**: `database/sql` from the standard library with the `github.com/jackc/pgx/v5/stdlib` driver.
- **Rationale**: `database/sql` covers everything this feature needs (`PingContext`, `BeginTx`, `ExecContext`, `QueryRowContext`). pgx is the maintained PostgreSQL driver for Go. It exposes typed server errors (`*pgconn.PgError`), so the repository can recognize a unique violation (SQLSTATE `23505`) without parsing error text. The pgx import stays inside the infrastructure package, as Principle III requires.
- **Alternatives considered**:
  - `github.com/lib/pq`: in maintenance mode, and its own docs recommend pgx.
  - Native `pgxpool` API: adds API surface that this feature does not need. `database/sql` is enough.
  - An ORM such as GORM or ent: an unnecessary dependency, and the constitution no longer requires one.

## R3. Password hashing algorithm

- **Decision**: **Argon2id** from `golang.org/x/crypto/argon2`, with the RFC 9106 parameters: memory 64 MiB, iterations 3, parallelism 4, 16-byte random salt from `crypto/rand`, and a 32-byte key. The stored value is a self-describing PHC string: `$argon2id$v=19$m=65536,t=3,p=4$<salt-b64>$<hash-b64>`.
- **Rationale**: OWASP's Password Storage Cheat Sheet and RFC 9106 both list Argon2id as the first choice. It is salted, one-way and memory-hard (FR-008, SC-003). The PHC string stores the parameters next to the hash, so a future login feature can verify old hashes after the parameters change. `golang.org/x/crypto` is maintained by the Go team. The CLI already needs its sibling `golang.org/x/term` (R5), so this adds no new trust boundary.
- **Alternatives considered**:
  - `crypto/pbkdf2` (in the standard library since Go 1.24): acceptable to OWASP at 600k iterations of HMAC-SHA256, but it is not memory-hard, so it is weaker against GPU attacks. It is the fallback if a standard-library-only rule ever becomes mandatory.
  - bcrypt (`x/crypto/bcrypt`): secure, and the 20-character password limit stays well below bcrypt's 72-byte cap. It was rejected only because Argon2id is the current first recommendation and it lives in the same module.
  - Plain SHA-256 or another fast hash, with or without salt: rejected as unsuitable for password storage.

## R4. Validation rules (FR-003 to FR-006)

- **Decision**:
  - Name: `strings.TrimSpace`, then check it is not empty, then `utf8.RuneCountInString` ≤ 80.
  - E-mail: `strings.TrimSpace`, then check it is not empty, then rune count ≤ 100, then a format check, then `strings.ToLower`. The format check is `net/mail.ParseAddress`, and the parsed `Address` must equal the input (this rejects display names and angle brackets). It also requires no whitespace, exactly one `@`, a non-empty local part, and a domain that contains at least one `.` and does not start or end with `.`.
  - Password: not trimmed. Rune count must be between 8 and 20. There are no composition rules.
  - Fields are checked in order. The first failure for a field returns a single `ValidationError{Field, Message}` with a Portuguese message.
- **Rationale**: Everything uses the standard library (`strings`, `unicode/utf8`, `net/mail`). Counting runes satisfies "characters, not bytes" ("João" counts as 4). `net/mail` alone is too permissive for FR-005 (it accepts `a@b` and `Nome <a@b.c>`), so the extra checks enforce the rule exactly as the spec states it.
- **Alternatives considered**: A regular expression is harder to read and easy to get subtly wrong. A third-party validator library is an unnecessary dependency.

## R5. Hidden password input and interactive-only mode

- **Decision**: Use `golang.org/x/term`. At startup, `term.IsTerminal(int(os.Stdin.Fd()))` must be true; otherwise the command exits with status 1 and the message "Este comando deve ser executado em um terminal interativo." Passwords are read with `term.ReadPassword`. Name and e-mail are read line by line with `bufio.Reader` from the same stdin.
- **Rationale**: The standard library cannot turn off terminal echo portably without platform-specific `syscall` code. `x/term` is the Go team's supported way to do it (FR-013). Refusing to run without a TTY enforces the spec's decision that there is no non-interactive mode (FR-002) and keeps the password out of shell history and process listings.
- **Alternatives considered**: Raw `syscall`/`ioctl` code is platform-specific and fragile. Taking values from flags or environment variables is explicitly forbidden by FR-002.

## R6. Interruption (Ctrl+C)

- **Decision**: Wrap `main` in `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`. Save the terminal state with `term.GetState` before any prompt. When the signal arrives, restore the state with `term.Restore` (so echo comes back if the signal arrived during a hidden prompt), print a newline and "Operação cancelada. Nenhum usuário foi criado.", and exit with status 1. If the signal arrives while the user is being saved, the command does not exit at once: the insert takes the same context, so it is canceled and rolled back, and the command reports the real outcome (the cancel message and exit 1 if rolled back, or the success message and exit 0 if the row was already committed). See the contract.
- **Rationale**: This covers the edge case "interrupt at any prompt → no user created, failure status" and never leaves the operator's terminal without echo.
- **Alternatives considered**: Leaving the default SIGINT behavior could leave echo disabled if the signal arrives during `ReadPassword`.

## R7. E-mail uniqueness and concurrency (FR-007, SC-004)

- **Decision**:
  1. Always store e-mails in lowercase. The database enforces `UNIQUE (email)` plus `CHECK (email = lower(email))`.
  2. The insert runs inside an explicit transaction (`BeginTx`, `INSERT`, `Commit`). A `*pgconn.PgError` with code `23505` on the e-mail constraint is mapped to `user.ErrEmailAlreadyRegistered`.
  3. To give feedback early, the CLI calls `Service.CheckEmailAvailable` right after a valid e-mail is entered. If the e-mail is already taken, the command reports it and ends with failure (FR-010) before asking for the password. This check is only a convenience. The constraint in step 2 is the guarantee.
- **Rationale**: The unique constraint is the only reliable guard when two executions race. Only one `INSERT` can commit, and the other gets `23505`. That satisfies the concurrency edge case and SC-004. The CHECK makes case-insensitive comparison a property of the schema, not only of the application.
- **Alternatives considered**:
  - Relying on the pre-check alone: subject to race conditions. Rejected.
  - A `citext` column: requires a database extension. Lowercasing plus a CHECK gives the same result with plain `TEXT`.
  - A unique index on `lower(email)`: equivalent, but redundant once storage is always lowercase.

## R8. Transaction scope

- **Decision**: The repository's `Create` method opens the transaction, inserts the user row, and commits. Any error or context cancellation triggers `Rollback` through `defer`. The application service does not see transactions.
- **Rationale**: This meets the requirement that the user is created inside a transaction and that no partial record remains, without leaking a transaction type into the domain (Principle III). Today the operation is a single row. If more rows are added later (for example audit records), they go inside the same transaction in the repository.
- **Alternatives considered**: A unit-of-work abstraction passed through the service was rejected as an unnecessary abstraction for one statement.

## R9. Configuration through environment variables

- **Decision**: One required variable, `DATABASE_URL` (a PostgreSQL URL such as `postgres://youxs:youxs@db:5432/youxs?sslmode=disable`). If it is missing, the command exits with status 1 and the message "Variável de ambiente DATABASE_URL não definida." Before the first prompt, the command calls `PingContext` with a 5-second timeout. Each database operation uses a 10-second timeout. In Docker, the `go` service sets `DATABASE_URL=postgres://youxs:youxs@db:5432/youxs?sslmode=disable` for the command and `TEST_DATABASE_URL=postgres://youxs:youxs@db:5432/youxs_test?sslmode=disable` for the integration tests, so one service serves both purposes and the tests never touch the application database.
- **Rationale**: A single URL is the usual twelve-factor convention, and Docker Compose and the tests can set it easily. Pinging first means the operator learns that storage is unreachable before typing any data (edge case "data store unavailable").
- **Alternatives considered**: Separate `PG*` variables are also supported by pgx, but they are less explicit and harder to document as one contract. Configuration files were not requested and add a parsing step.

## R10. Schema migration

- **Decision**: Plain SQL files in `backend/migrations/` (starting with `0001_create_users.sql`), with no migration library. Compose mounts that folder read-only at `/migrations` in the `db` service. In Docker, `docker/postgres/initdb.sh` runs on the first start of the database volume. It creates the `youxs_test` database and applies every `/migrations/*.sql` file, in order, to both `youxs` and `youxs_test`. Databases outside Docker apply the files with `psql -f`.
- **Rationale**: One table does not justify a migration framework ("no unnecessary dependencies"). The spec assumes that the data store already exists, so the command does not create schema at runtime.
- **Alternatives considered**: golang-migrate or goose would add a dependency and a runtime step. Running migrations from inside the command would be out of scope, because the command only creates the ROOT user.

## R11. Authorization: who can create a ROOT user

- **Decision**: Access is enforced at the infrastructure level, as the spec's assumptions state.
  1. There is no HTTP endpoint. The capability exists only in the `create-root` command (`backend/cmd/create-root`).
  2. Running the command requires shell access to the host or container and a valid `DATABASE_URL` with INSERT permission on `users`.
  3. The command never reads credentials from arguments.
- **Rationale**: These rules match the restriction "Não permitir a criação de ROOT por usuários não autorizados" (do not let unauthorized users create a ROOT user) without inventing authentication, which the spec puts out of scope.
- **Alternatives considered**: Requiring an existing ROOT user to log in first was rejected. It would make the first ROOT user impossible to create, and authentication is out of scope.

## R12. Testing strategy (Principle IV)

- **Decision**:
  - **Unit tests** (`go test ./...`, no database): the validators (limits, trimming, rune counting, every FR-005 format case), `Service.CreateRoot` and `Service.CheckEmailAvailable` with an in-memory fake repository (duplicate e-mail, role always ROOT, the stored hash never equals the password and parses as Argon2id), the password hasher (distinct salts produce distinct hashes and the PHC format is correct), and the CLI prompt loop with fake reader, writer and password reader (re-prompt on invalid input, confirmation mismatch, success and failure messages, exit codes).
  - **Integration tests** (`internal/user/postgres`, run when `TEST_DATABASE_URL` is set): insert and read back a user, duplicate e-mail with different case returns `ErrEmailAlreadyRegistered`, two concurrent `Create` calls with the same e-mail produce exactly one success, and the CHECK constraint rejects mixed-case e-mail.
  - Tests use only the standard `testing` package.
- **Rationale**: Principle IV requires unit tests for every business rule. Its integration-test rule targets endpoints, and this feature has none. The repository integration tests are still the only way to verify the concurrency and transaction guarantees.
- **Alternatives considered**: testify or gomock would be unnecessary dependencies. testcontainers-go was rejected because the Compose database is already available.

## R13. Repository layout and running the command through `go run`

- **Decision**:
  1. All Go code lives in `backend/`, which is the Go module root (`backend/go.mod`, module path `youxs`, so import paths such as `youxs/internal/user` do not change). The repository root keeps `compose.yaml`, `docker/`, `.gitignore` and the spec-kit folders.
  2. The plan reserves `backend/cmd/api/` (future HTTP server) and `backend/internal/user/http/` (future handlers of the user module). They are not created by this feature.
  3. Compose has two services. `db` uses `postgres:18-alpine` and mounts `./backend/migrations` at `/migrations`. `go` uses `golang:1.26-alpine`, mounts `./backend` at `/src/backend` with `working_dir: /src/backend`, keeps the named module-cache volume (`/go/pkg/mod`) and a named build-cache volume (`/root/.cache/go-build`), sets `TZ`, `DATABASE_URL` and `TEST_DATABASE_URL`, and has `stdin_open: true` and `tty: true` so the interactive command gets a terminal.
  4. The command runs with `docker compose run --rm go go run ./cmd/create-root`. Tests run with `docker compose run --rm go go test ./...`.
- **Behavior under `go run`** (checked against the contract):
  - `go run` compiles to a temporary binary and runs it as a child process. Stdin, stdout, stderr and the TTY pass through unchanged, so prompts, hidden input and the TTY check behave as before.
  - When the program exits with a non-zero status, `go run` exits with status 1 and prints one extra line, `exit status 1`, on stderr. The contract's exit codes are only 0 and 1, so the exit status is preserved. The extra line is printed by the Go tool, after the command's own message, and the contract notes it.
  - Ctrl+C in the terminal sends SIGINT to the whole foreground process group. `go run` ignores the signal and waits for the child, and the child handles it as in R6, so the interrupt behavior is unchanged.
  - A SIGTERM sent to the container (`docker stop`) reaches only PID 1, which is the `go` tool, not the command. That path is outside normal operator use (the operator presses Ctrl+C); the command still creates no partial row, because an interrupted connection rolls back any open transaction.
- **Rationale**: The user asked for the code to live in a `backend/` folder and for only two images, one for PostgreSQL and one for Go. Keeping `backend/` as the module root leaves the standard Go layout intact inside it. Mounting only `./backend` into the `go` service keeps `.git`, `specs/` and the spec-kit folders out of the toolchain container.
- **Alternatives considered**:
  - Keeping `go.mod` at the repository root with code under `backend/`: import paths would become `youxs/backend/internal/...` for no benefit, and the root would still mix Go and non-Go files.
  - Moving `compose.yaml` and `docker/` into `backend/`: the orchestration could then not serve a future frontend at the root. Rejected by the user.
  - **Build cache (found during T036)**: without a volume for `/root/.cache/go-build`, every `docker compose run --rm` started with an empty build cache and `go run` recompiled all dependencies, so input typed early arrived before the prompts and a Ctrl+C during compilation ended the Go tool with status 2 instead of reaching the command. With the volume, a run starts in about 2 seconds and the contract behavior holds.
  - Building the binary inside the `go` service (`go build` then running it) to avoid the extra `exit status 1` line: it needs a shell wrapper in every invocation. The extra line is harmless and documented instead.
  - Creating empty `cmd/api/` and `internal/user/http/` folders now: git does not track empty folders and Go would see no package. The locations are documented in the plan instead.
