# Agents

Rules and layout for automated contributors working on subtill.

## Documentation

File names are prefixed with the date of creation in `YYYYMMDD-` format.

- Product Requirements Documents: `docs/prd/` (product feature descriptions, requirements, and user stories). They are not perpetually maintained and can diverge from the codebase.
- Architectural Decision Records: `docs/adr/` (technical design documents, documentation of technical decisions, trade-offs, and rationale). They are kept up to date with the codebase. Avoid duplicating the same information into multiple ADRs.
- Package guidance: an `AGENTS.md` next to the code holds the invariants and rules for changing that package (e.g. `internal/core/database/AGENTS.md`). It is kept up to date with the package.

## Layout

- Business logic: `internal/core` (domain, database, locale, store; migrations under `internal/core/database/migrations/`).
- Adapters: `internal/cli`, `internal/http`, `internal/mcp`, `internal/desktop`.
- Desktop UI: `frontend/` only (Vite → `frontend/dist`, embedded via `frontend/embed.go`).
- E2E: Playwright in `e2e/` (Wails server mode); not the REST `--api` adapter.

## Code Style Preferences

### Implementation

- Avoid comments unless they add value and clarify the code's intention.
- Functions that involve multiple steps or complex logic should be structured hierarchically:
  - the top level function is a composed method that holds the context, owns the control flow and orchestrates the lower-level functions
  - lower-level functions are as agnostic as possible of the caller's context, own the implementation details and are further decomposed until they have a single responsibility.
- Function names:
  - prefer a more descriptive name over a shorter name preceded by a comment indicating what the function does.
  - use intent-based naming: nouns for functions that retrieve state, verbs for functions that mutate state. Avoid mixing state retrieval and mutation in the same function.
  - follow naming conventions:
    - mustX for panicking guards
    - isX or hasX for guards that return a boolean
    - checkX for guards that may return an error
    - validateX for input validation helpers

### Tests

- Test packages through their public API from an external `_test` package. Internal tests (`*_internal_test.go`) are a last resort for logic that can't be reached through the API.
- When a test needs to vary a dependency (migration source, clock, timeout, filesystem, client), make it injectable instead of testing unexported functions
- When testing a scenario requires fixtures, create them in-memory (`fstest.MapFS`) or as files under `testdata/`.
- As much as possible, structure tests in BDD style: `Test<Subject>` only groups `t.Run("<assert> when <precondition>", ...)` subtests, one behavior each, split by blank lines into Given, When, and Then. Keep names to about 12 words, omit articles when the sentence stays clear, and keep related names symmetric.

## Workflows

Use **`wails3 task <name>`** from the repo root ([`Taskfile.yml`](Taskfile.yml)). [`.vscode/tasks.json`](.vscode/tasks.json) wraps these tasks only. Human-oriented task reference: [CONTRIBUTING.md](CONTRIBUTING.md).

- **`install:frontend:deps`** / **`install:e2e:deps`**: npm installs (do not run bare `npm install` in `frontend/` for setup).
- **`ensure:embeds`**: creates `frontend/dist` before Go lint/test (gitignored `dist`).
- **`test`**: unit Go tests + frontend Vitest (no `integration` tag).
- **`test:integration`**: `go test -tags integration` for `internal/http` and `internal/mcp`.
- **`migrate:create`**: `migrate create` (CLI on `PATH`; see CONTRIBUTING) for new SQL migrations.
- **`openapi`**: regenerates Swagger under `internal/http/docs`.

## API

- REST (`--api`): JSON under `/api/*` with bearer auth; Swagger UI at `/swagger` (no bearer).
- Regenerate OpenAPI with `wails3 task openapi`; commit `internal/http/docs/`.

## Frontend

- shadcn owned components in `frontend/src/components/ui/`; app-specific tailwind-based shared components in `frontend/src/components/shared/`.
- TanStack Query default `queryFn` → Wails bindings (`frontend/src/lib/query/default-query-fn.ts`).
- Regenerate bindings after Wails service changes; do not hand-edit `frontend/bindings/`.

## Windows

Production builds use `windowsgui`. For `--cmd` / `--mcp` / `--api` from a shell, `internal/runtime/console.PrepareHeadless()` runs before stdio output.
