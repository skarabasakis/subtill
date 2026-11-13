# Operations

This document provides a list of operations that can be performed on an existing subscription database. Each of these operations is exposed through:

- the command line interface (cmd)
- the API (api)
- the MCP (mcp)

## List of Operations

### Subscriptions

- `subscription.list`: return a list of subscriptions that match filter criteria, along with their associated billing periods and reminders
- `subscription.getByName`: return a subscription by name, along with its associated billing periods and reminders
- `subscription.get`: return a subscription by ID, along with its associated billing periods and reminders
- `subscription.start`: start or restart a subscription or change the price; if it does not exist, create it; if it exists and is ended, start a new cycle; if it exists and is ongoing, end the current cycle and start a new one (e.g. price change) with the end date specified by the user
- `subscription.cancel`: end a subscription (i.e. end the current cycle) and optionally cancel all associated "cancel" reminders
- `subscription.resume`: restart a subscription (uncancel or extend the latest, still active but canceled, billing period)
- `subscription.delete`: completely delete a subscription and all its associated cycles and reminders, as if it was never added
- `subscription.edit`: edit subscription metadata or edit the current billing period

### Billing Periods

- `billingPeriod.add`: add a billing period to a subscription (any dates; must not overlap existing periods)
- `billingPeriod.edit`: edit a billing period
- `billingPeriod.delete`: delete a billing period

### Payments

- `payment.list`: display a log of payments for subscriptions that match specific filter criteria
- `payment.total`: display total cost (either current or projected) of payments for subscriptions that match specific filter criteria

### Reminders

- `reminder.list`: list all reminders
- `reminder.add`: add a new reminder
- `reminder.snooze`: add a positive number of calendar days to the reminder's current `triggerOn` and mark its notification as not sent
- `reminder.clear`: clear a reminder

### Preferences

- `preferences.get`: get current values of database preferences (e.g. default currency, currency conversion mode)
- `preferences.set`: set values of database preferences (e.g. default currency, currency conversion mode)

### Utilities

- `icon.fetch`: resolve a base64-encoded PNG from a favicon URL, a domain, or another subscription’s stored `icon` (or set `iconAuto` on `subscription.start` / `subscription.edit`)
- `currency.convert`: convert an amount in a billing currency to the database preferred currency at a given exchange date; the returned `converted.amount` is the `cost` passed manually or via `costAuto` on `subscription.start`, `subscription.edit`, `billingPeriod.add`, or `billingPeriod.edit`

## Specification of Operations

Each operation is described in terms of its:

- name
- Request params (struct with `json` and `validate` tags)
  - input data validation rules and related errors
  - idempotency rules
- Successful Response (struct with json tags)
- Error Conditions

