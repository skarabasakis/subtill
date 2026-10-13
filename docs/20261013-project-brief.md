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

- Core: Go
  - Web Framework: built-in `net/http` package
  - Database: Turso (SQLite)
    - ORM: built-in `database/sql` package
    - Schema Migrations: `golang-migrate/migrate` package
  - Desktop GUI Framework: Wails3
  - Testing: built-in `testing` package
- Frontend: React, Tailwind and Shadcn UI
  - Runtime: Node.js 24.20.0
  - Language: TypeScript (wails3 default)
  - Package Manager: npm
  - Tooling: Vite (wails3 default)
    - Linter: Oxlint
    - Formatter: Oxfmt
    - Testing: vitest
  - Framework: React 18.2 (wails3 default)
    - Routing: HashRouter from react-router-dom
    - State Management: Tanstack Query
    - Form Handling: Tanstack Form
    - Table Handling: Tanstack Table
    - Charts: Tanstack Charts
  - UI Components
    - Styling: Tailwind CSS
    - Component Library: Shadcn UI
    - Icons: Lucide React

## Data Model

A subscription record has the following attributes:

- `name`: the name of the subscription. Unique within the database.
- `group`: Can optionally be set to group subscriptions together.
- `tags`: Array of strings. Can optionally be set to add tags to the subscription.
- `notes`: A string. Can optionally be set to add notes to the subscription.
- `icon`: A base64-encoded favicon of the subscription.
- `url`: A website URL from which the subscription can be managed.

A subscription can have multiple cycles, i.e. it can be stopped and started again. Each cycle has its own start and end date. In addition, the price of a subscription can change over time, and this is modeled by creating a new cycle. Thus each subscription cycle has the following attributes:
- `subscription_id`: the id of the subscription
- `frequency`: (decimal) the monthly frequency of the subscription, e.g 1 = monthly, 3 = quarterly, 6 = semiannual, 12 = yearly, 24 = every 2 years, 0.5 = bi-monthly
- `cost`: Required. The cost of the subscription, in the subscription's currency, which can be different from the user's default currency.
  - `currency`: the currency of the subscription cost, e.g. USD, EUR, GBP, etc.
  - `amount`: (decimal) the amount of the subscription cost, e.g. 14.00
- `starts_on`: the date the subscription was charged and the cycle started, e.g. 2021-01-01
- `ends_on`: the date the cycle ends. It can be null if the subscription is ongoing.

A subscription cycle can be associated with one or more reminders, which are triggered on a specific date and remain active until deleted. A reminder has the following attributes:
- `cycle_id`: the id of the cycle
- `action`: (enum) the action to perform when the reminder is triggered, e.g. "cancel", "renew"
- `note`: (string) a note to display when the reminder is triggered
- `date`: the date the reminder is triggered, e.g. 2021-01-01
- `status`: the status of the reminder, e.g. active, deleted

The user can set database-scoped preferences, such as database name and default currency.

When calculating total costs, the program should convert the costs to the user's default currency using the exchange rate of the date of the cost. Exchange rates are retrieved from online sources when first needed and cached in the database. An exchange rate has the following attributes:
- `currency`: the base currency of the exchange rate, e.g. USD, EUR, GBP, etc.
- `rate`: (decimal) the exchange rate, e.g. 1.12
- `date`: Optional. Scopes the exchange rate to a specific date.

As `subtill` evolves, the data model may be extended to support more features. This indroduces a need to version the database schema. The database schema is versioned using the `golang-migrate/migrate` package.
- Migrations are embedded in the program and are run automatically when a database is opened and the schema is outdated.
- If the database schema has a version greater that the current version, the program will refuse to open the database

## Core Features

The core features of `subtill` are:

- Track subscriptions: Add a new subscription, Cancel a subscription, Renew a subscription, List, Edit, Delete subscriptions and cycles, Search and filter subscriptions by name, group, tags, lifetime
- Payments: List all payments for a subscription
- Reports: Total cost of specific subscriptions, subscription groups or all subscriptions since a specific date or for all time. Total lifetime cost of specific subscriptions. Total cost of all subscriptions by month (past or future)
- Reminders: Set reminders for cancelling or renewing a subscription, List active reminders, Let user snooze reminders and mark reminders as completed

## Interfaces

### Command Line Interface (`subtill <program-options> --command <subcommand> <subcommand-options>`)

The program works on the command line and exposes its features through a series of commands:

- `add`: start tracking a new subscription, or renew it if it already exists
- `cancel`: end a subscription and stop tracking it
- `delete`: completely delete a subscription and all its associated cycles and reminders, as if it was never added
- `list`: display a list of tracked subscriptions
- `payments`: display a log of subscription payments
- `cost`: display total cost of subscription payments


Program options are as follows:
- `--db`: the path to the database file. Defaults to `./sub.db`
- `--output`: the output format. Defaults to `table`. Can be `table`, `json`, `csv`

Usage examples:


