package store_test

import (
	"path/filepath"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/jmoiron/sqlx"
	"github.com/skarabasakis/subtill/internal/core/database"
	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/skarabasakis/subtill/internal/core/store"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

// newDB returns a pool on a new database file, closed at the end of the test.
func newDB(t *testing.T) *sqlx.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "subtill.db")
	require.NoError(t, database.Create(path, domain.Preferences{Currency: ptr("EUR")}))
	db, closeDB, err := database.Open(path, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, closeDB()) })
	return db
}

func date(s string) civil.Date {
	d, err := civil.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func insertSubscription(t *testing.T, q sqlx.Ext, id, name string, tags ...string) domain.SubscriptionRecord {
	t.Helper()
	record := domain.SubscriptionRecord{ID: id, Name: name}
	require.NoError(t, store.Subscriptions{}.Insert(q, record))
	for _, tag := range tags {
		insertTag(t, q, id, tag)
	}
	return record
}

func insertTag(t *testing.T, q sqlx.Ext, subscriptionID, tag string) {
	t.Helper()
	require.NoError(t, store.Tags{}.Insert(q, domain.TagRecord{
		ID: subscriptionID + "-" + tag, SubscriptionID: subscriptionID, Tag: tag,
	}))
}

// insertBillingPeriod inserts a monthly period from startOn to endOn; an
// empty endOn makes it open-ended.
func insertBillingPeriod(t *testing.T, q sqlx.Ext, id, subscriptionID, startOn, endOn string) domain.BillingPeriodRecord {
	t.Helper()
	record := billingPeriod(id, subscriptionID, startOn, endOn)
	require.NoError(t, store.BillingPeriods{}.Insert(q, record))
	return record
}

func billingPeriod(id, subscriptionID, startOn, endOn string) domain.BillingPeriodRecord {
	record := domain.BillingPeriodRecord{
		ID:              id,
		SubscriptionID:  subscriptionID,
		StartOn:         date(startOn),
		Frequency:       1,
		BillingCurrency: "USD",
		BillingCost:     "10.00",
	}
	if endOn != "" {
		record.EndOn = ptr(date(endOn))
	}
	return record
}

func insertReminder(t *testing.T, q sqlx.Ext, id, subscriptionID string, action domain.ReminderAction, triggerOn string) domain.ReminderRecord {
	t.Helper()
	record := domain.ReminderRecord{
		ID: id, SubscriptionID: subscriptionID, Action: action, TriggerOn: date(triggerOn),
	}
	require.NoError(t, store.Reminders{}.Insert(q, record))
	return record
}

func subscriptionNames(records []domain.SubscriptionRecord) []string {
	names := []string{}
	for _, r := range records {
		names = append(names, r.Name)
	}
	return names
}
