---
name: Repository SQL filters
overview: Give SubscriptionRepository.Count and ListAll an optional SubscriptionFiltersDb so name, group, tags, and status are filtered in one SQL statement. The ADR records the chosen SQL only; the alternatives stay in this plan.
todos:
  - id: signatures
    content: Add filter and currentDate arguments to Count and ListAll and document the dynamic WHERE
    status: completed
  - id: filter-section
    content: Add the Filtering subscriptions section with per-field SQL and the status aggregate
    status: completed
  - id: conventions-coverage
    content: Update the Types convention and the coverage table ListAll calls
    status: completed
isProject: false
---

# SubscriptionFiltersDb on Count and ListAll

Edit only [`docs/adr/20261022-repositories.md`](docs/adr/20261022-repositories.md).

## Signatures

```go
func Count(filter *SubscriptionFiltersDb, currentDate time.Date) (count int, err error)
func ListAll(filter *SubscriptionFiltersDb, currentDate time.Date) (subscriptions []SubscriptionRecord, err error)
```

- A `nil` or zero filter adds no predicates.
- `currentDate` is bound as `:current_date` only when `filter.Status` is non-empty. The service passes local today, the same date it uses to derive `Subscription.status`.
- `WithAssociationsA(ListAll(...))` is unchanged.

## Recommended SQL

Go builds one statement and appends a predicate only for each set field. Predicates are joined with `AND`. Lists are bound as JSON arrays and read with `json_each`, so the statement shape does not depend on list length. Example with every field set (tags `any`):

```sql
SELECT s.id, s.name, s."group", s.notes, s.icon, s.url   -- Count: SELECT COUNT(*)
FROM subscriptions s
WHERE s.name = :name                                      -- or contains, below
  AND s."group" = :group
  AND EXISTS (SELECT 1 FROM tags t
               WHERE t.subscription_id = s.id
                 AND t.tag IN (SELECT value FROM json_each(:tags)))
  AND (SELECT CASE
                WHEN MAX(b.start_on <= :current_date
                         AND (b.end_on IS NULL OR b.end_on > :current_date)) THEN 'ongoing'
                WHEN MAX(b.start_on > :current_date) THEN 'scheduled'
                ELSE 'ended'
              END
         FROM billing_periods b
        WHERE b.subscription_id = s.id) IN (SELECT value FROM json_each(:status))
ORDER BY s.id DESC                                        -- ListAll only
```

Per-field rules:

- **Name equals:** `s.name = :name`. The column is `COLLATE NOCASE`, so its unique index serves the lookup.
- **Name contains:** `instr(lower(s.name), lower(:name)) > 0`. Unlike `LIKE '%'||:name||'%'`, it does not require escaping `%` and `_`. Neither form can use an index.
- **Group:** `s."group" = :group`. Omitted when empty.
- **Tags `any`:** the correlated `EXISTS` above. It probes the `UNIQUE (subscription_id, tag)` index, and `t.tag` comparisons use `NOCASE`.
- **Tags `all`:** `NOT EXISTS (SELECT 1 FROM json_each(:tags) j WHERE NOT EXISTS (SELECT 1 FROM tags t WHERE t.subscription_id = s.id AND t.tag = j.value))`. This passes only when no requested tag is missing. Its shape is fixed for any number of tags, and duplicates in the input do not change the result.
- **Status:** one correlated aggregate over the subscription's billing periods, using the `billing_periods (subscription_id)` index. `ongoing` wins over `scheduled`, and `scheduled` wins over `ended`, as in [subscription derived fields](docs/adr/20261018-subscription-derived-fields.md). With no periods, `MAX` is `NULL`, which gives `ended`. That matches the no-periods corner case. Several requested statuses are combined with OR through `IN`.

## Alternatives considered (plan only; not recorded in the ADR)

- **Fixed statement with optional guards**, for example `(:name IS NULL OR s.name = :name)`. It needs only one prepared statement, but `OR` guards stop SQLite from using the name index, and every subquery runs even when its filter is unset. This is the slowest option.
- **Join `tags` and `billing_periods`, then `GROUP BY s.id ... HAVING`** with conditional aggregates. Rows multiply as tags times periods, so it needs `DISTINCT` counts and does more work per subscription.
- **CTE that computes every status with `GROUP BY subscription_id`, joined with `LEFT JOIN` and `COALESCE(status, 'ended')`.** When status is the only filter, which is the usual case, both forms read every billing period once. The CTE trades one index seek per subscription for materializing the grouped result and building an automatic index for the join. Performance is about even, with the CTE slightly ahead when there are many subscriptions. When other filters have already narrowed the subscriptions, the CTE still scans all billing periods. Kept the correlated aggregate.
- **`INTERSECT` of ID sets**, one `SELECT` per filter. It composes easily, but each set is fully materialized before the intersection, and the second step loses indexed probes back into `subscriptions`.
- **Tags `all` as one `EXISTS` per tag, or as `COUNT(*) = :n`.** Per-tag `EXISTS` changes the statement shape with the number of tags. `COUNT(*) = :n` requires Go to deduplicate tags case-insensitively first.
- **Status as two `EXISTS` (ongoing, then scheduled) in a `CASE`.** It is equivalent to the aggregate and can stop early, but it reads the period index twice for subscriptions that are not ongoing.

The recommended form appends only the predicates that are set, uses correlated probes on existing indexes, and keeps one statement per call. The data set is small, single-user, and local, so plan stability and fewer scans matter more than micro-tuning.

## Other ADR edits

- **Types** convention: `SubscriptionFiltersDb` (name, group, tags, status) is applied in SQL by `Count` and `ListAll`. Every other `SubscriptionFilters` field, and all sort, is applied in the service after derivation.
- Add a short **Filtering subscriptions** section with the per-field rules above. Do not include the alternatives.
- Coverage table:
  - `subscription.list` uses `ListAll(&filter.SubscriptionFiltersDb, today)`.
  - `payment.list` and `payment.total` use `ListAll` with a `SubscriptionFiltersDb` built from the payment filter's name, group, tags, and status.
  - `reminder.list` and `preferences.set` use `ListAll(nil, zero)`.
