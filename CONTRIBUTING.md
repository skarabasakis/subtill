# Subtill development guide

Repository layout and agent conventions: [AGENTS.md](AGENTS.md).

## Prerequisites

### Runtimes

- **Go 1.25+**
- **Node.js 24.x**

Both can be installed with [asdf](https://asdf-vm.com/):

```bash
brew install asdf
asdf install
```

### Commands

- **wails3** (`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.30 && wails3 setup`)
- **golangci-lint** (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`)
- **migrate** (`go install github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.2`)

## First-time setup

```bash
wails3 task install:frontend:deps
wails3 task install:e2e:deps
cd e2e && npx playwright install   # once per machine
```

## Workflows

Run all workflows from the **repo root** with **`wails3 task <name>`** (not bare `task`). VS Code tasks in [`.vscode/tasks.json`](.vscode/tasks.json) call the same commands.

Discover everything available:

```bash
wails3 task -list
```

### Development

- **`dev`** — daily desktop development (Wails dev server + hot reload)
- **`lint`** — golangci-lint + frontend/e2e oxlint
- **`fmt`** — golangci-lint fmt + frontend/e2e oxfmt
- **`openapi`** — regenerate Swagger under `internal/http/docs/` (commit results)
- **`migrate:create`** — new SQL migration pair under `internal/core/migrate/migrations/`

#### Examples

```bash
# Desktop dev
wails3 task dev

# Quality (typical pre-push)
wails3 task lint

# New migration (name becomes part of the filename)
wails3 task migrate:create -- add_subscriptions

# Regenerate OpenAPI after handler annotations change
wails3 task openapi
```

#### `migrate:create`

Creates `.up.sql` / `.down.sql` via the **`migrate`** CLI (same major/minor as `github.com/golang-migrate/migrate/v4` in `go.mod`):

```bash
wails3 task migrate:create -- <migration_name>
```

Runtime migrations are applied by the app from embedded SQL, not by running `migrate up` in CI.

### Testing

E2E setup is included in [first-time setup](#first-time-setup) (`install:e2e:deps` and Playwright browsers).

- **`install:e2e:deps`** — after clone or when `e2e/package.json` changes
- **`test`** — Go unit tests (`go test ./...`) + frontend Vitest
- **`test:integration`** — Go tests with `-tags integration` (`internal/http`, `internal/mcp`)
- **`e2e`** — Playwright smoke (builds dev server binary, runs tests in `e2e/`)

Integration tests live in `*_integration_test.go` with `//go:build integration`; they are not run by plain `go test ./...` or by **`test`** alone. In VS Code, leave **`go.testTags` unset** so the editor matches **`wails3 task test`** (not integration).

```bash
# Unit tests (Go + frontend)
wails3 task test

# HTTP and MCP adapters
wails3 task test:integration

# Typical pre-push
wails3 task lint
wails3 task test
wails3 task test:integration

# Desktop E2E (Wails server mode)
wails3 task e2e
```

**`e2e`** depends on **`build:server`** with `DEV=true` (see [Building & Packaging](#building--packaging)), then starts `./bin/subtill-server --config-path e2e/.tmp-config` via `e2e/playwright.config.ts`.

### Building & Packaging

#### Desktop

- **`build`** — production desktop binary → `bin/subtill` (or `.exe`)
- **`run`** — run the desktop app from the last `build` (platform-specific)
- **`package`** — platform installer/archive (nfpm, DMG, MSIX, etc.) via `{{.GOOS}}:package`
- **`install:msix:tools`** — Windows only: tools for MSIX packaging

Cross-compile desktop builds with `GOOS` / `wails3 build GOOS=linux` as supported by Wails.

```bash
wails3 task build
wails3 task run
wails3 task package
```

#### Server mode & Docker

Headless UI (Playwright, local HTTP testing). Binary: **`bin/subtill-server`**. Also used by Playwright **`e2e`**; see [Testing](#testing).

- **`build:server`** — server-mode binary (`-tags server`); dev builds: `wails3 task build:server DEV=true`
- **`run:server`** — build (dev) server binary and run it
- **`build:docker`** — Docker image with production server binary
- **`run:docker`** — build image and run container (port 8080 by default)
- **`setup:docker`** — one-time Docker image for cross-compilation (~800MB download)

#### Build outputs

- **Desktop** — `bin/subtill` (macOS/Linux), `bin/subtill.exe` (Windows)
- **Server mode** — `bin/subtill-server`
- **Frontend bundle** — `frontend/dist` (embedded; gitignored)

`bin/` is gitignored—paths are stable for local smoke tests:

```bash
./bin/subtill --api --port 8000 --token test
```

#### Under the hood (`common:*`)

Wails **`build/`** Taskfile powers frontend install, bindings, and icons. Root tasks delegate to these; you rarely call them directly:

- `common:install:frontend:deps` — npm/bun/pnpm/yarn install in `frontend/`
- `common:build:frontend` — bindings generation + Vite build
- `common:generate:bindings` — `wails3 generate bindings`
- `common:generate:icons` — app icons from `build/appicon.png`
- `common:dev:frontend` — Vite dev server (used inside `wails3 dev`)
- `common:update:build-assets` — refresh Wails build templates (use with care)

Use **`wails3 task -list`** for the full platform matrix (`darwin:build`, `linux:package`, …).

## Go module notes

- **`go.mod` `ignore`**: `./frontend/node_modules`, `./e2e` so `./...` tooling skips npm trees.

## Product direction

Feature scope and UX: [docs/prd/20261013-project-brief.md](docs/prd/20261013-project-brief.md).
