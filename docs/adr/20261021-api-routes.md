# API Routes

REST mapping for operations in [Operations](./20261018-operations.md). JSON routes are served under the `/api` prefix (see [project brief](../prd/20261013-project-brief.md)); bearer auth applies to all `/api/*` routes except unrelated infrastructure such as `GET /api/health`.

## Conventions

- Paths use **kebab-case** plural resource names. Path parameters are subscription, billing-period, and reminder **IDs** (UUID strings).
- **Request and response bodies** match the operation structs in [Operations](./20261018-operations.md), except:
  - Fields carried in the path (`id`, `subscriptionId`) are omitted from the body.
  - `subscription.get`, `subscription.delete`, `subscription.cancel`, `subscription.resume`, `billingPeriod.delete`, `reminder.clear`, and `reminder.snooze` omit the path `id` from the body; remaining fields stay in the body.
  - `preferences.get` has no body.
- **List and aggregate reads** (`subscription.list`, `payment.list`, `payment.total`, `reminder.list`) use `GET`. Nested `filter` and `sort` objects are passed as URL-encoded JSON in query parameters `filter` and `sort` (when applicable), using the same shapes as the operation request structs.
- **Lookup helpers** (`icon.fetch`, `currency.convert`) use `GET` with query parameters matching the operation request fields (after applying documented defaults in the handler before validation).

## subscriptions

- `subscription.list`: `GET` `/subscriptions?filter[...]=...&sort[...]=...`
- `subscription.getByName`: `GET` `/subscriptions/name/{name}`
- `subscription.get`: `GET` `/subscriptions/{id}`
- `subscription.start`: `POST` `/subscriptions`
- `subscription.edit`: `PATCH` `/subscriptions/{id}`
- `subscription.cancel`: `POST` `/subscriptions/{id}/cancel`
- `subscription.resume`: `POST` `/subscriptions/{id}/resume`
- `subscription.delete`: `DELETE` `/subscriptions/{id}`

## billing periods

- `billingPeriod.add`: `POST` `/subscriptions/{subscriptionId}/billing-periods`
- `billingPeriod.edit`: `PATCH` `/billing-periods/{id}`
- `billingPeriod.delete`: `DELETE` `/billing-periods/{id}`

## payments

- `payment.list`: `GET` `/payments?filter[...]=...&sort[...]=...`
- `payment.total`: `GET` `/payments/total?filter[...]=...`

## reminders

- `reminder.list`: `GET` `/reminders?filter[...]=...&sort[...]=...`
- `reminder.add`: `POST` `/subscriptions/{subscriptionId}/reminders`
- `reminder.snooze`: `POST` `/reminders/{id}/snooze`
- `reminder.clear`: `DELETE` `/reminders/{id}`

## preferences

- `preferences.get`: `GET` `/preferences`
- `preferences.set`: `PATCH` `/preferences`

## utilities

- `icon.fetch`: `POST` `/icons/fetch` (body: exactly one of `fromImageUrl`, `fromDomain`, or `fromSubscriptionId`)
- `currency.convert`: `POST` `/currency/convert`
