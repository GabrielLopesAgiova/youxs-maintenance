<!--
Sync Impact Report
==================
Version change: 2.0.0 → 2.1.0
Bump rationale: MINOR. A new principle (VIII) was added. No existing principle was removed or
redefined.

Modified principles: none

Added sections:
  - VIII. Brasília Time Zone (new principle)
  - Development Workflow and Quality Gates: code review checklist now includes time zone (VIII)

Removed sections: none

Notes:
  - The request said "UTC+3"; Brasília time is UTC−03:00. The principle follows the stated
    intent (Brasília time) and uses the IANA zone America/Sao_Paulo.

Follow-up TODOs:
  - specs/001-root-user-registration: plan/data-model use TIMESTAMPTZ DEFAULT now() for
    created_at; the plan should state that the DB/app time zone is America/Sao_Paulo
    (re-run /speckit-plan or adjust the plan) before implementation.
-->

# YOUXS Maintenance Constitution

## Core Principles

### I. Endpoints Delegate to Application Services

- An endpoint (controller, route or HTTP handler) MUST NOT access the database directly:
  no repository, ORM context/session, connection or query inside it.
- All data access originating at the HTTP edge MUST go through an application service.
- The endpoint is limited to: receiving the request, mapping it to the service input,
  calling the service and translating the result into an HTTP response.

**Rationale**: keeps the HTTP layer thin and replaceable, and gives every use case a single
entry point that can be tested independently of the transport.

### II. Business Rules Stay Out of the HTTP Layer

- Business rules MUST live in the domain or in application services, never in endpoints,
  HTTP handlers, middlewares or filters.
- Request shape validation (types, required fields) MAY happen at the edge; any decision
  that depends on state or business policy MUST live in the service or the domain.

**Rationale**: rules scattered across the HTTP layer are not reusable, get duplicated
between endpoints and escape the unit tests required by Principle IV.

### III. Isolated Persistence Layer

- All persistence MUST be isolated in an infrastructure layer.
- The application and the domain MUST NOT depend directly on database implementation
  details (drivers, connections, vendor-specific types, schema details or query languages).
- SQL queries MAY be used when justified, provided they remain isolated in the persistence
  layer. The justification MUST be recorded in the plan (complexity tracking section).

**Rationale**: the database technology can evolve or be replaced by changing only the
infrastructure layer, and data access is audited in a single place.

### IV. Mandatory Tests (NON-NEGOTIABLE)

- Every business rule MUST have at least one unit test exercising it in isolation,
  without a database or HTTP.
- Every endpoint MUST have at least one integration test covering the happy path, from the
  HTTP request to the response, going through real or test persistence.
- A task that introduces a business rule or an endpoint MUST NOT be considered done until
  the corresponding tests pass.

**Rationale**: unit tests protect the rules; integration tests prove that the wiring
between layers (Principles I–III) actually works.

### V. Standardized Validation Errors

- Every validation error MUST return HTTP **400 Bad Request**, without exception.
- The response body MUST carry the error details: at minimum, for each violation, the field
  (when applicable) and a message explaining the problem.
- The error body format MUST be the same across all endpoints of the application.

**Rationale**: clients can handle errors predictably and show users exactly what needs to
be fixed.

### VI. Code Language and User-Facing Messages

- Names of packages, types, functions, variables and files MUST follow the naming
  conventions of the language and framework in use and MUST be written in English.
- Messages intended for users (including error messages returned by the API) MUST be
  written in Portuguese.

**Rationale**: code aligned with the vocabulary and idioms of the technical ecosystem, and
communication with users in the product's language.

### VII. Modular Organization by Business Domain

- The application MUST be organized into modules that represent well-defined
  responsibilities and business domains.
- Each module MUST have a clear, single purpose; modules that exist only for technical or
  organizational grouping MUST be justified in the plan.

**Rationale**: domain-aligned modules keep related rules together, reduce coupling and make
ownership and change impact easy to reason about.

### VIII. Brasília Time Zone

- Every date and time the system records, displays, compares or interprets (including
  timestamps, schedules, deadlines, logs and reports) MUST use Brasília time: the IANA zone
  `America/Sao_Paulo` (currently UTC−03:00).
- The zone MUST be configured by its IANA name (`America/Sao_Paulo`), never as a hard-coded
  fixed offset, in the application runtime (e.g. `TZ`), the database session/server time zone
  and container images.
- Stored timestamps MUST carry their offset (e.g. PostgreSQL `TIMESTAMPTZ`), so each instant
  remains unambiguous.
- Dates and times shown to users MUST be presented in Brasília time.

**Rationale**: the product's users and operations are in Brazil; a single, explicit time zone
avoids off-by-hours bugs between application, database and reports.

## Commit Conventions

- Every commit message MUST follow the format `type(scope): description`.
- `type` MUST be one of the Conventional Commits types (`feat`, `fix`, `refactor`, `test`,
  `docs`, `chore`, `build`, `ci`, `perf`, `style`).
- The message MUST contain the identifier of the requirement it addresses, as defined in
  the feature specification (e.g. `FR-003`).
- Example: `feat(work-order): allow closing a work order (FR-003)`.
- Commits that do not address a functional requirement (e.g. infrastructure adjustments)
  MUST reference the identifier of the task or specification that motivated them.

## Development Workflow and Quality Gates

- Every plan (`/speckit-plan`) MUST pass the *Constitution Check* before the design phase
  and again after it.
- Code reviews MUST verify, at minimum: no data access and no business rules in endpoints
  (I, II), no database implementation details or SQL outside the persistence layer (III),
  presence of the required tests (IV), status and format of validation errors (V), naming
  conventions and message language (VI), domain-aligned module boundaries (VII), use of the
  Brasília time zone (VIII) and commit message format.
- A change that violates any principle MUST NOT be merged.

## Governance

- This constitution supersedes any other practice, plan, specification or task list in the
  project.
- **If a plan contradicts any principle, the plan is what changes.** A violation found in a
  plan MUST be resolved by adjusting the plan, never by relaxing the principle. Exceptions
  exist only through a formal amendment to this constitution.
- **Amendments**: MUST be proposed in writing, with rationale and impact on existing
  artifacts, and applied via `/speckit-constitution`.
- **Versioning** (SemVer):
  - MAJOR: backward-incompatible removal or redefinition of a principle or governance rule.
  - MINOR: new principle or section, or materially expanded guidance.
  - PATCH: clarifications, wording and fixes with no semantic effect.
- **Compliance**: every plan, code review and analysis (`/speckit-analyze`) MUST verify
  adherence to these principles; violations are blocking.

**Version**: 2.1.0 | **Ratified**: 2026-11-24 | **Last Amended**: 2026-11-24
