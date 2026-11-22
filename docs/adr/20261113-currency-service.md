# Currency & Exchange Rate Services

Decimal arithmetic is used for all currency conversions, powered by govalues/decimal. In sqlite, decimals are represented as TEXT.
Currency names are represented as ISO 4217 currency codes (three uppercase letters).

Note that neither the Currency service nor the Exchange Rate service validate the currency codes. It is okay for the services to return nil or a not found error if the currency code is not supported. All currency codes in the database are assumed to be valid, and all user inputs are validated at the controller/operation layer before they are passed to the services and persisted to the database (see [Validation](#validation)).

## Currencies List

The https://pkg.go.dev/golang.org/x/text/currency package embeds a snapshot of the CLDR database, identified by `currency.CLDRVersion` (see [CLDR version](#cldr-version)). By default, `currency.Query()` iterates over the units that are legal tender in some region as of that snapshot, so no further date filtering is needed. Units are repeated once per region and are deduplicated by the map key. `x/text/currency` exposes no currency names, so each name comes from the Frankfurter `/v2/currencies` response. That response also lists crypto assets (`"type": "crypto"`), which are skipped.

The service supports the intersection of:
- the active CLDR currencies
- the Frankfurter currencies

The list is generated ahead of time, committed to the repository and embedded into the binary, so building, testing and running the app never need the network.

The list is add-only: once a currency is supported, it stays in the list, even after it is retired in CLDR or Frankfurter. Billing periods are stored with their `billing_currency` and may span years, so a currency retired after it was added must stay convertible for the dates it was in use. Frankfurter keeps serving historical rates for retired currencies (for example, USD/HRK is covered until 2023-01-14, when Croatia adopted the euro). Removing a code would make stored data fail validation and conversion. Enforcing this in the generator (merging the previous list into the new one) is out of scope for now: the generator rebuilds the list from its inputs, and a regenerated list that drops a code must be caught in review and the code restored by hand (see [Open questions](#open-questions)).

### CLDR version

`golang.org/x/text` has not regenerated its CLDR tables in years: `currency.CLDRVersion` is `32` (2017). Upgrading `x/text` does not change it. As a result, currencies introduced or redenominated since then are unknown to `x/text` (`currency.ParseISO` rejects them) and are left out of the intersection, even though Frankfurter publishes rates for them. Examples include VES (Venezuela), SLE (Sierra Leone), MRU (Mauritania) and ZWG (Zimbabwe). Conversely, CLDR 32 still lists some currencies that have since been retired, such as VEF, SLL, MRO and HRK. Frankfurter does not list them, so they are dropped from the intersection as well.

This trade-off is accepted. `x/text` is the only maintained Go source of CLDR scale and symbol data. The missing currencies are rare in subscription billing, and the intersection keeps every supported code backed by both a rate provider and CLDR formatting data. A small committed override file that adds the scale and symbol of codes unknown to CLDR 32 may be considered later; it is out of scope for now.

The list is built as follows:

```go
type CurrencyInfo struct {
    Name   string `json:"name"`
    Symbol string `json:"symbol"`
    Scale  int    `json:"scale"`
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
    scale, _ := currency.Standard.Rounding(unit)
    supportedCurrencies[unit.String()] = CurrencyInfo{
        Name:      name,
        Symbol:    printer.Sprint(currency.Symbol(unit)),
        Scale:     scale,
    }
}
```

`Symbol` is the standard CLDR symbol for English and is only a display hint; it is not locale-aware. Standard English symbols tell currencies apart (`$` for USD, `CA$` for CAD, `A$` for AUD, `CN¥` for CNY); no two active currencies share one in CLDR 32. `currency.NarrowSymbol` is not used, because it gives `$` for every dollar and `¥` for both JPY and CNY.

`Scale` comes from the `Standard` rounding kind, which applies to non-cash amounts such as subscription billing. The rounding increment that `Rounding` also returns is dropped: it is 1 for every currency under `Standard`. Cash increments (for example 0.05 for CHF) only exist under `currency.Cash`, which the app does not use.

### Rounding

Every amount the app stores or derives in a currency is rounded to the `Scale` of that currency in this list, half to even (bankers' rounding, the default of govalues/decimal). This list is the only source of a currency's number of decimals: the app never uses the ISO 4217 exponent or a default scale, because every code it handles is in the list (see [Validation](#validation)). Stored `billing_cost` values are rounded on save (see [operations](./20261018-operations.md#notes)); derived amounts are rounded by their own formulas (see [subscription derived fields](./20261018-subscription-derived-fields.md#shared-functions)) or by `Convert` (see [Currency Service](#currency-service)).

### Generation

- **Generator**: a dedicated `main` package at `internal/core/currency/internal/gencurrencies`, holding the build function above. It is not part of the currency service's API: the service only reads the generated list. It needs network access to reach Frankfurter. Its result also depends on the CLDR data embedded in `golang.org/x/text`, identified by `currency.CLDRVersion`.
- **Output**: `internal/core/currency/currencies.json`, a JSON object that records the generator inputs next to the list:

  ```json
  {
    "generatedFrom": {
      "cldrVersion": "32",
      "frankfurterRetrievedOn": "2026-11-19"
    },
    "currencies": {
      "EUR": { "name": "Euro", "symbol": "€", "scale": 2 }
    }
  }
  ```

  `cldrVersion` is the `currency.CLDRVersion` constant. The currency tables in `x/text` are only regenerated from CLDR, so an `x/text` upgrade that keeps the same `CLDRVersion` is not expected to change the list. `frankfurterRetrievedOn` is the date (`civil.Date`, `YYYY-MM-DD`) the generator fetched `/v2/currencies`, because the Frankfurter response carries no version of its own. `currencies` is keyed by ISO code and sorted by key, so regenerating with unchanged inputs only changes `frankfurterRetrievedOn`.
- **Embedding**: the currency package embeds the file with `//go:embed currencies.json` and decodes it lazily, on first use, with `sync.OnceValue`. A decoding error panics, since the file is generated and committed; a unit test decodes it so a bad file fails CI rather than at runtime. Only `currencies` is used at runtime; `generatedFrom` is provenance for reviewers.
- **Trigger**: `//go:generate go run ./internal/gencurrencies` in the currency package, wrapped by `wails3 task generate:currencies` (same pattern as `generate:openapi`). It is run manually when the inputs change (for example a `golang.org/x/text` upgrade or a Frankfurter currency addition), the diff is checked for removed codes (the list is add-only), and the result is committed. It is not a dependency of `build` or `test`.

### Public methods

- `func Currencies() map[string]CurrencyInfo`: the supported currencies keyed by ISO code. Callers must not mutate the map; return a copy if this becomes a risk.
- `func CurrencyByCode(code string) (CurrencyInfo, bool)`: the info for one code; `false` when the code is not in the list.

### Validation

Request fields that hold a currency code use a custom `currency` validation tag, registered on the shared `go-playground/validator` instance. It accepts a code only when `CurrencyByCode` finds it, and is used instead of the validator's built-in `iso4217` tag. The built-in ISO 4217 table accepts codes the app cannot convert (for example XAU, XTS, or codes Frankfurter does not publish), so an unsupported code would only fail later, deep in a conversion, instead of as a validation error.

Because the list is add-only, a code that passed validation once keeps passing it, so stored `billing_currency` and `preferences.currency` values stay valid in later requests.

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
- `func (c *Client) FetchRates(ctx context.Context, baseCurrency, quoteCurrency string, from, to civil.Date) (map[civil.Date]decimal.Decimal, error)`: `GET {baseURL}/v2/rates?base={base}&quotes={quote}&from={from}&to={to}`. The map contains only the dates Frankfurter published a rate for. Which dates those are depends on the providers: some publish on business days only, while the default blend may also cover weekends. When `from` itself has no rate, Frankfurter also returns the latest published date before it. A range entirely after the last published date returns an empty map.
- `func (c *Client) FetchCoverage(ctx context.Context, baseCurrency, quoteCurrency string) (Coverage, error)`: `GET {baseURL}/v2/coverage?base={base}&quotes={quote}`. `Coverage` mirrors the response: `StartDate` (`start_date`) and `EndDate` (`end_date`), the first and last dates a rate is published for the pair.

URLs are built with `url.JoinPath` for path segments and `url.Values` for query strings, never with `fmt.Sprintf`. Dates are formatted with `civil.Date.String()` (`YYYY-MM-DD`). Rates are decoded straight into `decimal.Decimal` and never pass through `float64`.

`baseCurrency` and `quoteCurrency` must differ, because Frankfurter rejects a pair of identical currencies. The client returns `ErrSameCurrency` without making a request. Callers are expected to handle that case before calling the client (see `ExchangeRates` step 1).

### Errors

- `ErrSameCurrency`: base and quote are the same currency.
- `ErrNotFound`: the response status is 404 (the pair is not published).
- `ErrRateLimited`: the response status is 429. The client does not retry.
- `ErrUnexpectedStatus`: any other non-2xx response. It is wrapped with the status code and the request path.
- Transport, timeout and context errors, and JSON decoding errors, are returned wrapped, so `errors.Is(err, context.DeadlineExceeded)` still works.

### Testing

Client tests run against an `httptest.Server` injected with `WithBaseURL`, with response fixtures under `testdata/`. Services that depend on the client use a small interface they define, so their tests can use a fake.

`FetchRates` has a test with a recorded Frankfurter response where `from` has no published rate (a weekend). It checks that the map includes the latest published date before `from`. `ExchangeRates` relies on that date to resolve a published date without its own rate (step 5), so if Frankfurter drops it, the test fails instead of those dates silently becoming unavailable. Re-record the fixture when the client is changed against a new Frankfurter version.

## Exchange Rate Service

This service wraps the Frankfurter API to provide a simple interface for fetching exchange rates. Reading and persisting them in the `exchange_rates` cache are each optional: the caller passes the cache functions to use (see [Cache](#cache)). It is oriented around bulk exchange rate retrieval, i.e. fetching the exchange rate for a bunch of dates at once.

### Rates are illustrative

Converted amounts are illustrative, not fit for accounting. Frankfurter cannot tell whether a published rate is final: providers publish at different times of day, may skip days that are holidays for them but not for others, and may publish with a delay of several days. So the app never tries to decide whether a rate is final:

- The cache stores whatever Frankfurter returns, and a cached rate is never refreshed. A rate that a provider revises or fills in later stays stale in the cache.

Refreshing cached rates is out of scope for now.

The service is also intended to be called outside of existing transactions, i.e. after a list of billing periods has been retrieved and the respective payments have been calculated.

In practice, every payment and subscription derivation (`payment.list`, `payment.total`, and the derived cost fields of `subscription.list`, `subscription.get`, `subscription.getByName` and the `subscription.start` response) calls it with `DatabaseCache`, so the cache is both read and updated. `subscription.start` calls it after its write is committed. `currency.convert` calls `ExchangeRate` with `NoCache`, so it neither reads nor writes `exchange_rates`, and converts with the Currency Service. It maps `ErrRateUnavailable` to `exchange_rate_unavailable` and `ErrRateProviderUnavailable` to `exchange_rate_provider_unavailable`. The other write operations return no derived costs, so they never call the Frankfurter client or the service.

### Quote currency

Every row in `exchange_rates` stores its currency pair: `base_currency`, the currency converted from, and `quote_currency`, the currency converted to. Rows are keyed by `(base_currency, quote_currency, exchange_on)`, so rates quoted in different currencies never collide and a read only ever returns rates in the quote currency it asked for. A `CHECK (base_currency <> quote_currency)` constraint rejects rows with identical currencies. It is a backstop: the service never produces such rows, because it returns the rate between identical currencies before using the cache (step 1). The service API takes both currencies, `baseCurrency` and `quoteCurrency`, and never reads `preferences`.

- **Where it comes from.** The caller passes the preferred currency as `quoteCurrency`, taken from a single `preferences` read. Because `preferences` is read only once, the quote currency of the returned rates and the currency the caller labels the converted amounts with always agree, even if another process changes `preferences.currency` during the call. The service passes `quoteCurrency` to the cache functions, which use it for `store.ExchangeRates.List` and for every record they upsert.
- **Rate direction.** A rate is the number of units of `quoteCurrency` per 1 unit of `baseCurrency`, as in the [subscription derived fields ADR](20261018-subscription-derived-fields.md) and `CurrencyConvertResponse.Rate`. It is fetched from Frankfurter with `base=baseCurrency` and `quote=quoteCurrency`, and a converted amount is `amount × rate`.
- **No write guard.** If `preferences.currency` changes while a call runs, the call still returns and caches rates quoted in the `quoteCurrency` it was given. They are correct for that pair; they are simply not used until that currency is preferred again.
- **No invalidation.** Changing `preferences.currency` leaves cached rates in place. Reads in the new preferred currency don't see rates quoted in the old one, and switching back reuses them. Rates in a newly preferred currency are fetched and cached lazily by later conversions.

### Construction

The service is a struct whose dependencies are injected, so tests can replace the Frankfurter client with a fake:

```go
type RateFetcher interface {
    FetchCoverage(ctx context.Context, baseCurrency, quoteCurrency string) (frankfurter.Coverage, error)
    FetchRates(ctx context.Context, baseCurrency, quoteCurrency string, from, to civil.Date) (map[civil.Date]decimal.Decimal, error)
}

type ExchangeRateService struct {
    client RateFetcher
}

func NewExchangeRateService(client RateFetcher) *ExchangeRateService
```

- `RateFetcher` is defined by the service, not by the `frankfurter` package, and lists only the methods the service calls. `*frankfurter.Client` satisfies it.
- The service has no clock. The cutoff between published and unpublished dates is the coverage `end_date` reported by Frankfurter (step 3), not the local date, which would be wrong in timezones ahead of the providers and before they publish.
- The struct holds no database handle, no state between calls, and does not read `preferences`: the quote currency is an argument (see [Quote currency](#quote-currency)). The service itself never touches the database. The caller passes the [cache functions](#cache) to each call. They are bound to the database, given as `q sqlx.Ext` like every `store` method, so the caller decides whether the cache is used, and whether on the pool or inside a transaction.

### Public methods

The caller passes the [cache functions](#cache) to use, grouped in an `ExchangeRateCache`. A step the caller doesn't want is given a no-op function rather than `nil`, so the caller can read the cache, update it, do both, or neither, and `ExchangeRates` runs the same steps in every case, with no branch on which cache functions were passed:

```go
type CacheReader func(baseCurrency, quoteCurrency string, dates []civil.Date) (map[civil.Date]decimal.Decimal, error)
type CacheWriter func(baseCurrency, quoteCurrency string, rates map[civil.Date]decimal.Decimal) error

type ExchangeRateCache struct {
    Read  CacheReader
    Write CacheWriter
}
```

Both fields are required. A zero `ExchangeRateCache`, or one with a `nil` field, is a programming error and panics when the missing function is called; it never means "no cache". Callers use one of the ready-made values from [Cache](#cache) (`DatabaseCache`, `NoCache`) and only build the struct by hand for other combinations, with `NoCacheReader` or `NoCacheWriter` for the step they skip.

Typical call site, with the cache read and updated:

```go
rates, err := svc.ExchangeRates(ctx, baseCurrency, quoteCurrency, dates, currency.DatabaseCache(db))
```

The service keeps control of when each cache function runs. It alone knows which rates are published and may be cached (step 6), and it updates the cache even after a Frankfurter call fails (step 7), so neither rule leaks to the caller.

- `func (s *ExchangeRateService) ExchangeRate(ctx context.Context, baseCurrency, quoteCurrency string, date civil.Date, cache ExchangeRateCache) (decimal.Decimal, error)`: convenience wrapper for `s.ExchangeRates(ctx, baseCurrency, quoteCurrency, []civil.Date{date}, cache)`.

- `func (s *ExchangeRateService) ExchangeRates(ctx context.Context, baseCurrency, quoteCurrency string, dates []civil.Date, cache ExchangeRateCache) (map[civil.Date]decimal.Decimal, error)`

`ctx` bounds the Frankfurter requests; a cancelled or expired `ctx` fails the call like any other transient Frankfurter error (step 8). `quoteCurrency` is the preferred currency as read by the caller (see [Quote currency](#quote-currency)). The method never touches the database except through `cache`.

1. Deduplicate `dates`. If `baseCurrency` equals `quoteCurrency`, return a rate of 1 for every date, without calling `cache` or Frankfurter. No row is ever cached with identical base and quote currencies, so the cache functions need no special case for them. Return an empty map when `dates` is empty.
2. Look up the cached rates with `cache.Read(baseCurrency, quoteCurrency, dates)`, and compute the missing dates from the requested dates and the cached rates with `missingDates`. With `NoCacheReader`, every date is missing. If there are none, return the cached rates without calling Frankfurter. A failed read returns its error without calling Frankfurter.
3. Call `FetchCoverage(baseCurrency, quoteCurrency)` and keep its `start_date` and `end_date`, the first and last dates a rate is published for. They split the missing dates into three groups:
   - **uncovered dates** (before `start_date`): unavailable, without asking Frankfurter for them;
   - **published dates** (from `start_date` to `end_date`);
   - **unpublished dates** (after `end_date`).
4. If every missing date is uncovered, skip this step. Otherwise call `FetchRates(baseCurrency, quoteCurrency, from, to)` once for the published and unpublished dates, with `from` = the earlier of `end_date` and the earliest missing date that isn't uncovered, and `to` = the latest missing date. `from` is never before `start_date`, so the request never asks for a range that Frankfurter has no data for, which it might reject instead of answering partially. Including `end_date` in the range guarantees a rate for it in the response, even when every missing date is unpublished. If the response has no rate for `end_date`, Frankfurter broke its own coverage; treat every unpublished date as unavailable. The response covers every published date in the range, so missing dates far apart (for example, one in 2015 and one in 2026) fetch every rate in between. This trade-off is accepted: the range is neither split into clusters nor capped. A decade of daily rates for one pair is only a few thousand entries, one request is simpler than several and uses less of the rate limit, and when the caller's cache writes, the requested published dates are cached, so later calls for the same dates make no request.
5. Resolve each missing date from the response:
   - **Published date**: the rate of the latest date on or before it in the response. That is the date's own rate when one was published, otherwise the rate of the latest date published before it (a weekend or holiday for its providers). Frankfurter includes that earlier date in the response even when it falls before `from`. A published date with no rate on or before it in the response is unavailable: since it is on or after `start_date`, this only happens if Frankfurter broke its own coverage.
   - **Unpublished date**: the rate for `end_date`.
6. Collect the rates of the published dates for caching, keyed by the requested date, so that a weekend or holiday is found in the cache next time instead of being missing again. Unpublished dates are never cached: their rate will be published later. Published dates are cached even though their rate may still be revised or filled in by a late provider (see [Rates are illustrative](#rates-are-illustrative)).
7. Cache the collected rates with `cache.Write(baseCurrency, quoteCurrency, rates)`, even when there are none: an empty write is a no-op for every writer. With `NoCacheWriter`, nothing is written. This step runs even when a Frankfurter call in step 3 or 4 failed, so a retry needs fewer requests. It caches whatever was resolved before the failure, which may be nothing.
8. Return, in order of precedence:
   - If the write failed, its error, joined with `errors.Join` to the Frankfurter error described below if any, with no rates. A database error fails the call like any other `store` error, while `ErrRateUnavailable` and `ErrRateProviderUnavailable` stay reachable with `errors.Is`.
   - If a Frankfurter call failed transiently (see Errors), an error wrapping `ErrRateProviderUnavailable` and the cause, with no rates. A rate limit never falls back to the cached rate of another date.
   - Otherwise, if any date is unavailable (uncovered, `ErrNotFound`, or a response that breaks the reported coverage), the cached and resolved rates of the other dates together with an error wrapping `ErrRateUnavailable`, listing the unavailable dates. Unavailable dates are absent from the map.
   - Otherwise, the cached and resolved rates for all requested dates.

Errors (see [Handling of unavailable exchange rates](./20261120-unavailable-exchange-rates.md)):

- `ErrRateUnavailable`: the provider has no rate for at least one requested date. It is returned with the rates of the other dates. Subscription and payment derivations turn each unavailable date into a null converted cost; `currency.convert` maps it to `exchange_rate_unavailable`. A cause such as `ErrNotFound` stays reachable with `errors.Is`.
- `ErrRateProviderUnavailable`: Frankfurter could not be reached or failed transiently: transport errors and timeouts, a cancelled or expired `ctx`, `ErrRateLimited`, `ErrUnexpectedStatus`, or a JSON decoding error. Operations map it to `exchange_rate_provider_unavailable`. The cause stays reachable with `errors.Is`.

`ExchangeRate` returns the rate with no error, or a zero rate with the error of either class. Callers check the error and never use the zero rate.

### Cache

Reading and updating the `exchange_rates` cache are each optional. They are public package functions, and the only ones that take a database. They can be called directly, or bound to a database and passed to `ExchangeRates` in an `ExchangeRateCache`, which decides when to call them. `q` is normally the pool, because conversions run outside transactions (see above).

Database access:

- `func CachedExchangeRates(q sqlx.Ext, baseCurrency, quoteCurrency string, dates []civil.Date) (map[civil.Date]decimal.Decimal, error)`: the cached rates with `store.ExchangeRates.List(q, baseCurrency, quoteCurrency, dates)`, a single query for all dates, keyed by `ExchangeOn`. Dates without a cached rate are absent from the map. Rows whose `Rate` fails to parse return an error.
- `func CacheExchangeRates(q sqlx.Ext, baseCurrency, quoteCurrency string, rates map[civil.Date]decimal.Decimal) error`: upserts `rates` with `store.ExchangeRates.Upsert(q, records)`, each record with `baseCurrency` as its base currency and `quoteCurrency` as its quote currency. An empty `rates` returns without a query, so callers never need to check. It writes whatever it is given: choosing which rates may be cached is up to the caller (`ExchangeRates` step 6). The write is safe outside any transaction, because rows are keyed by quote currency: if the preferred currency has changed since the caller read it, the rates are cached under the old one, where they are still correct (see [Quote currency](#quote-currency)).

Cache functions, for building an `ExchangeRateCache`:

- `func DatabaseCacheReader(q sqlx.Ext) CacheReader` and `func DatabaseCacheWriter(q sqlx.Ext) CacheWriter`: bind `CachedExchangeRates` and `CacheExchangeRates` to `q`.
- `func NoCacheReader(baseCurrency, quoteCurrency string, dates []civil.Date) (map[civil.Date]decimal.Decimal, error)`: returns an empty map and no error, so every date is missing and fetched.
- `func NoCacheWriter(baseCurrency, quoteCurrency string, rates map[civil.Date]decimal.Decimal) error`: writes nothing and returns no error.

`NoCacheReader` and `NoCacheWriter` are plain functions with the signatures of `CacheReader` and `CacheWriter`, so they are used as values without being called (null object). They replace `nil`, so `ExchangeRates` never branches on which cache functions it was given.

Ready-made caches:

- `func DatabaseCache(q sqlx.Ext) ExchangeRateCache`: reads and updates the cache, `{DatabaseCacheReader(q), DatabaseCacheWriter(q)}`. Used by every payment and subscription derivation.
- `var NoCache = ExchangeRateCache{NoCacheReader, NoCacheWriter}`: always fetches and never writes. Used by `currency.convert`.

Read-only and write-only caches have no ready-made value because nothing needs them yet. They are built by hand, as `ExchangeRateCache{DatabaseCacheReader(q), NoCacheWriter}` and `ExchangeRateCache{NoCacheReader, DatabaseCacheWriter(q)}`.

`CacheReader` and `CacheWriter` are function types rather than interfaces, so a test can pass a closure, and a single function such as `NoCacheReader` is usable without a wrapper type.

Private helpers:

- `func missingDates(requested []civil.Date, cached map[civil.Date]decimal.Decimal) []civil.Date`: the requested dates without a cached rate. It is pure: the cache read stays in `ExchangeRates`, the composed method, so the helper has a single responsibility and needs no database.

### Testing

`ExchangeRates` is tested with a fake `RateFetcher` that reports a fixed coverage `end_date`, and with fake cache functions (closures that record their calls and return canned rates and errors), so it needs no database. Besides the happy path, the tests cover:

- **Cache read**: Frankfurter is asked only for the dates missing from `cache.Read`, and the cached rates are merged into the result; no Frankfurter call is made when every date is cached; a failed read is returned without calling Frankfurter.
- **Published vs unpublished split**, with `end_date` pinned:
  - a date on or before `end_date` with its own rate is passed to `cache.Write` under that date;
  - a date on or before `end_date` without its own rate (weekend or holiday) gets the latest earlier rate, passed to `cache.Write` under the requested date;
  - a date after `end_date` gets the rate for `end_date` and is not passed to `cache.Write`;
  - all missing dates are fetched with a single `FetchRates` call whose range includes `end_date`.
- **Errors**: a date before the coverage `start_date` yields `ErrRateUnavailable` listing that date, is excluded from the `FetchRates` range, and doesn't prevent the rates of the other dates from being returned and written; when every missing date is before `start_date`, `FetchRates` is not called; a response without a rate for `end_date` makes the unpublished dates unavailable; an `ErrNotFound` from `FetchCoverage` or `FetchRates` yields `ErrRateUnavailable` wrapping the cause; a transport error, timeout, cancelled `ctx`, `ErrRateLimited` or `ErrUnexpectedStatus` from either call yields `ErrRateProviderUnavailable` wrapping the cause, even when some dates are also unavailable, and the rates resolved before the failure are still written; a failed write is joined to the Frankfurter error.
- **Same currency**: when `baseCurrency` equals `quoteCurrency`, every rate is 1, and neither the fake fetcher nor the cache functions are called.

`CachedExchangeRates` and `CacheExchangeRates` run against a real database like the `store` tests. They cover:

- rates written by `CacheExchangeRates` are returned by `CachedExchangeRates` for the same pair, and dates without a cached rate are absent;
- an empty `rates` writes nothing and returns no error;
- **quote currency isolation**: rates cached for one `quoteCurrency` are not returned for another quote currency with the same base currency and dates.

`NoCacheReader` returns an empty map and `NoCacheWriter` returns no error; neither needs a database. `ExchangeRates` is not tested once per ready-made cache: it has no branch on them, so the fake cache functions cover it.

## Currency Service

Public methods:

- func Convert(baseAmount decimal.Decimal, baseCurrency string, quoteCurrency string, exchangeRate decimal.Decimal) (quoteAmount decimal.Decimal, error)

1. If `baseCurrency` or `quoteCurrency` is not found by `CurrencyByCode`, return an error wrapping `ErrUnsupportedCurrency` that names the code, and a zero amount. Callers check the error and never use the zero amount.
2. If `baseCurrency` is the same as `quoteCurrency`, the result is `baseAmount` and `exchangeRate` is ignored.
3. Otherwise, the result is `baseAmount` multiplied by `exchangeRate`.
4. In both cases, it rounds the result to the scale of `quoteCurrency`, as described in [Rounding](#rounding).

Rounding also applies when the currencies match, so every converted amount has the scale of its currency whether or not a rate was used. For a stored `billing_cost`, which is already rounded on save, this changes nothing: the converted amount equals `billing_cost`. It matters only for `currency.convert`, whose `amount` is not rounded on input. For example, `9.999` EUR to EUR gives `10.00`.

Validation rejects unsupported codes before they reach the database (see [Validation](#validation)), so `ErrUnsupportedCurrency` means a programming error or a stored code that was removed from the list (see [Currencies List](#currencies-list)). Operations return it as an internal error.

## Open questions

- **Automatic add-only enforcement.** The add-only rule of the [Currencies List](#currencies-list) depends on reviewers. Enforcing it automatically is out of scope for now. Options include having the generator merge the previous `currencies.json` into the new one, or a CI check that fails when a code disappears. Until then, a regenerated list that drops a code would make stored `billing_currency` and `preferences.currency` values fail validation, and `Convert` would fail with `ErrUnsupportedCurrency` when converting from or to them.

- **Uncached today and future rates.** Every subscription read converts at today (`nextChargeCost`, and `cost` of a scheduled billing period), and projected payments convert at future dates. Future dates are always after the coverage `end_date`. Today is after it until the providers publish its rate, and on days they don't publish (weekends and holidays for the providers of the pair). Such dates are unpublished and never cached (step 6). As a result, a read that involves a billing currency other than the preferred currency calls `FetchCoverage` and `FetchRates` whenever it needs an unpublished date, even when every published rate it needs is cached. Projected payments always need one; `nextChargeCost` needs one only while today is unpublished. Such reads depend on the network, use up the rate limit, and fail with `exchange_rate_provider_unavailable` when offline. Options include caching the rate resolved for an unpublished date for a limited time, or falling back to the latest cached rate for the pair. Neither is decided yet.
