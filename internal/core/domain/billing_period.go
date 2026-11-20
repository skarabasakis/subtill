package domain

import "cloud.google.com/go/civil"

type BillingPeriodRecord struct {
	ID              string      `json:"id"`
	SubscriptionID  string      `json:"subscriptionId"`
	StartOn         civil.Date  `json:"startOn"`
	EndOn           *civil.Date `json:"endOn"`
	Frequency       int         `json:"frequency"`
	BillingCurrency string      `json:"billingCurrency"`
	BillingCost     string      `json:"billingCost"`
}
