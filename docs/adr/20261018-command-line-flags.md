# Command Line Flags

Subtill accepts command line flags to configure:
- the mode of operation: api, mcp, cmd, desktop (--api, --mcp, --cmd; desktop is the default mode)
- the database it operates on (--db) (required for all modes, optional for desktop).

## Global Flags

### --db

The database is a SQLite database file. It should already exist on disk and it should be migrated to the current schema version (not earlier and not later). If these conditions are not met, the program will exit with an error, unless any of the following flags are also provided:

- --db-create: Create a new database if it does not exist (error if database already exists)
- --db-migrate: Migrate the database from an earlier schema version to the current schema version

A database with a later schema version than the current one is always refused.

## Mode Specific Flags

Each mode further accepts mode-specific flags. Mode-specific flags are parsed by the respective mode's entry point module.

### api

- --bind: the address to listen on (default: 127.0.0.1)
- --port: the port to listen on (default: 8000)
- --token: the token to use for authentication (default: SUBTILL_TOKEN or generated)

### mcp

None.

### desktop

- --config-path: the path to the configuration directory (default: os.UserConfigDir() + "/subtill")

### cmd

- --cmd: the subcommand to run (required)
- --output: the output format (accepts json, csv, table; default: table)

The cmd argument accepts one subcommand per invocation. Each subcommand maps to a single operation in [Operations](./20261018-operations.md). Subcommand names use **kebab-case**; request fields use **kebab-case** flags that correspond to the operation struct’s `json` field names (for example `billingCurrency` → `--billing-currency`).

Calendar dates use `YYYY-MM-DD`. Decimal amounts are passed as strings (same validation as the API). Repeat a flag to populate JSON arrays (for example `--tag`).

## Subcommands

### Subscriptions

| Subcommand | Operation |
|------------|-----------|
| `list` | `subscription.list` |
| `get` | `subscription.get` or `subscription.getByName` (see flags) |
| `start` | `subscription.start` |
| `edit` | `subscription.edit` |
| `cancel` | `subscription.cancel` |
| `resume` | `subscription.resume` |
| `delete` | `subscription.delete` |

### Billing periods

| Subcommand | Operation |
|------------|-----------|
| `billing-period-add` | `billingPeriod.add` |
| `billing-period-edit` | `billingPeriod.edit` |
| `billing-period-delete` | `billingPeriod.delete` |

### Payments

| Subcommand | Operation |
|------------|-----------|
| `payments` | `payment.list` |
| `total` | `payment.total` |

### Reminders

| Subcommand | Operation |
|------------|-----------|
| `reminders` | `reminder.list` |
| `reminder-add` | `reminder.add` |
| `reminder-snooze` | `reminder.snooze` |
| `reminder-clear` | `reminder.clear` |

### Preferences

| Subcommand | Operation |
|------------|-----------|
| `info` | `preferences.get` |
| `set` | `preferences.set` |

### Utilities

| Subcommand | Operation |
|------------|-----------|
| `icon-fetch` | `icon.fetch` |
| `currency-convert` | `currency.convert` |

## Command Flags

### Conventions