Request validation uses [`github.com/go-playground/validator`](https://github.com/go-playground/validator). Apply documented JSON defaults (for example `currency.convert` `amount`, payment `paymentDate` bounds) **before** `validator.Struct`, then run operation logic. Validation failures are rejected before logical checks. Implementations may map validator failures to stable API error codes separately from the logical error codes below.

### Custom validate tags

Project-registered tags used in request and filter structs (in addition to built-in tags such as `required`, `gt`, `gte`, `oneof`, `required_with`, `required_without`, `excluded_with`, `gtfield`, `omitempty`, `dive`):

| Tag | Behavior |
|-----|----------|
| `decimal` | Non-empty valid non-negative decimal string (`govalues/decimal` rules). |
| `currency` | Non-empty supported ISO-like currency code. |
| `date_range` | Struct-level on `DateRangeFilter`: if both bounds are set, `gte < lt` (calendar dates). |
| `int_range` | Struct-level on `IntegerRangeFilter`: if both bounds are set, `gte <= lte`. |
| `decimal_range` | Struct-level on `DecimalRangeFilter`: if both bounds are set, compare with `govalues/decimal` (`gte <= lte`). |
| `multiple_of` | Cross-field on integers; param is another field name (e.g. `multiple_of=Frequency`). When the divisor field is > 0, value ≥ 0 and `value % divisor == 0`. Validate the divisor with `gt=0` first. Not used on `subscription.cancel` (divisor comes from the database). |
| `at_least_one_set` | Struct-level: at least one optional pointer field is non-nil (`subscription.edit`, `preferences.set`). |

### Request and Response schemas

#### subscription.list

Return a list of subscriptions that match filter criteria

```go
type SubscriptionListRequest struct {
  Filter SubscriptionFilters `json:"filter" validate:"dive"`
  Sort   []SubscriptionSort    `json:"sort"   validate:"dive"`
}

type SubscriptionListResponse struct {
  Subscriptions []Subscription `json:"subscriptions"`
}
```

#### subscription.get

Return a subscription by ID, along with its associated billing periods and reminders

```go
type SubscriptionGetRequest struct {
  ID string `json:"id" validate:"required"`
}

type SubscriptionGetResponse struct {
  Subscription Subscription `json:"subscription"`
}
```

#### subscription.getByName

Return a subscription by name (case-insensitive exact match), along with its associated billing periods and reminders

```go
type SubscriptionGetByNameRequest struct {
  Name string `json:"name" validate:"required"`
}

type SubscriptionGetByNameResponse struct {
  Subscription Subscription `json:"subscription"`
}
```

#### subscription.start

Add a new subscription; if it exists and is inactive, start a new cycle; if it is active, end the current cycle and start a new one (e.g. price change). When both `billingCurrency` and `billingCost` are omitted, the service sets `billing_currency` to the database preferred currency and `billing_cost` to `cost` before persist (apply this before `costAuto`).

When `iconAuto` is true, the service resolves `icon` via `icon.fetch` with `fromDomain` set to `name` (after validation, before persist). The request must not include `icon` when `iconAuto` is true.

When `costAuto` is true, the service sets `cost` via `currency.convert` using `billingCurrency`, `billingCost`, and `exchangeOn` equal to the minimum of the effective `startOn` (today when zero) and the current date. The request must not include `cost` when `costAuto` is true. `costAuto` requires `billingCost` in the request; billing defaults do not satisfy `required_with` unless `billingCost` is present on the request.

```go
type SubscriptionAddRequest struct {
  Name            string    `json:"name"         validate:"required"`
  Group           string    `json:"group"`
  Tags            []string  `json:"tags"`
  Notes           string    `json:"notes"`
  Icon            string    `json:"icon" validate:"excluded_if=IconAuto true"`
  IconAuto        bool      `json:"iconAuto,omitempty"`
  URL             string    `json:"url"`
  StartOn         time.Date `json:"startOn"` // defaults to today when zero; empty allowed
  Frequency       int       `json:"frequency"  validate:"required,gt=0"`
  Duration        int       `json:"duration"   validate:"gte=0,multiple_of=Frequency"`
  BillingCurrency string    `json:"billingCurrency" validate:"omitempty,currency,required_with=BillingCost"`
  BillingCost     string    `json:"billingCost" validate:"omitempty,decimal,required_with=BillingCurrency"`
  Cost            string    `json:"cost"         validate:"omitempty,decimal,required_without=CostAuto,excluded_if=CostAuto true"`
  CostAuto        bool      `json:"costAuto,omitempty" validate:"required_with=BillingCost"`
}

type SubscriptionAddResponse struct {
  Subscription Subscription `json:"subscription"`
}
```

#### subscription.edit

Edit subscription metadata or the latest billing period. Omitted (nil) fields are left unchanged. For nullable metadata strings (`group`, `notes`, `icon`, `url`), a non-nil pointer to `""` clears the stored value. `name` is required in the database; when `name` is set it must be non-empty (it cannot be cleared to `""`).

`addTags` and `removeTags` add or remove individual tags without replacing the full set. They are idempotent: adding a tag that already exists or removing a tag that does not exist is a no-op. When more than one tag field is present in a single request, apply in order: `tags` replacement, then `removeTags`, then `addTags`.

When `iconAuto` is true, the service resolves `icon` via `icon.fetch` with `fromDomain` set to the subscription name after applying other metadata fields in the same request (or the stored name if `name` is omitted). The request must not include `icon` when `iconAuto` is true (including a non-nil `icon` set to `""`).

When `costAuto` is true on the latest billing period, the service sets `cost` via `currency.convert` using `billingCurrency`, `billingCost`, and `exchangeOn` equal to the minimum of the effective `startOn` of that period (after applying coalesced request fields) and the current date. The request must not include `cost` when `costAuto` is true. `costAuto` requires `billingCost` in the request.

```go
// validate:"at_least_one_set" on SubscriptionEditRequest
type SubscriptionEditRequest struct {
  ID string `json:"id" validate:"required"`

  // Subscription metadata
  Name       *string    `json:"name,omitempty" validate:"omitempty,min=1"`
  Group      *string    `json:"group,omitempty"`
  Tags       *[]string  `json:"tags,omitempty"`       // replaces all tags; [] removes all tags
  AddTags    *[]string  `json:"addTags,omitempty"`
  RemoveTags *[]string  `json:"removeTags,omitempty"`
  Notes      *string    `json:"notes,omitempty"`
  Icon       *string    `json:"icon,omitempty" validate:"excluded_if=IconAuto true"` // or `iconAuto`; or use `icon.fetch` manually
  IconAuto   *bool      `json:"iconAuto,omitempty"`
  URL        *string    `json:"url,omitempty"`

  // Latest billing period
  Frequency       *int    `json:"frequency,omitempty"       validate:"omitempty,gt=0"`
  BillingCurrency *string `json:"billingCurrency,omitempty" validate:"omitempty,currency,required_with=BillingCost"`
  BillingCost     *string `json:"billingCost,omitempty"     validate:"omitempty,decimal,required_with=BillingCurrency"`
  Cost            *string `json:"cost,omitempty"            validate:"omitempty,decimal,required_with=BillingCurrency,required_with=BillingCost,excluded_if=CostAuto true"`
  CostAuto        *bool   `json:"costAuto,omitempty"        validate:"required_with=BillingCost"`
}

type SubscriptionEditResponse struct{}
```

#### subscription.cancel

End a subscription (i.e. set the end date of the current billing period) and optionally delete its pending "cancel" reminders

```go
type SubscriptionCancelRequest struct {
  ID                    string `json:"id"                       validate:"required"`
  Duration              int    `json:"duration,omitempty"       validate:"gte=0"` // see Notes; multiple-of-frequency checked in service
  DeleteCancelReminders bool   `json:"deleteCancelReminders,omitempty"`
}

type SubscriptionCancelResponse struct{}
```

#### subscription.resume

Uncancel a subscription whose latest billing period is still active but has an end date, by clearing that end date

```go
type SubscriptionResumeRequest struct {
  ID string `json:"id" validate:"required"`
}

type SubscriptionResumeResponse struct{}
```

#### subscription.delete

Completely delete a subscription and all its associated billing periods, tags and reminders

```go
type SubscriptionDeleteRequest struct {
  ID string `json:"id" validate:"required"`
}

type SubscriptionDeleteResponse struct{}
```

#### billingPeriod.add

Add a billing period to a subscription (past, present, or future dates). The period must not overlap any existing billing period of the subscription. When both `billingCurrency` and `billingCost` are omitted, the service sets `billing_currency` to the database preferred currency and `billing_cost` to `cost` before persist (apply this before `costAuto`).

When `costAuto` is true, the service sets `cost` via `currency.convert` using `billingCurrency`, `billingCost`, and `exchangeOn` equal to the minimum of `startOn` and the current date. The request must not include `cost` when `costAuto` is true. `costAuto` requires `billingCost` in the request.

```go
type BillingPeriodAddRequest struct {
  SubscriptionID  string    `json:"subscriptionId"  validate:"required"`
  StartOn         time.Date `json:"startOn"         validate:"required"`
  EndOn           time.Date `json:"endOn"           validate:"required,gtfield=StartOn"`
  Frequency       int       `json:"frequency"       validate:"required,gt=0"`
  BillingCurrency string    `json:"billingCurrency" validate:"omitempty,currency,required_with=BillingCost"`
  BillingCost     string    `json:"billingCost"     validate:"omitempty,decimal,required_with=BillingCurrency"`
  Cost            string    `json:"cost"            validate:"omitempty,decimal,required_without=CostAuto,excluded_if=CostAuto true"`
  CostAuto        bool      `json:"costAuto,omitempty" validate:"required_with=BillingCost"`
}

type BillingPeriodAddResponse struct{}
```

#### billingPeriod.edit

Edit a billing period. Omitted (nil) fields are left unchanged. Only `endOn` may be cleared (set to null on the latest billing period); other stored columns are NOT NULL and must not be cleared. The edited billing period must not overlap any other billing period of the subscription.

When `costAuto` is true, the service sets `cost` via `currency.convert` using `billingCurrency`, `billingCost`, and `exchangeOn` equal to the minimum of the effective `startOn` of the period (after applying coalesced request fields) and the current date. The request must not include `cost` when `costAuto` is true. `costAuto` requires `billingCost` in the request.

```go
type BillingPeriodEditRequest struct {
  ID              string     `json:"id"              validate:"required"`
  StartOn         *time.Date `json:"startOn,omitempty"`
  EndOn           *time.Date `json:"endOn,omitempty"` // can only be cleared on the latest billing period
  Frequency       *int       `json:"frequency,omitempty"       validate:"omitempty,gt=0"`
  BillingCurrency *string    `json:"billingCurrency,omitempty" validate:"omitempty,currency,required_with=BillingCost"`
  BillingCost     *string    `json:"billingCost,omitempty"     validate:"omitempty,decimal,required_with=BillingCurrency"`
  Cost            *string    `json:"cost,omitempty"            validate:"omitempty,decimal,required_with=BillingCurrency,required_with=BillingCost,excluded_if=CostAuto true"`
  CostAuto        *bool      `json:"costAuto,omitempty"        validate:"required_with=BillingCost"`
}

type BillingPeriodEditResponse struct{}
```

#### billingPeriod.delete

Delete a billing period. A subscription's only billing period can't be deleted; use `subscription.delete` instead.

```go
type BillingPeriodDeleteRequest struct {
  ID string `json:"id" validate:"required"`
}

type BillingPeriodDeleteResponse struct{}
```

#### payment.list

Return the payments of subscriptions that match filter criteria, ordered by date, then subscription name

```go
type PaymentListRequest struct {
  Filter PaymentFilters `json:"filter" validate:"dive"`
}

type PaymentListResponse struct {
  Payments []Payment `json:"payments"`
}
```

#### payment.total

Return the total cost, in the database default currency, of the payments of subscriptions that match filter criteria. Projected totals are obtained by setting `filter.paymentDate.lt` to a future date.

```go
type PaymentTotalRequest struct {
  Filter PaymentFilters `json:"filter" validate:"dive"`
}

type PaymentTotalResponse struct {
  BillingCurrency string `json:"billingCurrency,omitempty"` // set only when every payment shares one billing currency and it differs from the preferred currency
  BillingCost     string `json:"billingCost,omitempty"`     // sum of `billingCost` in that currency; omitted with `billingCurrency`
  Cost            string `json:"cost"`                      // total in the database preferred currency
}
```

#### reminder.list

Return reminders that match filter criteria

```go
type ReminderListRequest struct {
  Filter ReminderFilters `json:"filter" validate:"dive"`
  Sort   []ReminderSort  `json:"sort"   validate:"dive"`
}

type ReminderListResponse struct {
  Reminders []Reminder `json:"reminders"`
}
```

#### reminder.add

Add a reminder to a subscription. For `cancel` and `renew`, a non-empty `customNote` is allowed but ignored at the service layer (not persisted).

```go
type ReminderAddRequest struct {
  SubscriptionID string         `json:"subscriptionId" validate:"required"`
  Action         ReminderAction `json:"action"         validate:"required,oneof=cancel renew custom"`
  CustomNote     string         `json:"customNote,omitempty" validate:"required_if=Action custom"`
  TriggerOn      time.Date      `json:"triggerOn"      validate:"required"`
}

type ReminderAddResponse struct {
  Reminder Reminder `json:"reminder"`
}
```

#### reminder.snooze

Add `days` calendar days to the reminder's current `triggerOn` and mark its notification as not sent. The service sets `triggerOn` to `currentTriggerOn.AddDays(days)`.

```go
type ReminderSnoozeRequest struct {
  ID   string `json:"id"   validate:"required"`
  Days int    `json:"days" validate:"required,gt=0"`
}

type ReminderSnoozeResponse struct{}
```

#### reminder.clear

Mark a reminder as complete, deleting it

```go
type ReminderClearRequest struct {
  ID string `json:"id" validate:"required"`
}

type ReminderClearResponse struct{}
```

#### preferences.get

Return the database preferences

```go
type PreferencesGetRequest struct{}

type PreferencesGetResponse struct {
  Preferences PreferencesRecord `json:"preferences"`
}
```

#### preferences.set

Set database preferences. Omitted (nil) fields are left unchanged. Changing `Currency` recalculates the default-currency cost of all billing periods (and, in live mode, repopulates exchange rates) in a single transaction.

```go
// validate:"at_least_one_set" on PreferencesSetRequest
type PreferencesSetRequest struct {
  Currency         *string           `json:"currency,omitempty"         validate:"omitempty,currency"`
  CurrencyConversionMode *CurrencyConversionMode `json:"currencyConversionMode,omitempty" validate:"omitempty,oneof=approximate live"`
}

type PreferencesSetResponse struct{}
```

#### icon.fetch

Return a base64-encoded PNG. Exactly one of `fromImageUrl`, `fromDomain`, or `fromSubscriptionId` must be provided.

When `fromImageUrl` or `fromDomain` is set, fetch a favicon over the network. When `fromSubscriptionId` is set, load the subscription by ID and return its stored `icon` field as-is (no network fetch).

```go
type IconFetchRequest struct {
  FromImageUrl       string `json:"fromImageUrl,omitempty"       validate:"required_without=FromDomain FromSubscriptionId,excluded_with=FromDomain FromSubscriptionId"`
  FromDomain         string `json:"fromDomain,omitempty"         validate:"required_without=FromImageUrl FromSubscriptionId,excluded_with=FromImageUrl FromSubscriptionId"`
  FromSubscriptionId string `json:"fromSubscriptionId,omitempty" validate:"required_without=FromImageUrl FromDomain,excluded_with=FromImageUrl FromDomain"`
}

type IconFetchResponse struct {
  Icon string `json:"icon"` // base64-encoded PNG
}
```

#### currency.convert

Convert an amount in a billing currency to the database preferred currency, using the exchange rate on a specific date

```go
type CurrencyConvertRequest struct {
  Currency   string    `json:"currency"           validate:"required,currency"`
  Amount     string    `json:"amount,omitempty"     validate:"omitempty,decimal"` // defaults to 1 before validate
  ExchangeOn time.Date `json:"exchangeOn,omitempty"` // defaults to the current date before validate
}

type CurrencyAmount struct {
  Currency string `json:"currency"`
  Amount   string `json:"amount"`
}

type CurrencyConvertResponse struct {
  Source     CurrencyAmount `json:"source"`     // echoes request `currency` and `amount` (after defaults)
  Converted  CurrencyAmount `json:"converted"`  // preferred currency and converted amount
  Rate       string         `json:"rate"`       // units of preferred currency per 1 unit of `source.currency`
  ExchangeOn time.Date      `json:"exchangeOn"` // echoes request `exchangeOn` (after defaults)
}
```

### Error Conditions

The conditions below are **logical** errors checked after request validation succeeds. They are returned as explicit, stable error codes (implementations may attach a human-readable message).

#### Cross-cutting

- `exchange_rate_unavailable` — A conversion needs an exchange rate (`currency.convert`, or payment derivation in **live** mode) and the rate cannot be resolved from cache or online sources for the requested date.

Operations not listed below rely on struct validation only (for example `subscription.list`, `reminder.list`, `preferences.get`).

#### subscription.get

- `subscription_not_found` — No subscription exists with the given `id`.

#### subscription.getByName

- `subscription_not_found` — No subscription exists with the given `name` (case-insensitive exact match).

#### subscription.start

- `subscription_name_taken` — The request would create a new subscription, but another subscription already has this `name` (case-insensitive). Restarting an **inactive** subscription with the same name is allowed.
- `billing_period_not_consecutive` — Ending the current billing period and opening a new one would violate non-overlap or consecutivity (`previous.ends_on` must equal `new.starts_on`).
- `icon_fetch_failed` — `iconAuto` is true and the favicon could not be resolved for `name` as domain.
- `exchange_rate_unavailable` — `costAuto` is true and conversion cannot be resolved for the billing currency, preferred currency, and derived `exchangeOn`.

#### subscription.edit

- `subscription_not_found` — No subscription exists with the given `id`.
- `subscription_name_taken` — `name` is changed to a value that another subscription already has (case-insensitive).
- `icon_fetch_failed` — `iconAuto` is true and the favicon could not be resolved for the effective subscription name.
- `exchange_rate_unavailable` — `costAuto` is true and conversion cannot be resolved for the billing currency, preferred currency, and derived `exchangeOn`.

#### subscription.cancel

- `subscription_not_found` — No subscription exists with the given `id`.
- `subscription_inactive` — The subscription has no billing period that is still active (already fully ended).
- `invalid_duration` — `duration` is not a multiple of the latest billing period's `frequency`.
- `already_canceled` — The latest billing period already has an `ends_on` on or before the end date implied by `duration` (no change).

#### subscription.resume

- `subscription_not_found` — No subscription exists with the given `id`.
- `subscription_inactive` — The subscription is not active (latest billing period has already ended).
- `subscription_not_cancelled` — The latest billing period has no `ends_on` (already ongoing) or has already ended.

#### subscription.delete

- `subscription_not_found` — No subscription exists with the given `id`.

#### billingPeriod.add

- `subscription_not_found` — No subscription exists with the given `subscriptionId`.
- `billing_period_overlap` — The new period overlaps an existing billing period on the same subscription.
- `billing_period_not_consecutive` — The new period does not abut existing periods where required (gaps or crossing boundaries that violate application invariants).
- `exchange_rate_unavailable` — `costAuto` is true and conversion cannot be resolved for the billing currency, preferred currency, and derived `exchangeOn`.

#### billingPeriod.edit

- `billing_period_not_found` — No billing period exists with the given `id`.
- `invalid_period` — After applying changes, `endOn` is not strictly after `startOn` (when `endOn` is set).
- `billing_period_overlap` — The updated period overlaps another period on the same subscription.
- `billing_period_not_consecutive` — Updated dates break consecutivity with adjacent periods.
- `cannot_clear_end_on` — `endOn` is cleared on a billing period that is not the latest for its subscription.
- `exchange_rate_unavailable` — `costAuto` is true and conversion cannot be resolved for the billing currency, preferred currency, and derived `exchangeOn`.

#### billingPeriod.delete

- `billing_period_not_found` — No billing period exists with the given `id`.
- `last_billing_period` — Deleting this period would leave the subscription with no billing periods.

#### payment.list / payment.total

- `exchange_rate_unavailable` — Preferences use **live** exchange rates and a payment in the result set needs a rate that cannot be resolved for its date and billing currency.

`payment.total` uses the same payment derivation and date window as `payment.list`; errors are identical.

#### reminder.add

- `subscription_not_found` — No subscription exists with the given `subscriptionId`.

#### reminder.snooze

- `reminder_not_found` — No reminder exists with the given `id`.

#### reminder.clear

- `reminder_not_found` — No reminder exists with the given `id`.

#### preferences.set

- `exchange_rate_unavailable` — `currency` or `currencyConversionMode` changes require repopulating rates or recalculating stored costs and conversion fails—the whole update is rolled back.

#### icon.fetch

- `icon_fetch_failed` — The icon could not be obtained: favicon fetch failed for the given URL or domain; or (when `fromSubscriptionId` is set) no subscription exists with that ID, or the subscription’s stored `icon` is empty. These cases are indistinguishable by error code by design.

#### currency.convert

- `exchange_rate_unavailable` — No rate can be resolved for `currency`, the preferred currency, and `exchangeOn` (after defaulting `exchangeOn` to today).

## Shared types

### Database Records

```go
type SubscriptionRecord struct {
  ID    string   `json:"id"`
  Name  string   `json:"name"`
  Group string   `json:"group"`
  Tags  []string `json:"tags"`
  Notes string   `json:"notes"`
  Icon  string   `json:"icon"`
  URL   string   `json:"url"`
}

type BillingPeriodRecord struct {
  ID                    string    `json:"id"`
  SubscriptionID        string    `json:"subscriptionId"`
  StartOn               time.Date `json:"startOn"`
  EndOn                *time.Date `json:"endOn"`
  Frequency             int       `json:"frequency"`
  BillingCurrency       string    `json:"billingCurrency"`
  BillingCost           string    `json:"billingCost"`
  Cost                  string    `json:"cost"`
}

type TagRecord struct {
  ID             string `json:"id"`
  SubscriptionID string `json:"subscriptionId"`
  Tag            string `json:"tag"`
}

type ReminderAction string
const (
  ReminderActionCancel ReminderAction = "cancel"
  ReminderActionRenew  ReminderAction = "renew"
  ReminderActionCustom ReminderAction = "custom"
)

type ReminderRecord struct {
  ID             string         `json:"id"`
  SubscriptionID string         `json:"subscriptionId"`
  Action         ReminderAction `json:"action"`
  CustomNote     string         `json:"customNote"`
  TriggerOn      time.Date      `json:"triggerOn"`
  IsTriggered    bool           `json:"isTriggered"`
}

type CurrencyConversionMode string
const (
  CurrencyConversionModeApproximate CurrencyConversionMode = "approximate"
  CurrencyConversionModeLive        CurrencyConversionMode = "live"
)

type PreferencesRecord struct {
  Currency               string                 `json:"currency"`
  CurrencyConversionMode CurrencyConversionMode `json:"currencyConversionMode"`
}
```

### Domain Models

```go

type SubscriptionStatus string
const (
  SubscriptionStatusEnded     SubscriptionStatus = "ended"
  SubscriptionStatusOngoing   SubscriptionStatus = "ongoing"
  SubscriptionStatusScheduled SubscriptionStatus = "scheduled"
)

type SubscriptionTermStatus string
const (
  SubscriptionTermStatusOpenEnded SubscriptionTermStatus = "openended"
  SubscriptionTermStatusFixed     SubscriptionTermStatus = "fixed"
)

type Subscription struct {
  SubscriptionRecord

  // Derived fields
  Status                       SubscriptionStatus `json:"status"`
  SubscriptionTermStatus       SubscriptionTermStatus `json:"subscriptionTermStatus"`
  StartOn                      time.Date          `json:"startOn"`
  EndOn                        *time.Date         `json:"endOn"`
  Span                         int                `json:"span"`
  Duration                     int                `json:"duration"`
  Cost                         string             `json:"cost"`
  Frequency                    int                `json:"frequency"`
  NextPaymentOn                *time.Date         `json:"nextPaymentOn"`
  MonthlyCost                  string             `json:"monthlyCost"`
  YearlyCost                   string             `json:"yearlyCost"`
  SpanCost                     string             `json:"spanCost"`
  DurationCost                 string             `json:"durationCost"`
  RemainingCost                string             `json:"remainingCost"`
  LifetimeStartOn              time.Date          `json:"lifetimeStartOn"`
  LifetimeEndOn                *time.Date         `json:"lifetimeEndOn"`
  LifetimeNextPaymentOn        *time.Date         `json:"lifetimeNextPaymentOn"`
  LifetimeSpan                 int                `json:"lifetimeSpan"`
  LifetimeDuration             int                `json:"lifetimeDuration"`
  LifetimeSpanCost             string             `json:"lifetimeSpanCost"`
  LifetimeDurationCost         string             `json:"lifetimeDurationCost"`
  LifetimeRemainingCost        string             `json:"lifetimeRemainingCost"`
  BillingCurrency              string             `json:"billingCurrency"`
  BillingCost                  string             `json:"billingCost"`
  MonthlyBillingCost           string             `json:"monthlyBillingCost"`
  YearlyBillingCost            string             `json:"yearlyBillingCost"`
  SpanBillingCost              string             `json:"spanBillingCost"`
  DurationBillingCost          string             `json:"durationBillingCost"`
  RemainingBillingCost         string             `json:"remainingBillingCost"`
  LifetimeBillingCurrency      *string            `json:"lifetimeBillingCurrency"`
  LifetimeSpanBillingCost      *string            `json:"lifetimeSpanBillingCost"`
  LifetimeDurationBillingCost  *string            `json:"lifetimeDurationBillingCost"`
  LifetimeRemainingBillingCost *string            `json:"lifetimeRemainingBillingCost"`

  // Associated records
  Reminders      []Reminder      `json:"reminders"`
  BillingPeriods []BillingPeriod `json:"billingPeriods"`

  // Derived associations
  ConsecutiveBillingPeriods []BillingPeriod `json:"consecutiveBillingPeriods"`
  ClosestConsecutiveBillingPeriod BillingPeriod `json:"closestConsecutiveBillingPeriod"`
}

type BillingPeriod struct {
  BillingPeriodRecord

  Status               SubscriptionStatus `json:"status"`
  SubscriptionTermStatus SubscriptionTermStatus `json:"subscriptionTermStatus"`
  SpanPaymentCount     int                `json:"spanPaymentCount"`
  DurationPaymentCount int                `json:"durationPaymentCount"`
  Span                 int                `json:"span"`
  Duration             int                `json:"duration"`
  NextChargeCost       string             `json:"nextChargeCost"`
  MonthlyCost          string             `json:"monthlyCost"`
  YearlyCost           string             `json:"yearlyCost"`
  SpanCost             string             `json:"spanCost"`
  DurationCost         string             `json:"durationCost"`
  RemainingCost        string             `json:"remainingCost"`
  MonthlyBillingCost   string             `json:"monthlyBillingCost"`
  YearlyBillingCost    string             `json:"yearlyBillingCost"`
  SpanBillingCost      string             `json:"spanBillingCost"`
  DurationBillingCost  string             `json:"durationBillingCost"`
  RemainingBillingCost string             `json:"remainingBillingCost"`
}

type Reminder struct {
  ReminderRecord

  Subscription SubscriptionRecord `json:"subscription"`
}

type Payment struct {
  Date             time.Date `json:"date"`
  SubscriptionID   string    `json:"subscriptionId"`
  SubscriptionName string    `json:"subscriptionName"`
  BillingPeriodID  string    `json:"billingPeriodId"`
  BillingCurrency  string    `json:"billingCurrency"`
  BillingCost      string    `json:"billingCost"`
  Cost             string    `json:"cost"` // in the database default currency, converted according to the currency conversion mode
}
```

### Sorting

```go
type SortDirection string
const (
  SortDirectionAsc  SortDirection = "asc"
  SortDirectionDesc SortDirection = "desc"
)

type SubscriptionSort struct {
  Field     SubscriptionSortFields `json:"field,omitempty" validate:"omitempty,oneof=id name subscriptionTermStatus startOn endOn span duration cost frequency monthlyCost yearlyCost spanCost durationCost remainingCost lifetimeStartOn lifetimeEndOn lifetimeNextPaymentOn lifetimeSpan lifetimeDuration lifetimeSpanCost lifetimeDurationCost lifetimeRemainingCost billingCurrency billingCost monthlyBillingCost yearlyBillingCost spanBillingCost durationBillingCost remainingBillingCost lifetimeBillingCurrency lifetimeSpanBillingCost lifetimeDurationBillingCost lifetimeRemainingBillingCost"`
  Direction SortDirection          `json:"dir,omitempty"   validate:"omitempty,oneof=asc desc"`
}

type ReminderSort struct {
  Field     ReminderSortFields `json:"field,omitempty" validate:"omitempty,oneof=name triggerOn"`
  Direction SortDirection      `json:"dir,omitempty"   validate:"omitempty,oneof=asc desc"`
}
```

#### Sort fields

```go
type SubscriptionSortFields string
const (
  SubscriptionSortFieldID                           SubscriptionSortFields = "id"
  SubscriptionSortFieldName                         SubscriptionSortFields = "name"
  SubscriptionSortFieldGroup                        SubscriptionSortFields = "group"
  SubscriptionSortFieldSubscriptionTermStatus       SubscriptionSortFields = "subscriptionTermStatus"
  SubscriptionSortFieldStartOn                      SubscriptionSortFields = "startOn"
  SubscriptionSortFieldEndOn                        SubscriptionSortFields = "endOn"
  SubscriptionSortFieldSpan                         SubscriptionSortFields = "span"
  SubscriptionSortFieldDuration                     SubscriptionSortFields = "duration"
  SubscriptionSortFieldCost                         SubscriptionSortFields = "cost"
  SubscriptionSortFieldFrequency                    SubscriptionSortFields = "frequency"
- SubscriptionSortFieldNextPaymentOn                SubscriptionSortFields = "nextPaymentOn"
  SubscriptionSortFieldMonthlyCost                  SubscriptionSortFields = "monthlyCost"
  SubscriptionSortFieldYearlyCost                   SubscriptionSortFields = "yearlyCost"
  SubscriptionSortFieldSpanCost                     SubscriptionSortFields = "spanCost"
  SubscriptionSortFieldDurationCost                 SubscriptionSortFields = "durationCost"
  SubscriptionSortFieldRemainingCost                SubscriptionSortFields = "remainingCost"
  SubscriptionSortFieldLifetimeStartOn              SubscriptionSortFields = "lifetimeStartOn"
  SubscriptionSortFieldLifetimeEndOn                SubscriptionSortFields = "lifetimeEndOn"
  SubscriptionSortFieldLifetimeNextPaymentOn        SubscriptionSortFields = "lifetimeNextPaymentOn"
  SubscriptionSortFieldLifetimeSpan                 SubscriptionSortFields = "lifetimeSpan"
  SubscriptionSortFieldLifetimeDuration             SubscriptionSortFields = "lifetimeDuration"
  SubscriptionSortFieldLifetimeSpanCost             SubscriptionSortFields = "lifetimeSpanCost"
  SubscriptionSortFieldLifetimeDurationCost         SubscriptionSortFields = "lifetimeDurationCost"
  SubscriptionSortFieldLifetimeRemainingCost        SubscriptionSortFields = "lifetimeRemainingCost"
  SubscriptionSortFieldBillingCurrency              SubscriptionSortFields = "billingCurrency"
  SubscriptionSortFieldBillingCost                  SubscriptionSortFields = "billingCost"
  SubscriptionSortFieldMonthlyBillingCost           SubscriptionSortFields = "monthlyBillingCost"
  SubscriptionSortFieldYearlyBillingCost            SubscriptionSortFields = "yearlyBillingCost"
  SubscriptionSortFieldSpanBillingCost              SubscriptionSortFields = "spanBillingCost"
  SubscriptionSortFieldDurationBillingCost          SubscriptionSortFields = "durationBillingCost"
  SubscriptionSortFieldRemainingBillingCost         SubscriptionSortFields = "remainingBillingCost"
  SubscriptionSortFieldLifetimeBillingCurrency      SubscriptionSortFields = "lifetimeBillingCurrency"
  SubscriptionSortFieldLifetimeSpanBillingCost      SubscriptionSortFields = "lifetimeSpanBillingCost"
  SubscriptionSortFieldLifetimeDurationBillingCost  SubscriptionSortFields = "lifetimeDurationBillingCost"
  SubscriptionSortFieldLifetimeRemainingBillingCost SubscriptionSortFields = "lifetimeRemainingBillingCost"
)

type ReminderSortFields string
const (
  ReminderSortFieldName      ReminderSortFields = "name"
  ReminderSortFieldTriggerOn ReminderSortFields = "triggerOn"
)
```

### Filters

```go
// validate:"date_range" on DateRangeFilter
type DateRangeFilter struct {
  Gte time.Date `json:"gte,omitempty"`
  Lt  time.Date `json:"lt,omitempty"`
}

// validate:"int_range" on IntegerRangeFilter
type IntegerRangeFilter struct {
  Gte int `json:"gte,omitempty"`
  Lte int `json:"lte,omitempty"`
}

// validate:"decimal_range" on DecimalRangeFilter
type DecimalRangeFilter struct {
  Gte string `json:"gte,omitempty" validate:"omitempty,decimal"`
  Lte string `json:"lte,omitempty" validate:"omitempty,decimal"`
}

type StringFilter struct {
  Value    string `json:"value,omitempty"`
  Operator string `json:"operator,omitempty" validate:"omitempty,oneof=contains equals"` // default equals before validate
}

type ArrayFilter struct {
  Values   []string `json:"values,omitempty"`
  Operator string   `json:"operator,omitempty" validate:"omitempty,oneof=all any"` // default any when omitted
}

// Subset of SubscriptionFilters that store.Subscriptions may apply in SQL.
// Zero value means no extra predicates. See internal/core/store/AGENTS.md.
type SubscriptionFiltersDb struct {
  Name   StringFilter         `json:"name"   validate:"dive"`
  Group  string               `json:"group"`
  Tags   ArrayFilter          `json:"tags"   validate:"dive"`
  Status []SubscriptionStatus `json:"status" validate:"dive,omitempty"`
}

type SubscriptionFilters struct {
  SubscriptionFiltersDb

  SubscriptionTermStatus       SubscriptionTermStatus `json:"subscriptionTermStatus,omitempty" validate:"omitempty,oneof=openended fixed"`
  StartOn                      DateRangeFilter    `json:"startOn"                      validate:"dive"`
  EndOn                        DateRangeFilter    `json:"endOn"                        validate:"dive"`
  Span                         IntegerRangeFilter `json:"span"                         validate:"dive"`
  Duration                     IntegerRangeFilter `json:"duration"                     validate:"dive"`
  Cost                         DecimalRangeFilter `json:"cost"                         validate:"dive"`
  Frequency                    int                `json:"frequency"                    validate:"omitempty,gt=0"`
  MonthlyCost                  DecimalRangeFilter `json:"monthlyCost"                  validate:"dive"`
  YearlyCost                   DecimalRangeFilter `json:"yearlyCost"                   validate:"dive"`
  SpanCost                     DecimalRangeFilter `json:"spanCost"                     validate:"dive"`
  DurationCost                 DecimalRangeFilter `json:"durationCost"                 validate:"dive"`
  RemainingCost                DecimalRangeFilter `json:"remainingCost"                validate:"dive"`
  LifetimeStartOn              DateRangeFilter    `json:"lifetimeStartOn"              validate:"dive"`
  LifetimeEndOn                DateRangeFilter    `json:"lifetimeEndOn"                validate:"dive"`
  LifetimeSpan                 IntegerRangeFilter `json:"lifetimeSpan"                 validate:"dive"`
  LifetimeDuration             IntegerRangeFilter `json:"lifetimeDuration"             validate:"dive"`
  LifetimeSpanCost             DecimalRangeFilter `json:"lifetimeSpanCost"             validate:"dive"`
  LifetimeDurationCost         DecimalRangeFilter `json:"lifetimeDurationCost"         validate:"dive"`
  LifetimeRemainingCost        DecimalRangeFilter `json:"lifetimeRemainingCost"        validate:"dive"`
  BillingCurrency              string             `json:"billingCurrency"              validate:"omitempty,currency"`
  BillingCost                  DecimalRangeFilter `json:"billingCost"                  validate:"dive"`
  MonthlyBillingCost           DecimalRangeFilter `json:"monthlyBillingCost"           validate:"dive"`
  YearlyBillingCost            DecimalRangeFilter `json:"yearlyBillingCost"            validate:"dive"`
  SpanBillingCost              DecimalRangeFilter `json:"spanBillingCost"              validate:"dive"`
  DurationBillingCost          DecimalRangeFilter `json:"durationBillingCost"          validate:"dive"`
  RemainingBillingCost         DecimalRangeFilter `json:"remainingBillingCost"         validate:"dive"`
  LifetimeBillingCurrency      string             `json:"lifetimeBillingCurrency"      validate:"omitempty,currency"`
  LifetimeSpanBillingCost      DecimalRangeFilter `json:"lifetimeSpanBillingCost"      validate:"dive"`
  LifetimeDurationBillingCost  DecimalRangeFilter `json:"lifetimeDurationBillingCost"  validate:"dive"`
  LifetimeRemainingBillingCost DecimalRangeFilter `json:"lifetimeRemainingBillingCost" validate:"dive"`
}

type PaymentFilters struct {
  Name        StringFilter       `json:"name"        validate:"dive"`
  Group       string             `json:"group"`
  Tags        ArrayFilter        `json:"tags"        validate:"dive"`
  Status      SubscriptionStatus `json:"status"      validate:"omitempty,oneof=ended ongoing scheduled"`
  PaymentDate DateRangeFilter    `json:"paymentDate" validate:"dive"` // defaults applied before validate; see Notes
}

type ReminderFilters struct {
  Name      StringFilter    `json:"name"      validate:"dive"`
  Action    ReminderAction  `json:"action"    validate:"omitempty,oneof=cancel renew custom"`
  TriggerOn DateRangeFilter `json:"triggerOn" validate:"dive"`
}
```

## Notes

- Initializing the database is not an operation, since it is not exposed through the API or MCP.
- Reminder notifications (OS notifications when a reminder is due, and tracking whether they have been sent) are a feature of the desktop application. They are not exposed through the api, mcp or cmd.
- Not all subscription fields are sortable; only the fields listed in `SubscriptionSortFields` are. For example, subscriptions can't be sorted by group or tags.
- `SubscriptionFilters.subscriptionTermStatus` omitted or empty matches every subscription. `openended` and `fixed` keep only subscriptions with that `SubscriptionTermStatus`. The empty string is the zero value and is not a status.
- By design, `subscription.start` and `subscription.cancel` can't take an explicit end date. They accept a `duration` instead: an integer number of months, a multiple of the billing frequency, starting from zero. The end date is derived from it:
  - `subscription.start`: counted from `startOn`. A duration of 0 starts an ongoing billing period (no end date).
  - `subscription.cancel`: counted from the next charge date. A duration of 0 ends the subscription on the next charge date, i.e. at the end of the period already paid for.
- `StringFilter.Operator` defaults to `equals` when omitted (apply before `validator.Struct`).
- `PaymentFilters.PaymentDate` is required. If it is not provided, it is equivalent to `[null, current_date)`, i.e. all payments before the current date. If only `gte` is provided, `lt` defaults to the current date.
- Reports on "specific subscriptions" can only be retrieved if the set of subscriptions can be selected with the currently defined filters (name, group, tags, status). Arbitrary sets of subscriptions that share no group or tag can't be reported on in a single request.
