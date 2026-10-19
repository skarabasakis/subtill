# `subtill`: Project Brief

## Introduction

These days, subscription services are all too common, both online and offline. Charging their users on a recurring basis has become the de facto business model for several types of services such as:
- streaming video, music, gaming and entertainment services
- cloud services, web applications, mobile apps -- even desktop software
- digital content such as online courses, books and audiobooks
- gym membership, car insurance, novelty "subscription boxes" and everything in between

Subscriptions have become unavoidable for users, who often find themselves subscribed to a myriad different services. Each subscription renews at a different frequency (e.g. monthly, quarterly, semiannual, yearly, every 2 years etc) and at a different date. Some subscriptions renew automatically while others have a fixed end date. As the subscriptions pile up, it becomes non-trivial for users to monitor how much they are spending and plan their budget.

## Project Overview

`subtill` is a subscription manager, a program designed to help users track their subscriptions and associated payments.

`subtill` is developed in Go and is designed to be a standalone application that runs locally on the user's machine. Subscription data are stored in a local SQLite database.

`subtill` is cross-platform and can be run on Windows, macOS and Linux.

## Stack

- Core: Go (use version pinned in asdf .tool-versions)
  - Database: SQLite
    - DB access layer: `jmoiron/sqlx` package
    - Schema Migrations: `golang-migrate/migrate` package
  - Decimals: `github.com/govalues/decimal` package
  - Desktop GUI Framework: Wails3
  - Testing: built-in `testing` package
  - Adapters:
    - MCP: `github.com/modelcontextprotocol/go-sdk/mcp`
    - HTTP: built-in `net/http` package with built-in mux router
- Frontend (desktop GUI): React, Tailwind and Shadcn UI
  - Runtime: Node.js (use version pinned in asdf .tool-versions)
  - Language: TypeScript (wails3 default)
  - Package Manager: npm
  - Tooling: Vite (wails3 default)
    - Linter: Oxlint
    - Formatter: Oxfmt
    - Testing: vitest
  - Framework: React 19 (upgraded from wails3 default)
    - Routing: HashRouter from react-router
    - Data Fetching: Tanstack Query with a default query function that maps query keys to Wails bindings
    - Form Handling: Tanstack Form
    - Table Handling: Tanstack Table
    - Charts: shadcn/ui charts with Recharts
  - UI Components
    - Styling: Tailwind CSS
    - Component Library: Shadcn UI
    - Icons: Lucide React

## Data Model

Constraints that are trivially expressible in SQL (`NOT NULL`, `UNIQUE`, `CHECK` on ranges and enums, foreign keys) are part of the schema. Invariants spanning multiple rows (e.g. non-overlapping cycles) are enforced at the application layer.

Decimal values are stored as `TEXT` in SQLite. No decimal arithmetic happens in the database; it is performed in Go using `govalues/decimal`. Decimals are also passed as strings, never as numbers, everywhere they cross a boundary: between Go and JS, in CLI JSON output, and in MCP and REST inputs and outputs; the frontend uses a decimal library (e.g. decimal.js) for any decimal calculations.

A subscription record has the following attributes:

- `id`: the id of the subscription. Unique, uuidv7
- `name`: the name of the subscription. Unique within the database, case-insensitive (`COLLATE NOCASE`). An opaque label (not required to be a domain).
- `group`: A string. Can optionally be set to group subscriptions together.
- `tags`: Array of strings. Can optionally be set to add tags to the subscription. Stored in a separate `subscription_tags` table with columns `id` (uuidv7), `subscription_id` (foreign key to the subscription) and `tag` (string, case-insensitive, `COLLATE NOCASE`). The same tag can't be added twice to a subscription (`subscription_id`, `tag` is unique).
- `notes`: A string. Can optionally be set to add notes to the subscription.
- `icon`: A base64-encoded PNG favicon of the subscription. Best-effort fetch from a favicon api using the first segment of the name before `/` as the domain; lookup fails if the name is not domain-like. Left empty if lookup fails. The application is responsible for retrieving the favicon in PNG format. Optional.
- `url`: A website URL from which the subscription can be managed (cancelled/renewed). Optional.

