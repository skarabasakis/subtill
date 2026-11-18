# Currency & Exchange Rate Services

Decimal arithmetic is used for all currency conversions, powered by govalues/decimal. In sqlite, decimals are represented as TEXT.
Currency names are represented as ISO 4217 currency codes (three uppercase letters).

Note that neither the Currency service nor the Exchange Rate service validate the currency codes. It is okay for the services to return nil or a not found error if the currency code is not supported. All currency codes in the database are assumed to be valid, and all user inputs are validated at the controller/operation layer before they are passed to the services and persisted to the database.

## Currencies List

The https://pkg.go.dev/golang.org/x/text/currency package internally queries a recent version of the CLDR database. By default, `currency.Query()` iterates over the units that are legal tender and currently in use in some region, so no further date filtering is needed. Units are repeated once per region and are deduplicated by the map key. `x/text/currency` exposes no currency names, so each name comes from the Frankfurter `/v2/currencies` response. That response also lists crypto assets (`"type": "crypto"`), which are skipped.

The service supports the intersection of the active CLDR currencies and the Frankfurter currencies, built as follows:

```go
type CurrencyInfo struct {
    Name      string
    Symbol    string
    Scale     int
    Increment int
}

frankfurterCurrencies, err := frankfurter.New().FetchCurrencies(ctx) // []frankfurter.Currency
if err != nil {
    return nil, err
}
names := map[string]string{}
for _, c := range frankfurterCurrencies {
    if c.Type == "currency" {
        names[c.ISOCode] = c.Name
    }
}

printer := message.NewPrinter(language.English)
supportedCurrencies := map[string]CurrencyInfo{}
for it := currency.Query(); it.Next(); {
    unit := it.Unit()
    name, ok := names[unit.String()]
    if !ok {
        continue
    }
    scale, increment := currency.Standard.Rounding(unit)
    supportedCurrencies[unit.String()] = CurrencyInfo{
        Name:      name,
        Symbol:    printer.Sprint(currency.Symbol(unit)),
        Scale:     scale,
        Increment: increment,
    }
}
```

`Symbol` is the standard CLDR symbol for English and is only a display hint; it is not locale-aware.

### Generation

The list is generated ahead of time, committed to the repository and embedded into the binary, so building, testing and running the app never need the network.

- **Generator**: a dedicated `main` package at `internal/core/currency/internal/gencurrencies`, holding the build function above. It is not part of the currency service's API: the service only reads the generated list.
- **Output**: `internal/core/currency/currencies.json`, a JSON object keyed by ISO code with `CurrencyInfo` values, sorted by key so regenerating with unchanged inputs yields no diff.
- **Embedding**: the currency package loads the file with `//go:embed currencies.json` and decodes it once at package initialization.
- **Trigger**: `//go:generate go run ./internal/gencurrencies` in the currency package, wrapped by `wails3 task generate:currencies` (same pattern as `generate:openapi`). It is run manually when the inputs change (for example a `golang.org/x/text` upgrade or a Frankfurter currency addition) and the result is committed. It is not a dependency of `build` or `test`.

## Api Client: Frankfurter (v2)

The api client lives in the `internal/core/clients/frankfurter` package. It makes HTTP requests to the Frankfurter API and decodes the responses into Go types. It knows nothing about the database or the preferred currency.

### Construction

```go
type Client struct {
    httpClient *http.Client
    baseURL    string
}

func New(opts ...Option) *Client
func WithHTTPClient(c *http.Client) Option
func WithBaseURL(u string) Option
```

- `baseURL` defaults to `https://api.frankfurter.dev`. It can be overridden for tests (`httptest.Server`) or a self-hosted instance.
- `httpClient` defaults to an `*http.Client` with a 10s `Timeout`. Callers can also bound a request with the `ctx` deadline.

### Public methods

