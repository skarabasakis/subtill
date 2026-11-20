# Handling of unavailable exchange rates

## Context

- We convert costs to the preferred currency using rates from Frankfurter (see [Currency & Exchange Rate Services](./20261113-currency-service.md)).
- Sometimes Frankfurter simply has no rate for a date:
  - the date is before the first rate it publishes for the currency pair;
  - it answers 404 for the pair or date;
  - its response contradicts its own coverage, for example by leaving out a date it says it covers.
- Until now, one missing rate failed the whole request with `exchange_rate_unavailable`. A single bad date in a payment list hid every other payment.
- Missing rates should be rare.
- This ADR doesn't cover failures to reach Frankfurter (network errors, rate limits). Those still fail the request.

## Decision

- When a rate is missing, the cost that needs it is `null`. The request still succeeds.
- Costs in the billing currency don't need a rate, so they are never `null` for this reason.
- **Propagation:** Any calculation that uses a `null` cost gives `null`. We never skip it or count it as 0. For example, if one payment's cost is `null`, the `payment.total` cost is `null` too.
- **Filtering** on a converted cost leaves out rows where that cost is `null`.
- **Sorting** on a converted cost puts `null` values last, in both directions.
- We don't tell the client why a cost is `null`: responses have no warnings or list of missing rates.
- `currency.convert` is the exception. It still fails with `exchange_rate_unavailable`, because a `null` result would make the call useless. It is now the only operation that returns this code.

## Consequences

- A missing rate never fails a request or causes a panic.
- Code that does arithmetic on converted costs has to handle `null`.
- Clients can't tell a `null` caused by a missing rate from other `null` values, and filters can drop rows without saying so. We accept this for now and can add reporting later.
- Converted cost fields are nullable in every adapter (API, MCP, CLI, desktop), and the OpenAPI docs need regenerating.
- When some dates are missing, the Exchange Rate Service returns the rates it did find along with the error.
