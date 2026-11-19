package domain

import "cloud.google.com/go/civil"

type ExchangeRateRecord struct {
	BaseCurrency  string     `json:"baseCurrency"`
	QuoteCurrency string     `json:"quoteCurrency"`
	Rate          string     `json:"rate"`
	ExchangeOn    civil.Date `json:"exchangeOn"`
}
