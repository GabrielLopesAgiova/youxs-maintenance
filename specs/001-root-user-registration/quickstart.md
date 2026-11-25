# Quickstart: validating ROOT User Registration

**Feature**: `001-root-user-registration` | Contracts: [create-root-cli.md](contracts/create-root-cli.md) | Data model: [data-model.md](data-model.md)

## Prerequisites

- Docker with Compose v2. Go does **not** need to be installed on the host.
- Run all commands from the repository root. The Go code lives in `backend/`; the `go` service already works in that folder, so paths such as `./cmd/create-root` are relative to `backend/` (research R13).
- There are only two images: `postgres:18-alpine` (service `db`) and `golang:1.26-alpine` (service `go`). Nothing is built.

## 1. Start the environment

```bash
docker compose up -d db          # PostgreSQL 18; on first start, initdb.sh creates youxs and youxs_test and applies backend/migrations/
docker compose ps                # db must be "healthy"
```

## 2. Run the tests

```bash
docker compose run --rm go go vet ./...
docker compose run --rm go go test ./...          # unit tests and integration tests (TEST_DATABASE_URL is set in the go service)
```

Expected result: every package reports `ok`. The integration tests in `internal/user/postgres` skip with a message if `TEST_DATABASE_URL` is not set.

Race detector (quality gate T028). It needs cgo and a C compiler, which the Alpine image does not ship, so the command enables cgo and installs gcc in the throwaway container (no extra image, research R1):

```bash
docker compose run --rm -e CGO_ENABLED=1 go sh -c 'apk add -q build-base && go test -race ./...'
```

## 3. Create a ROOT user (User Story 1)

```bash
docker compose run --rm go go run ./cmd/create-root
```

Enter `Administrador`, `admin@youxs.com`, then `senhaSegura1` twice. The first run takes a few seconds longer while Go compiles; later runs use the build cache.

Expected result: the password is not echoed, the command prints `Usuário ROOT criado com sucesso.`, and `echo $?` prints `0`.

Inspect the stored row (SC-003):

```bash
docker compose exec db psql -U youxs -d youxs -c "SELECT id, name, email, role, left(password_hash, 30) AS hash_prefix, created_at FROM users;"
```

Expected result: one row with role `ROOT`, e-mail `admin@youxs.com`, a hash prefix of `$argon2id$v=19$m=65536,t=3,p=4$`, and `created_at` shown with offset `-03` (Constitution VIII). The password text does not appear anywhere in the row.

## 4. Validation feedback (User Story 2)

Run `docker compose run --rm go go run ./cmd/create-root` and enter each of these. Each input must print the message shown and ask for the same field again.

| Input | Expected message |
|-------|------------------|
| Name with 81 characters | `O nome deve ter no máximo 80 caracteres.` |
| Name made only of spaces | `O nome é obrigatório.` |
| E-mail `admin@` | `O formato do e-mail é inválido.` |
| E-mail with 101 characters | `O e-mail deve ter no máximo 100 caracteres.` |
| Password `1234567` | `A senha deve ter no mínimo 8 caracteres.` |
| Password with 21 characters | `A senha deve ter no máximo 20 caracteres.` |
| Confirmation different from the password | `As senhas não conferem.` (asks for the password again) |

Values exactly at each limit (80, 100, 8 and 20 characters, and a name such as `João`) must be accepted.

**Duplicate e-mail**: run the command again and enter `ADMIN@youxs.com`. Expected result: `O e-mail informado já está cadastrado. Nenhum usuário foi criado.`, then the Go tool's `exit status 1` line, and exit status `1`. The count from `SELECT count(*) FROM users` must not change.

## 5. Failure paths

On every failure below, the command's message is followed by the Go tool's `exit status 1` line ([contract](contracts/create-root-cli.md#invocation)).

```bash
# Interrupt: press Ctrl+C at the "Senha:" prompt
docker compose run --rm go go run ./cmd/create-root              # expect "Operação cancelada..." and exit 1; the terminal still echoes input

# Storage unavailable
docker compose stop db
docker compose run --rm --no-deps go go run ./cmd/create-root    # expect "...o banco de dados não está acessível." and exit 1, with no prompts
docker compose start db

# Non-interactive input is refused
echo "x" | docker compose run --rm -T go go run ./cmd/create-root   # expect "...terminal interativo." and exit 1

# Arguments are refused
docker compose run --rm go go run ./cmd/create-root extra        # expect "Este comando não aceita argumentos." and exit 1
```

## 6. Concurrency (SC-004)

This is covered by the integration test `TestCreateConcurrentSameEmail` in `internal/user/postgres`, which runs as part of step 2. Expected result: exactly one call succeeds and the other returns `ErrEmailAlreadyRegistered`.

## 7. Layout check (revision 2026-11-24)

```bash
ls backend                       # cmd  go.mod  go.sum  internal  migrations
ls Dockerfile .dockerignore      # both must be missing
docker compose config --services # db and go only
```

## Reset

```bash
docker compose down -v    # removes the database volume; the next "up" applies the migrations again
```