A subscription has at least one cycle and can have multiple, strictly non-overlapping cycles (both enforced at the application layer), i.e. it can be stopped and started again. Each cycle has its own start and end date. In addition, the price of a subscription can change over time, and this is modeled by creating a new cycle. Thus each subscription cycle has the following attributes:
- `id`: the id of the cycle. Unique within the database, uuidv7
- `subscription_id`: the id of the subscription
- `frequency`: (integer) the monthly frequency of the subscription, e.g 1 = monthly, 3 = quarterly, 6 = semiannual, 12 = yearly, 24 = every 2 years. v1 supports month-based frequencies only.
- `cost`: Required. The cost of the subscription, in the subscription's currency, which can be different from the database's default currency.
  - `currency`: the currency of the subscription cost, e.g. USD, EUR, GBP, etc.
  - `amount`: (decimal) the amount of the subscription cost, e.g. 14.00
- `approximate_exchanged_amount`: (decimal) The approximate cost amount for the cycle, in the database's default currency. Required. Equals amount if cost currency matches the database's default currency. Otherwise, calculated from exchange rates retrieved from online sources at the moment the cycle is added or its amount is edited.
- `starts_on`: the date the subscription was charged and the cycle started, e.g. 2021-01-01
- `ends_on`: the date the cycle ends, exclusive (no charge occurs on it). It can be null if the subscription is ongoing. Fixed-term and cancelled subscriptions are not distinguished: both expire on `ends_on`. `ends_on` need not fall on a charge date; a cycle ending mid-period is equivalent to one ending at the end of that period (e.g. a monthly cycle Jan 1 - Jan 29 is equivalent to Jan 1 - Feb 1). Two cycles are consecutive when the first cycle's `ends_on` equals the second cycle's `starts_on`.

A subscription can be associated with one or more reminders, which are triggered on a specific date and remain active until deleted. All existing reminders are pending; clearing (marking complete) a reminder deletes it. Duplicate reminders (same subscription, action and date) are allowed. A reminder has the following attributes:
- `id`: the id of the reminder. Unique, uuidv7
- `subscription_id`: the id of the subscription
- `action`: (enum) the action to perform when the reminder is triggered, e.g. "cancel", "renew"
- `note`: (string) a note to display when the reminder is triggered
- `date`: the date on which to trigger the reminder, e.g. 2021-01-01
- `notification_sent`: (boolean) whether the OS notification for the reminder has been sent

The user can set database-scoped preferences, such as database display name (label only; the file path is chosen separately), default currency, and preferred time of day to receive notifications (integer hour 0-23, default 7).

