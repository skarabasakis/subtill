package domain

import "cloud.google.com/go/civil"

type ExchangeRateRecord struct {
	Currency   string     `json:"currency"`
	Rate       string     `json:"rate"`
	ExchangeOn civil.Date `json:"exchangeOn"`
}