```bash
subtill --db ./sub.db --command add --id netflix.com --cost 14 --currency EUR --frequency 1 --ends-on 2024-12-01
subtill --db ./sub.db --command add --id netflix.com --cost 14 --currency EUR --frequency 1 --duration 1 # end date is exactly 1 month from today. duration must be a strict multiple of frequency
subtill --db ./sub.db --command cancel --id netflix.com
subtill --db ./sub.db --command list
subtill --output json --db ./sub.db --command payments --id netflix.com --since 2024-01-01 --until 2024-04-01
subtill --output json --db ./sub.db --command payments --group streaming --since 2024-01-01 --until 2024-04-01
subtill --db ./sub.db --command total --id netflix.com
subtill --output json --db ./sub.db --command total --group streaming --since 2024-01-01 --until 2024-02-01
subtill --output json --db ./sub.db --command total --id netflix.com
subtill --output json --db ./sub.db --command total --group streaming --since 2024-01-01 --until 2024-02-01
```

Here is a sample interaction with the subtill program. Note the ascii bullets (●, ○) represent the subscription status.

```bash
$ date
Sun 20 Jun 2021 07:30:00 PM EEST
$ subtill --command list
● scrimba.com		60.00 / 3 months	2021-04-05 -
● spotify.com    6.00 / 6 months	2021-03-15 - 2021-09-15
● netflix.com		14.00 / month   	2020-12-01 -
● xbox.com   		14.99 / month			2021-01-01 - 2021-03-01
$ subtill --command cancel --id netflix.com # subscription ends on the closest interval of the frequency
$ subtill --command list
● scrimba.com		60.00 / 3 months	2021-04-05 -
● spotify.com 	 6.00 / 6 months	2021-03-15 - 2021-09-15
● netflix.com		14.00 / month   	2020-12-01 - 2021-07-01
○ xbox.com   		14.99 / month 		2021-01-01 - 2021-03-01
$ subtill --command add dropbox.com --cost 12
$ subtill --command list
● dropbox.com		12.00 / month   	2021-06-20 -
● scrimba.com		60.00 / 3 months	2021-04-05 -
● spotify.com 	 6.00 / 6 months	2021-03-15 - 2021-09-15
● netflix.com		14.00 / month   	2020-12-01 - 2021-06-22
○ xbox.com   		14.99 / month 		2021-01-01 - 2021-03-01
$ subtill --command payments --id netflix.com --since 2021-01-01 --until 2021-03-31
2021-01-01 	14.00 	netflix.com
2021-02-01 	14.00 	netflix.com
2021-03-01 	14.00 	netflix.com
$ subtill --command total netflix.com
98.00
$ subtill --command payments --until 2021-07-20
2021-07-05 	60.00 	scrimba.com
2021-07-20 	12.00 	dropbox.com
$ subtill --command total --until 2021-07-20
72.00
```

In case of errors, the program will exit with a non-zero status code and print the error message to stderr.

### MCP Interface (`sub-mcp`)

The program works as a MCP server and exposes its features through a series of MCP endpoints. The mcp interface has feature parity with the command line interface. Each subcommand is exposed as a tool.

MCP tools always emit json. The json is wrapped in a `{"status": "success" | "error", "result": ...}` object. In case of error, the `result` field will be null and the `message` field will contain the error message.

The MCP interface uses stdio as the transport layer. The program reads JSON from stdin and writes JSON to stdout.

To start the MCP server, run the program with the `--mcp` flag as follows:

```bash
subtill --db ./sub.db --mcp
```

### REST API

The program exposes a REST API, documented using OpenAPI 3.0. The API is versioned and the current version is v1.

The API is secured using either of the following authentication methods:
- bearer token authentication or basic authentication. The token is passed in the `Authorization` header.
- cookie authentication. The token is passed in the query

To start the REST API, run the program with the `--http` flag as follows:

```bash
$ sub-cli --db ./sub.db --http --port 8000 --token my-secret-token
Web UI started on http://localhost:8000/#/login/my-secret-token
API started on http://localhost:8000/api
Authorization Token: my-secret-token
```

If --port is not provided, a random available high port is assigned.

The API is available at `http://localhost:8000/api/v1`, with html swagger documentation at `http://localhost:8000/api/docs`.

The REST API has feature parity with the command line interface.

### Web UI

The program has a web UI, built with React, Tailwind and Shadcn UI. For routing, hash routing is used. The UI is available at `http://localhost:8000/`.

The Web UI starts alongside the REST API with the same command. The --auth-token is passed to the web UI via hash, then stored in session storage so it can be used for subsequent requests.

### Desktop GUI

The web ui is wrapped in a desktop GUI using Wails. The desktop GUI is different from the web UI in the following aspects:
- When first launched, user lands on a screen that allows them to create a new database or open an existing database file from disk. They can also reopen from a list of the most recently opened databases.
- The desktop GUI allows users to switch between databases
- The desktop GUI calls backend functionality via RPC calls to exported go functions, as Wails allows.

The desktop GUI is launched with the following command:

```bash
subtill --db ./sub.db
```

The `--db` flag is optional and if not provided, the desktop GUI will prompt the user to create a new database or open an existing database file from disk. If provided, the desktop GUI will open the database file specified by the path.

A db lock file is used to prevent multiple instances of the desktop GUI from opening the same database file. If the database is locked, the user is warned and can still force open the database by replacing the lock file.

## Testing

The program is implements testing at two levels:
- unit testing of core functionality & unit testing of react frontend functionality (with mocked state)
- integration testing of wails app & the backend & integration testing of the web ui with http server, both with playwright

## Build and Release

The program is built and released using github actions, triggered by pushing a version tag to the repository.
Wails3 cross-compilation is used to build the program for all supported platforms.
