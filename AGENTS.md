# Agents

Rules and layout for automated contributors working on subtill.

## Layout

- Business logic: `internal/core` (domain, services, store, migrations under `internal/core/migrate/migrations/`).
- Adapters: `internal/cli`, `internal/http`, `internal/mcp`, `internal/desktop`.
- Desktop UI: `frontend/` only (Vite → `frontend/dist`, embedded via `frontend/embed.go`).
- E2E: Playwright in `e2e/` (Wails server mode); not the REST `--api` adapter.

## Workflows

Use **`wails3 task <name>`** from the repo root ([`Taskfile.yml`](Taskfile.yml)). [`.vscode/tasks.json`](.vscode/tasks.json) wraps these tasks only. Human-oriented task reference: [CONTRIBUTING.md](CONTRIBUTING.md).

- **`install:frontend:deps`** / **`install:e2e:deps`**: npm installs (do not run bare `npm install` in `frontend/` for setup).
- **`ensure:embeds`**: creates `frontend/dist` before Go lint/test (gitignored `dist`).
- **`test`**: unit Go tests + frontend Vitest (no `integration` tag).
- **`test:integration`**: `go test -tags integration` for `internal/http` and `internal/mcp`.
- **`migrate:create`**: `migrate create` (CLI on `PATH`; see CONTRIBUTING) for new SQL migrations.
- **`openapi`**: regenerates Swagger under `internal/http/docs`.

Go **1.25+** with `go.mod` **`ignore`** for `./frontend/node_modules` and `./e2e`. Do not set VS Code `go.testTags` globally.

## API

- REST (`--api`): JSON under `/api/*` with bearer auth; Swagger UI at `/swagger` (no bearer).
- Regenerate OpenAPI with `wails3 task openapi`; commit `internal/http/docs/`.

## Frontend

- shadcn in `frontend/src/components/ui/`; app primitives in `frontend/src/components/primitives/`.
- TanStack Query default `queryFn` → Wails bindings (`frontend/src/lib/query/default-query-fn.ts`).
- Regenerate bindings after Wails service changes; do not hand-edit `frontend/bindings/`.

## Windows

Production builds use `windowsgui`. For `--cmd` / `--mcp` / `--api` from a shell, `internal/runtime/console.PrepareHeadless()` runs before stdio output.