- `func (c *Client) FetchCurrencies(ctx context.Context) ([]Currency, error)`: `GET {baseURL}/v2/currencies`. `Currency` mirrors the response items: `ISOCode` (`iso_code`), `Name` (`name`), `Symbol` (`symbol`), `Type` (`type`: `currency` or `crypto`).
- `func (c *Client) FetchRate(ctx context.Context, baseCurrency, quoteCurrency string) (decimal.Decimal, error)`: `GET {baseURL}/v2/rate/{base}/{quote}`: `GET {baseURL}/v2/rate/{base}/{quote}` (Today's rate)
- `func (c *Client) FetchRateOn(ctx context.Context, baseCurrency, quoteCurrency string, date civil.Date) (decimal.Decimal, error)`: `GET {baseURL}/v2/rate/{base}/{quote}?date={date}`
- `func (c *Client) FetchRates(ctx context.Context, baseCurrency, quoteCurrency string, from, to civil.Date) (map[civil.Date]decimal.Decimal, error)`: `GET {baseURL}/v2/rates?base={base}&quotes={quote}&from={from}&to={to}`. The map contains only the dates Frankfurter published a rate for (business days).

URLs are built with `url.JoinPath` for path segments and `url.Values` for query strings, never with `fmt.Sprintf`. Dates are formatted with `civil.Date.String()` (`YYYY-MM-DD`). Rates are decoded straight into `decimal.Decimal` and never pass through `float64`.

`baseCurrency` and `quoteCurrency` must differ, because Frankfurter rejects a pair of identical currencies. The client returns `ErrSameCurrency` without making a request. Callers are expected to handle that case before calling the client (see `ExchangeRates` step 0).

### Errors

- `ErrSameCurrency`: base and quote are the same currency.
- `ErrRateLimited`: the response status is 429. The client does not retry. The caller decides whether to fall back to cached rates.
- `ErrUnexpectedStatus`: any other non-2xx response. It is wrapped with the status code and the request path.
- Transport, timeout and context errors, and JSON decoding errors, are returned wrapped, so `errors.Is(err, context.DeadlineExceeded)` still works.

### Testing

Client tests run against an `httptest.Server` injected with `WithBaseURL`, with response fixtures under `testdata/`. Services that depend on the client use a small interface they define, so their tests can use a fake.

## Exchange Rate Service

This service wraps the Frankfurter API and the exchange_rates database repository to provide a simple interface for fetching and caching exchange rates. It is oriented around bulk exchange rate retrieval, i.e. fetching the exchange rate for a bunch of dates at once.

Note: In practice, the service is never called in approximate mode, only in live mode.

### Implied quote currency

Rows in `exchange_rates` have no quote currency column. The quote currency of every row is the preferred currency, `preferences.currency`. The service API follows the table: callers pass only the currency to convert from, never a quote currency.

- **Where it comes from.** The service reads the preferred currency with `store.Preferences.Get(q)`, on the same `q` it uses for `exchange_rates`. Inside a transaction, it therefore sees the preferred currency written earlier in that transaction (for example by `preferences.set`) and never mixes rates from two quote currencies.
- **Rate direction.** A rate is the number of units of the preferred currency per 1 unit of `currency`, as in the [subscription derived fields ADR](20261018-subscription-derived-fields.md) and `CurrencyConvertResponse.Rate`. It is fetched from Frankfurter with `base=currency` and `quote=preferred`, and a converted amount is `amount × rate`.
- **Invalidation invariant.** Changing `preferences.currency` makes every cached rate wrong. So the operation that changes it (`preferences.set`) must call `store.ExchangeRates.DeleteAll` in the same transaction as the preferences upsert, before recalculating any stored cost. The Exchange Rate Service does not detect a change of preferred currency and never clears the cache itself.

### Public methods

- func ExchangeRate(q sqlx.DB, currency string, date civil.Date) (decimal.Decimal, error) -- convenience wrapper for `ExchangeRates(q, currency, []civil.Date{date})`

- func ExchangeRates(q sqlx.DB, currency string, dates []civil.Date) (map[civil.Date]decimal.Decimal, error)

Before step 0, the service reads the preferred currency from `preferences` on `q`.

0. If currency is the same as the preferred currency, it returns an exchange rate of 1 for all dates, without reading the cache or calling the Frankfurter API.
1. Look up rates for the given date or dates in the exchange_rates table. Collect the dates of missing rates
2. Fetch missing rates from frankfurter.FetchRates. Collect the fetched rates for persistence using the response dates as keys. Skip this step if dates has a length of 1, and go right to the next step.
3. Identify any remaining missing rates not returned by the frankfurter api. Resolve each of these as follows:
  - Split them in two groups: past dates and today/future dates
  - For each past date, retrieve the rate returned by frankfurter.fetchRateOn. Collect the fetched rates for persistence using the request dates as keys. If frankfurter returns 404, throw an error.
  - For today/future dates, retrieve the rate of today's date with frankfurter.fetchRate, and use that rate for all dates in the group. Do not persist these rates. If frankfurter returns 404, throw an error.
4. Open a transaction and upsert the collected rates to the exchange_rates table using the request dates as keys. Inside the transaction ensure that the preferred currency is the same as the one used to fetch the rates, otherwise do not persist any rates and throw a concurrency-related error.
5. Return the collected rates

Mock the frankfurter api client for testing.

Private methods:

- func missingRates(q sqlx.Ext, currency string, dates []civil.Date) ([]civil.Date, error)

## Currency Service

The currency.Amount type comes from the https://pkg.go.dev/golang.org/x/text/currency package and is a wrapper around the decimal.Decimal type.

Public methods:

- func Convert(base currency.Amount, quoteCurrency string, exchangeRate decimal.Decimal) (quote currency.Amount)

1. It checks if base.Currency is the same as quoteCurrency. If so, it returns the base unchanged.
2. Multiplies the base amount by the exchange rate.
3. Rounds the result to the scale of the quote currency.
