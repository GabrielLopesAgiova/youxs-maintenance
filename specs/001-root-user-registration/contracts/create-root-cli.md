# Contract: `create-root` CLI command

**Feature**: `001-root-user-registration` | **Requirements**: FR-001, FR-002, FR-010 to FR-013, FR-015

## Invocation

```text
create-root
```

- The command takes **no arguments or flags**. If any argument is given, it prints `Este comando não aceita argumentos.` to stderr and exits with status 1 (FR-002).
- Stdin **must be a TTY**. If it is not, the command prints `Este comando deve ser executado em um terminal interativo.` to stderr and exits with status 1.
- Source: `backend/cmd/create-root`.
- In Docker: `docker compose run --rm go go run ./cmd/create-root` (the `go` service works in `/src/backend`; research R13).
- Under `go run`, a non-zero exit is reported by the Go tool as exit status 1, followed by one extra stderr line, `exit status 1`, after the command's own message. Exit status 0 adds nothing.
- Under `go run`, Ctrl+C (SIGINT from the terminal) reaches the command and behaves as described below. A SIGTERM sent to the container (`docker stop`) reaches only the Go tool, so the SIGTERM rows below apply only when the compiled command receives the signal directly (research R13). No partial row remains either way.

## Environment

| Variable       | Required | Description |
|----------------|----------|-------------|
| `DATABASE_URL` | yes      | PostgreSQL connection URL, for example `postgres://youxs:youxs@db:5432/youxs?sslmode=disable`. |

## Interaction sequence

All prompts go to stdout. Error messages for a field also go to stdout, and the command then asks for that field again. Fatal errors go to stderr.

```text
1. [startup] check args, check TTY, read DATABASE_URL, ping the database (5 s)
2. Nome: <line>                  → invalid: print message, repeat step 2
3. E-mail: <line>                → invalid: print message, repeat step 3
   [check e-mail, 10 s]          → already registered: FATAL (exit 1)
                                 → check fails (database unreachable, timeout): FATAL (exit 1)
4. Senha: <hidden>               → invalid: print message, repeat step 4
5. Confirme a senha: <hidden>    → mismatch: print "As senhas não conferem.", repeat step 4
6. [create in a transaction, 10 s] → success: print the success message, exit 0
                                   → duplicate (race): FATAL (exit 1)
                                   → canceled by Ctrl+C / SIGTERM: FATAL (exit 1)
                                   → other error: FATAL (exit 1)
```

Each database call after startup (the e-mail check and the create) has its own 10-second timeout (research R9). A timeout counts as a failure of that step.

## Messages (pt-BR, FR-012)

| Situation | Stream | Message | Exit |
|-----------|--------|---------|------|
| Success | stdout | `Usuário ROOT criado com sucesso.` | 0 |
| Field validation | stdout | Message from [data-model.md](../data-model.md#validation-rules), then the same prompt again | — |
| Password mismatch | stdout | `As senhas não conferem.` | — |
| E-mail already registered | stderr | `O e-mail informado já está cadastrado. Nenhum usuário foi criado.` | 1 |
| Database unreachable at startup, or the e-mail check fails for any reason other than a duplicate | stderr | `Não foi possível criar o usuário ROOT: o banco de dados não está acessível.` | 1 |
| Failure while saving | stderr | `Não foi possível criar o usuário ROOT: falha ao gravar os dados.` | 1 |
| `DATABASE_URL` missing | stderr | `Variável de ambiente DATABASE_URL não definida.` | 1 |
| Not a TTY | stderr | `Este comando deve ser executado em um terminal interativo.` | 1 |
| Arguments given | stderr | `Este comando não aceita argumentos.` | 1 |
| Ctrl+C / SIGTERM at a prompt or during the e-mail check | stderr | `Operação cancelada. Nenhum usuário foi criado.` | 1 |
| Ctrl+C / SIGTERM while the user is being saved | stderr / stdout | The command waits for the save to finish and reports what happened. If it was rolled back: `Operação cancelada. Nenhum usuário foi criado.` (stderr, exit 1). If it had already committed: the success message (stdout, exit 0). | 1 or 0 |
| End of input (EOF) at a prompt | stderr | `Operação cancelada. Nenhum usuário foi criado.` | 1 |

## Guarantees

- The password and its hash are never printed, logged or included in any error message.
- Typed passwords are not echoed (FR-013). On exit, including through a signal, the terminal's original state is restored.
- On any exit status other than 0, no user row exists from this execution. This also holds when a signal arrives during the save: the command never reports a cancellation for a row that was committed.
- Fatal messages never include the underlying error text (driver, SQL or network details).
- Exit status 0 means exactly one new `users` row exists with `role = 'ROOT'`.
