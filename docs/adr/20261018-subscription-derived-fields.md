# Subscription Derived fields

This document provides definitions for fields that are derived from other fields in the database.

## Application-layer invariants

- Each subscription has at least one billing period.
- Billing periods for a subscription strictly end on a charge date. The month interval between the start_on and the end_on of a billing period is a multiple of the billing period's frequency.
- Billing periods for a subscription are strictly non-overlapping. Corollaries:
  - Only the most recent billing period can be open-ended
- Consecutive billing periods satisfy `billing_period[n].ends_on = billing_period[n+1].starts_on`.
- **Payments** are derived from billing periods and the ongoing exchange-rate preference; they are not stored.
- Payment dates (charge dates) of a billing period are its start_on and every later date less than end_on at a whole multiple of frequency months from start_on, on the MonthsInterval charge calendar. A nil end_on does not limit them. The most recent payment date is the latest of these on or before now.

## Symbols

- now: the current date

## Shared functions

- MonthsInterval(startOn, endOn): the length of the half-open interval [startOn, endOn) in whole months. A nil endOn is the day after now. The length is 0 when endOn <= startOn. Months are added on the charge calendar anchored at startOn: the same day of the month, clamped to the last day of a shorter month and clamping back afterwards (Jan 31 → Feb 28 → Mar 31). Otherwise the length is the smallest integer m >= 0 such that startOn plus m months is on or after endOn. A partial month rounds up to the next month. An exact number of months is unchanged.

- PaymentCount(startOn, endOn, frequency): ceil(MonthsInterval(startOn, endOn) / frequency).

- Decimal division in MonthlyCost and YearlyCost, and in the billing-currency fields that use the same formulas, rounds the field's final value half away from zero to the minor-unit scale of the currency that field is expressed in. The scale is the ISO 4217 exponent of that currency code (for example 2 for USD and EUR, 0 for JPY, 3 for KWD). A code with no ISO 4217 exponent uses scale 2. The factor 12 / frequency is not rounded before it multiplies the charge amount. The schema does not store this scale.

## BillingPeriod

- Status:
  - ended: end_on is not null and end_on <= now
  - ongoing: (start_on <= now) and (end_on is null or end_on > now)
  - scheduled: start_on > now
- OpenEnded: end_on is null
- SpanPaymentCount: PaymentCount(start_on, end_on, frequency).
- DurationPaymentCount: PaymentCount(start_on, duration_end_on, frequency), where duration_end_on is end_on if end_on is not null and end_on <= now, otherwise nil.
- Span: SpanPaymentCount × frequency.
- Duration: DurationPaymentCount × frequency.

In approximate currency conversion mode:
- cost: (static, from the database)
- nextChargeCost: cost
- MonthlyCost: the average monthly cost of the billing period, i.e. nextChargeCost / frequency
- YearlyCost: the average yearly cost of the billing period, i.e. nextChargeCost * (12 / frequency), where 12 / frequency is decimal division

- SpanCost: DurationCost + RemainingCost
- DurationCost: cost × DurationPaymentCount
- RemainingCost: nextChargeCost × (SpanPaymentCount - DurationPaymentCount). Corollary: zero for open-ended billing periods.

- MonthlyBillingCost, YearlyBillingCost, SpanBillingCost, DurationBillingCost, RemainingBillingCost: Defined same as the respective cost fields, but using the billing_cost instead of the cost field as the cost value, and expressed in the billing_currency instead of the default_currency.

In live currency conversion mode, a converted amount is billing_cost multiplied by the exchange rate for the chosen date. The rate is the `exchange_rates` value for that billing_currency and date: units of the default currency per 1 unit of billing_currency. When billing_currency equals the default currency, the converted amount is billing_cost and no rate is used.
- cost: (derived -- the database cost field is ignored) billing_cost converted at the most recent payment date, or at today if the billing period is scheduled.
- nextChargeCost: billing_cost converted at today.
- MonthlyCost: the average monthly cost of the billing period, i.e. nextChargeCost / frequency
- YearlyCost: the average yearly cost of the billing period, i.e. nextChargeCost * (12 / frequency), where 12 / frequency is decimal division

- SpanCost: DurationCost + RemainingCost
- DurationCost: the sum of billing_cost converted at each charge date on or before now. Charge dates after now are not included; they contribute to RemainingCost.
- RemainingCost: nextChargeCost × (SpanPaymentCount - DurationPaymentCount). Corollary: zero for open-ended billing periods.

### Consecutive billing period

A consecutive billing period is a maximal run of billing periods. Each billing period belongs to exactly one run. Adjacent periods (the end_on of the previous equals the start_on of the next) share a run when they have the same frequency and billing_currency. A run stops at a gap, a frequency change, or a billing_currency change.

latest billing period: the billing period with the latest start_on