When converting to the default currency for payments and totals, a database-scoped preference chooses **live exchange rate** (use the exchange rate for each payment date) or **approximate exchange rate** (use each cycle's stored `approximate_exchanged_amount`). `approximate_exchanged_amount` is always stored on each cycle. With **approximate exchange rate**, payments and totals use those stored amounts only. With **live exchange rate**, they use rates from the `exchange_rates` table, fetched from online sources and cached when missing. The `exchange_rates` table is used and populated only when the preference is **live exchange rate**. An exchange rate has the following attributes:
- `currency`: the base currency of the exchange rate, e.g. USD, EUR, GBP, etc.
- `rate`: (decimal) the exchange rate, e.g. 1.12 units of the default currency per 1 unit of currency
- `date`: Scopes the exchange rate to a specific date.

(`currency`, `date`) is unique.

Converted amounts (`approximate_exchanged_amount`, `rate`) do not record the currency they are denominated in, by design: they are always in the current default currency.
If the default currency changes, `approximate_exchanged_amount` is recalculated for all cycles.
In addition, if the default currency changes and the preference is **live exchange rate**, the exchange rates table is truncated and repopulated for the new default currency.
These recalculations, truncations and repopulations run atomically in a single transaction, so converted columns never mix currencies.
Exchange rates are fetched from online sources.

As `subtill` evolves, the data model may be extended to support more features. This introduces a need to version the database schema. The database schema is versioned using the `golang-migrate/migrate` package.
- Migrations are embedded in the program and are run automatically when a database is opened and the schema is outdated.
- If the database schema has a version greater than the current version, the program will refuse to open the database

## Core Features

The core features of `subtill` are:

- Track subscriptions: Add a new subscription, Cancel a subscription, Renew a subscription, List, Edit, Delete subscriptions and cycles, Search, sort and filter subscriptions by name, group, tags, extrapolated current monthly/yearly/lifetime cost, start date, end date, lifetime duration (i.e. number of months subscribed across all cycles)
- Payments: List all payments for a subscription. Payments are derived from subscription cycles (`starts_on` may be any day of the month; each charge falls on that same day every `frequency` months, computed from `starts_on`; in shorter months the day is clamped to the last day of the month and clamps back afterwards, e.g. Jan 31 → Feb 28 → Mar 31) and the active exchange-rate preference, not stored in the database.
- Reports (desktop GUI only): Total cost of specific subscriptions, subscription groups or all subscriptions since a specific date or for all time; lifetime totals; cost by month (past or future). Built client-side from `list`, `payments`, and `total`.
- Reminders: Set reminders for cancelling or renewing a subscription; list current and upcoming reminders in the desktop GUI; OS notifications when a reminder is due. Snooze updates the reminder's date and resets `notification_sent` to false; marking complete deletes the reminder.

## Interfaces

Note: Interface mode flags (e.g. `--cmd`, `--mcp`, `--api`) cannot be combined. If more than one is provided, an error is returned.

### Command Line Interface (`subtill --cmd`)

Note: Command names and arguments are indicative, not final. For the final list of commands and arguments, see the [Operations](20261018-operations.md) and [Command Line Flags](20261018-command-line-flags.md) documents.

The program works on the command line and exposes its features through a series of commands:

- `add`: add a new subscription; if it exists and is inactive, start a new cycle; if it is active, end the current cycle and start a new one (e.g. price change)
- `cancel`: end a subscription (i.e. end the current cycle); optionally deletes the subscription's pending "cancel" reminders
- `delete`: completely delete a subscription and all its associated cycles and reminders, as if it was never added
- `edit`: edit subscription metadata or an existing cycle
- `list`: display a list of subscriptions that match filter criteria
- `payments`: display a log of payments for subscriptions that match specific filter criteria
- `total`: display total cost of payments for subscriptions that match specific filter criteria
- `reminder`: add, list, snooze, and complete reminders (same semantics as the data model)

The cli also provides additional commands for initializing the database with specific preferences, and also updating the database preferences.
- `init`: initialize the database with specific preferences
- `update`: update the database preferences

Program options are as follows:
- `--db`: the path to the database file. Defaults to `./sub.db`
- `--output`: the output format. Defaults to `table`. Can be `table`, `json`, `csv`

Usage examples:

```bash
subtill --db ./sub.db --cmd add --name netflix.com --cost 14 --currency EUR --frequency 1 --ends-on 2024-12-01
subtill --db ./sub.db --cmd add --name netflix.com --cost 14 --currency EUR --frequency 1 --duration 1 # end date is exactly 1 month from today. duration must be a strict multiple of frequency
subtill --db ./sub.db --cmd cancel --name netflix.com
subtill --db ./sub.db --cmd list
subtill --output json --db ./sub.db --cmd payments --name netflix.com --since 2024-01-01 --until 2024-04-01
subtill --output json --db ./sub.db --cmd payments --group streaming --since 2024-01-01 --until 2024-04-01
subtill --db ./sub.db --cmd total --name netflix.com
subtill --output json --db ./sub.db --cmd total --name netflix.com
subtill --output json --db ./sub.db --cmd total --group streaming --since 2024-01-01 --until 2024-02-01
```

Here is a sample interaction with the subtill program. Note the ascii bullets (●, ○) represent the subscription status.

```bash
$ date
Sun 20 Jun 2021 07:30:00 PM EEST
$ subtill --cmd list
● scrimba.com		60.00 / 3 months	2021-04-05 -
● spotify.com    6.00 / 6 months	2021-03-15 - 2021-09-15
● netflix.com		14.00 / month   	2020-12-01 -
● xbox.com   		14.99 / month			2021-01-01 - 2021-03-01
$ subtill --cmd cancel --name netflix.com # subscription ends on the closest interval of the frequency
$ subtill --cmd list
● scrimba.com		60.00 / 3 months	2021-04-05 -
● spotify.com 	 6.00 / 6 months	2021-03-15 - 2021-09-15
● netflix.com		14.00 / month   	2020-12-01 - 2021-07-01
○ xbox.com   		14.99 / month 		2021-01-01 - 2021-03-01
$ subtill --cmd add --name dropbox.com --cost 12
$ subtill --cmd list
● dropbox.com		12.00 / month   	2021-06-20 -
● scrimba.com		60.00 / 3 months	2021-04-05 -
● spotify.com 	 6.00 / 6 months	2021-03-15 - 2021-09-15
● netflix.com		14.00 / month   	2020-12-01 - 2021-07-01
○ xbox.com   		14.99 / month 		2021-01-01 - 2021-03-01
$ subtill --cmd payments --name netflix.com --since 2021-01-01 --until 2021-03-31
2021-01-01 	14.00 	netflix.com
2021-02-01 	14.00 	netflix.com
2021-03-01 	14.00 	netflix.com
$ subtill --cmd total --name netflix.com
98.00
$ subtill --cmd payments --until 2021-07-20
2021-07-05 	60.00 	scrimba.com
2021-07-20 	12.00 	dropbox.com
$ subtill --cmd total --until 2021-07-20
72.00
```

In case of errors, the program will exit with a non-zero status code and print the error message to stderr.

### MCP Interface (`subtill --mcp`)

The program works as a MCP server and exposes its features through a series of MCP endpoints.

The mcp interface has feature parity with the command line interface, with the exception that the init command is not available. Otherwise, each subcommand is exposed as a tool, with the same subcommand inputs and the same json output as the cli command.

MCP tools always emit json. Errors are communicated in accordance with the MCP specification.

The MCP interface uses stdio as the transport layer.

To start the MCP server, run the program with the `--mcp` flag as follows:

```bash
subtill --db ./sub.db --mcp
```

### REST API (`subtill --api`)

The program exposes a REST API, documented using OpenAPI 3.1.

The API is secured using bearer token authentication. The **`/swagger`** documentation UI is **exempt** from bearer authentication (including assets required to render it) so it can be opened in a browser without a token; all **`/api/*`** JSON routes remain protected.

To start the REST API, run the program with the `--api` flag as follows:

```bash
$ subtill --db ./sub.db --api --port 8000 --token my-secret-token
API started on http://localhost:8000/api
Swagger documentation mounted at http://localhost:8000/swagger
Authorization Token: my-secret-token # not printed unless autogenerated
```

If --port is not provided, a random available high port is assigned.

If --token is not provided, a random token is autogenerated. A token can be provided either via the `--token` flag or via the `SUBTILL_TOKEN` environment variable.

The listener binds to `127.0.0.1` by default. A `--bind <ip>` flag can be used to bind to a different address, such as `0.0.0.0` to bind to all interfaces.

The API is available at `http://localhost:8000/api`, with HTML Swagger documentation at `http://localhost:8000/swagger`. The REST adapter does not serve the desktop React UI—only JSON routes and API documentation.

The REST API has feature parity with the command line interface, with the exception that the init command is not available. Otherwise, each subcommand is exposed as a route, with the same subcommand inputs and the same json output as the cli command.

### Desktop GUI

When first launched, user lands on a screen that allows them to create a new database or open an existing database file from disk. Creating a database prompts for file location, display name, and preferences (same as `init`), then creates the file, runs migrations, and stores preferences. They can also reopen from a list of the most recently opened databases.

The desktop GUI allows users to switch between databases

The desktop GUI performs data fetching through Tanstack Query with a [default query function](https://tanstack.com/query/latest/docs/framework/react/guides/default-query-function), which takes the query key and maps it to a wails binding.

In addition, wails bindings are used directly (i.e. not through tanstack query) for native and app-level features: file dialogs, the recent-databases list, switching databases and app-level preferences. Database-scoped preferences, such as changing the default currency, go through Tanstack Query.

The desktop GUI is launched with the following command:

```bash
subtill --db ./sub.db
```

The `--db` flag is optional and if not provided, the desktop GUI will prompt the user to create a new database or open an existing database file from disk. If provided, the desktop GUI will open the database file specified by the path.

An optional `--config-path` flag can be supplied to override the directory from which the desktop app reads and stores application configuration and state, such as recently opened databases and app preferences, which are otherwise stored in os.UserConfigDir(). Note that the desktop variant is the only one that needs to store application configuration and state -- the other variants are stateless in the sense that their configuration is passed as command line arguments, not stored or retrieved from disk.

The desktop GUI strictly enforces a single instance of the app, through the Single Instance Lock mechanism provided by Wails. If the app is already running, the desktop GUI will focus the existing instance and the --db flag will be ignored.

## Testing

Testing happens at three levels:

- Unit tests
  - Go: core functionality, using the built-in `testing` package.
  - Frontend: React components and hooks are tested with vitest, with data fetching mocked.
- Integration tests
  - Go: REST and MCP adapters are covered with integration-style tests, using the built-in `testing` package.
- E2E (end-to-end) tests
  - The desktop app is built in Wails server mode (`-tags server`). Application configuration is mocked through --config-path flag.

## Build and Release

The program is built and released using github actions, triggered by pushing a version tag to the repository.
Wails3 cross-compilation is used to build the program for all supported platforms.
