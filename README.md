# subtill

**subtill** is a local subscription manager: track recurring subscriptions, costs, and reminders in a SQLite database on your machine. Nothing is sent to the cloud unless you choose to use optional network features (for example the REST API).

More background: [product brief](docs/prd/20261013-project-brief.md).

## Platforms

Windows, macOS, and Linux (desktop app).

## Getting the app

Install a release build for your platform when available, or build from source — see [CONTRIBUTING.md](CONTRIBUTING.md).

After building, the desktop app is **`bin/subtill`** (or **`bin/subtill.exe`** on Windows).

## Using subtill

### Desktop (default)

Open the **subtill** application like any other desktop app. Your data lives in a SQLite file you create or open on first launch (see the product brief for the database workflow).

### Command line (`--cmd`)

For scripting and terminal use:

```bash
subtill --cmd <command> [args...]
```

On Windows, run `subtill.exe` from the folder where you installed or built it.

### MCP (`--mcp`)

Run subtill as an [MCP](https://modelcontextprotocol.io/) server for tools that support it:

```bash
subtill --mcp
```

### REST API (`--api`)

Expose a JSON HTTP API (for automation or integrations). All **`/api/*`** routes require a bearer token; **Swagger UI** at **`/swagger`** does not.

```bash
subtill --api --port 8000 --token <your-secret>
```

- API base: `http://127.0.0.1:8000/api/`
- Swagger: `http://127.0.0.1:8000/swagger`

Use only one of `--cmd`, `--mcp`, or `--api` at a time.

### Optional flags

| Flag | Purpose |
|------|---------|
| `--config-path <dir>` | Directory for app configuration and isolated state (used by tests and advanced setups) |

## Server mode (browser UI, no native window)

A separate **server** build (`subtill-server`) serves the same web UI and Wails bindings over HTTP—mainly for development and automated testing, not typical end-user installs. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Contributing

Developers: prerequisites, tasks, tests, and tooling are documented in **[CONTRIBUTING.md](CONTRIBUTING.md)**.