The fields of a consecutive billing period are defined as follows:
- StartOn: the start_on of the first billing period in the consecutive billing period
- EndOn: the end_on of the last billing period in the consecutive billing period
- Status: Derived from the StartOn and the EndOn of the consecutive billing period, using the same logic as the BillingPeriod status. Corollary: if there is at least one ongoing billing period in the consecutive billing period, the status of the consecutive billing period is ongoing
- OpenEnded: Derived from the end_on of the consecutive billing period, using the same logic as the BillingPeriod OpenEnded.
- Span: Derived from the start_on and the end_on of the consecutive billing period, using the same logic as the BillingPeriod Span. Corollary: Assuming end_on is a charge date, the span is equal to the sum of the spans of the billing periods in the consecutive billing period.
- Duration: Derived from the start_on and the end_on of the consecutive billing period, using the same logic as the BillingPeriod Duration. Corollary: Assuming end_on is a charge date, the duration is equal to the sum of the durations of the billing periods in the consecutive billing period.
- Cost: the cost field of the latest billing period, as defined for the selected currency conversion mode.
- Frequency: the frequency of the latest billing period.
- MonthlyCost: The MonthlyCost of the latest among the billing periods in the consecutive billing period
- YearlyCost: The YearlyCost of the latest among the billing periods in the consecutive billing period

- SpanCost: Sum of the SpanCost of the billing periods in the consecutive billing period
- DurationCost: Sum of the DurationCost of the billing periods in the consecutive billing period
- RemainingCost: Sum of the RemainingCost of the billing periods in the consecutive billing period

- BillingCurrency: The common billing_currency of the billing periods in the consecutive billing period.
- BillingCost: the billing_cost of the latest billing period.
- MonthlyBillingCost, YearlyBillingCost: the MonthlyBillingCost and YearlyBillingCost of the latest billing period.
- SpanBillingCost, DurationBillingCost, RemainingBillingCost: the sum of that field over the billing periods in the run. Each period uses its own billing_cost in the approximate-mode formulas, expressed in the BillingCurrency, with no currency conversion. A price change inside the run is included: a monthly run priced at 14 and then 16 has SpanBillingCost = 14 × the months priced at 14 + 16 × the months priced at 16.

### Closest Consecutive Billing Period

We define the "closest consecutive billing period" as the latest consecutive billing period (i.e. sorted on start_on) of the subscription that is ongoing. If there is no ongoing consecutive billing period, the closest consecutive billing period is the latest consecutive billing period that is scheduled. If there is no scheduled consecutive billing period, the closest consecutive billing period is the latest consecutive billing period that is ended.

## Subscription

- Status: the status of the subscription's closest consecutive billing period. Corollaries:
  - if the subscription has at least one ongoing billing period, the status of the subscription is ongoing
  - if the subscription has at least one scheduled billing period and no ongoing billing period, the status of the subscription is scheduled
  - if the subscription has no ongoing or scheduled billing period, the status of the subscription is ended
- OpenEnded: the subscription's closest consecutive billing period is OpenEnded

- StartOn: StartOn of the subscription's closest consecutive billing period
- EndOn: EndOn of the subscription's closest consecutive billing period
- Span: Span of the subscription's closest consecutive billing period
- Duration: Duration of the subscription's closest consecutive billing period
- Cost: Cost of the closest consecutive billing period.
- Frequency: Frequency of the closest consecutive billing period.
- MonthlyCost: MonthlyCost of the closest consecutive billing period.
- YearlyCost: YearlyCost of the closest consecutive billing period.
- SpanCost: SpanCost of the closest consecutive billing period.
- DurationCost: DurationCost of the closest consecutive billing period.
- RemainingCost: RemainingCost of the closest consecutive billing period.

- LifetimeStartOn: the start_on of the subscription's earliest billing period
- LifetimeEndOn: the end_on of the subscription's latest billing period
- LifetimeSpan: the sum of the spans of all billing periods associated with the subscription
- LifetimeDuration: the sum of the durations of all billing periods associated with the subscription
- LifetimeSpanCost: Sum of the SpanCost of the billing periods in the subscription
- LifetimeDurationCost: Sum of the DurationCost of the billing periods in the subscription
- LifetimeRemainingCost: Sum of the RemainingCost of the billing periods in the subscription

- BillingCurrency: BillingCurrency of the closest consecutive billing period.
- BillingCost, MonthlyBillingCost, YearlyBillingCost, SpanBillingCost, DurationBillingCost, RemainingBillingCost: the field of that name on the closest consecutive billing period.

- LifetimeBillingCurrency: If all billing periods share the same billing_currency, it is the billing_currency of the billing periods in the subscription. Otherwise, it is nil.
- LifetimeSpanBillingCost, LifetimeDurationBillingCost, LifetimeRemainingBillingCost: if all billing periods share the same billing_currency, the sum of each billing period's SpanBillingCost, DurationBillingCost, or RemainingBillingCost. Each period substitutes its own billing_cost for cost in the approximate-mode formulas, with no currency conversion. Otherwise, they are nil.

## Corner case: No billing periods for a subscription

If a subscription has no billing periods (because the application-layer invariant is violated), the derived fields are defined as follows:
- Status: ended
- OpenEnded: false
- StartOn, EndOn, LifetimeStartOn, LifetimeEndOn: nil
- BillingCurrency, LifetimeBillingCurrency: nil
- LifetimeSpanBillingCost, LifetimeDurationBillingCost, LifetimeRemainingBillingCost: nil
- Frequency: 1
- MonthsInterval: 0
- PaymentCount: 0
- Span, Duration, LifetimeSpan, LifetimeDuration: 0
- Cost, MonthlyCost, YearlyCost, SpanCost, DurationCost, RemainingCost: 0
- LifetimeSpanCost, LifetimeDurationCost, LifetimeRemainingCost: 0
- BillingCost, MonthlyBillingCost, YearlyBillingCost, SpanBillingCost, DurationBillingCost, RemainingBillingCost: 0

This is merely to avoid a panic when calculating the derived fields.