- **List and aggregate reads** (`list`, `payments`, `total`, `reminders`) build the operation’s `filter` and `sort` objects from list filter flags and `--sort-*` (see [list filter flags](#list-filter-flags) below). Cmd flags are **per subcommand** (for example `--name` on `list` sets `filter.name`; on `get` it selects lookup-by-name).
- **Patch-style writes** (`edit`, `billing-period-edit`, `set`) only send fields for flags that appear on the command line. Omitted flags are omitted from the request (unchanged in the service). Use `--clear-<field>` only for **nullable** string columns in [db-schema](./20261018-db-schema.md) (`group`, `notes`, `icon`, `url` on `edit`). Also use `--clear-tags` (remove all tags) and `--clear-end-on` on `billing-period-edit` where the operation allows null/empty semantics. On `edit`, when tag flags are combined, apply in order: `--tag` / `--clear-tags` (full replace), then `--remove-tag`, then `--add-tag`.
- **Boolean fields:** For flags that default to **false**, document only `--<name>` (opt-in). Omitting the flag is false; do not document `--no-<name>`. For flags that default to **true**, or patch commands where the CLI must send **false** explicitly, document `--no-<name>` (and `--<name>` when needed). On patch writes, omitting a boolean flag leaves the request field unset (unchanged); passing `--<name>` sets true in the request.
- **Auto flags** (`--icon-auto`, `--cost-auto`) are mutually exclusive with their manual counterparts (`--icon` / `--clear-icon`, `--cost`). `--cost-auto` requires `--billing-cost`.

### List filter flags

Nested filter shapes match [Operations](./20261018-operations.md) (`SubscriptionFilters`, `PaymentFilters`, `ReminderFilters`). Flags populate the request `filter` object (and `sort` where applicable).

**`StringFilter`** (`filter.name` on subscriptions, payments, and reminders):

- `--<field>` — `value` (e.g. `--name`)
- `--<field>-operator` — `contains` or `equals` (default `equals` when omitted)

**`ArrayFilter`** (`filter.tags` on subscriptions and payments):

- `--tag` — repeat for each tag (`values`)
- `--tags-operator` — `all` or `any` (default `any` when omitted)

**`DateRangeFilter`** (`startOn`, `restartOn`, `endOn`, `paymentDate`, `triggerOn`):

- `--<field>-gte` — inclusive lower bound (kebab-case field, e.g. `--start-on-gte`, `--payment-date-gte`)
- `--<field>-lt` — exclusive upper bound (e.g. `--payment-date-lt`)

**`IntegerRangeFilter`** (`startInterval`, `restartInterval`, `endInterval`, `duration`, `lifetimeDuration`):

- `--<field>-gte`
- `--<field>-lte`

**`DecimalRangeFilter`** (`billingCost`, `cost`, `monthlyCost`, `yearlyCost`, `lifetimeCost`):

- `--<field>-gte`
- `--<field>-lte`

**Subscription-only scalar filters** (`list`):

- `--group`
- `--status` — `active` or `inactive`
- `--billing-status` — `ongoing`, `cancelScheduled`, or `canceled`
- `--billing-currency`
- `--frequency` — positive integer

**Payment-only scalar filters** (`payments`, `total`):

- `--group`
- `--status` — `active` or `inactive`

`paymentDate` on payments uses `--payment-date-gte` / `--payment-date-lt`. When both are omitted, the service applies the defaults documented in Operations before validation.

**Reminder-only scalar filters** (`reminders`):

- `--action` — `cancel`, `renew`, or `custom` (exact match)

**Sort** (`list`, `reminders`):

- `--sort-field` — subscription: `id`, `name`, `startOn`, `restartOn`, `endOn`, `duration`, `lifetimeDuration`, `monthlyCost`, `yearlyCost`, `lifetimeCost`; reminder: `name`, `triggerOn`
- `--sort-dir` — `asc` or `desc`

### `list` (`subscription.list`)

No required flags. Accepts all [list subscription filter flags](#list-filter-flags) and [sort flags](#list-filter-flags).

### `get` (`subscription.get` / `subscription.getByName`)

Exactly one of:

- `--id` — `subscription.get`
- `--name` — `subscription.getByName` (case-insensitive exact match)

### `start` (`subscription.start`)

- `--name` (required)
- `--group`
- `--tag` — repeat for each tag
- `--notes`
- `--icon` — not with `--icon-auto`
- `--icon-auto` — fetch favicon using `name` as domain; not with `--icon`
- `--url`
- `--start-on` — defaults to today when omitted
- `--frequency` (required) — positive integer (months)
- `--duration` — non-negative integer, multiple of `--frequency`; `0` means ongoing
- `--billing-currency` — required with `--billing-cost` when either is set
- `--billing-cost` — required with `--billing-currency` when either is set
- `--cost` — required unless `--cost-auto`; preferred-currency amount (from `currency-convert` when converting manually); not with `--cost-auto`
- `--cost-auto` — convert `--billing-cost` to preferred currency; requires `--billing-cost`; not with `--cost`

### `edit` (`subscription.edit`)

- `--id` (required)
- At least one of the optional fields below (matches `at_least_one_set` validation)

Subscription metadata:

- `--name` — rename; required column (no `--clear-name`)
- `--group` / `--clear-group`
- `--tag` — repeat; replaces all tags when set
- `--clear-tags` — remove all tags
- `--add-tag` — repeat; add tags without replacing the full set (idempotent)
- `--remove-tag` — repeat; remove tags without replacing the full set (idempotent)
- `--notes` / `--clear-notes`
- `--icon` / `--clear-icon` — not with `--icon-auto`
- `--icon-auto` — fetch favicon using effective subscription name; not with `--icon` or `--clear-icon`
- `--url` / `--clear-url`

Latest billing period:

- `--frequency` — positive integer
- `--billing-currency` — pair with `--billing-cost` when setting billing amounts
- `--billing-cost` — pair with `--billing-currency` when setting billing amounts
- `--cost` — not with `--cost-auto`
- `--cost-auto` — convert `--billing-cost` to preferred currency; requires `--billing-cost`; not with `--cost`

### `cancel` (`subscription.cancel`)

- `--id` (required)
- `--duration` — non-negative integer; multiple-of-frequency is checked in the service (default `0` ends on the next charge date)
- `--delete-cancel-reminders`

### `resume` (`subscription.resume`)

- `--id` (required)

### `delete` (`subscription.delete`)

- `--id` (required)

### `billing-period-add` (`billingPeriod.add`)

- `--subscription-id` (required)
- `--start-on` (required)
- `--end-on` (required) — must be strictly after `--start-on`
- `--frequency` (required) — positive integer
- `--billing-currency` — required with `--billing-cost` when either is set
- `--billing-cost`
- `--cost` — required unless `--cost-auto`; not with `--cost-auto`
- `--cost-auto` — requires `--billing-cost`; not with `--cost`

### `billing-period-edit` (`billingPeriod.edit`)

- `--id` (required)
- `--start-on`
- `--end-on` — clearing allowed only on the latest period (`--clear-end-on`)
- `--clear-end-on`
- `--frequency` — positive integer
- `--billing-currency`, `--billing-cost` — same pairing rules as `edit`
- `--cost` — not with `--cost-auto`
- `--cost-auto` — requires `--billing-cost`; not with `--cost`

### `billing-period-delete` (`billingPeriod.delete`)

- `--id` (required)

### `payments` (`payment.list`)

No required flags. Accepts [payment list filter flags](#list-filter-flags).

### `total` (`payment.total`)

Same flags as `payments`. Projected totals: set `--payment-date-lt` to a future date.

### `reminders` (`reminder.list`)

No required flags. Accepts [reminder list filter flags](#list-filter-flags) and [sort flags](#list-filter-flags).

### `reminder-add` (`reminder.add`)

- `--subscription-id` (required)
- `--action` (required) — `cancel`, `renew`, or `custom`
- `--custom-note` — required when `--action custom`; ignored for `cancel` and `renew` when set
- `--trigger-on` (required)

### `reminder-snooze` (`reminder.snooze`)

- `--id` (required)
- `--days` (required) — positive integer

### `reminder-clear` (`reminder.clear`)

- `--id` (required)

### `info` (`preferences.get`)

No flags.

### `set` (`preferences.set`)

At least one of:

- `--currency`
- `--currency-conversion-mode` — `approximate` or `live`

### `icon-fetch` (`icon.fetch`)

Exactly one of:

- `--from-image-url`
- `--from-domain`
- `--from-subscription-id`

### `currency-convert` (`currency.convert`)

- `--currency` (required)
- `--amount` — defaults to `1` when omitted
- `--exchange-on` — defaults to the current date when omitted
