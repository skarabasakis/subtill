package store_test

import (
	"testing"

	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/skarabasakis/subtill/internal/core/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingPeriods(t *testing.T) {
	billingPeriods := store.BillingPeriods{}

	t.Run("reads back all columns when period has end date", func(t *testing.T) {
		db := newDB(t)
		insertSubscription(t, db, "01", "Netflix")
		record := insertBillingPeriod(t, db, "b1", "01", "2026-01-01", "2026-07-01")

		got, err := billingPeriods.GetById(db, "b1")

		require.NoError(t, err)
		assert.Equal(t, &record, got)
	})

	t.Run("stores NULL end date when period is open-ended", func(t *testing.T) {
		db := newDB(t)
		insertSubscription(t, db, "01", "Netflix")

		insertBillingPeriod(t, db, "b1", "01", "2026-01-01", "")

		var nulls int
		require.NoError(t, db.Get(&nulls, `SELECT COUNT(*) FROM billing_periods WHERE end_on IS NULL`))
		assert.Equal(t, 1, nulls)
		got, err := billingPeriods.GetById(db, "b1")
		require.NoError(t, err)
		assert.Nil(t, got.EndOn)
	})

	t.Run("rejects insert when id is empty", func(t *testing.T) {
		db := newDB(t)
		insertSubscription(t, db, "01", "Netflix")

		err := billingPeriods.Insert(db, billingPeriod("", "01", "2026-01-01", ""))

		assert.Error(t, err)
		got, err := billingPeriods.ListBySubscriptionID(db, "01")
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("returns ErrNotFound when period does not exist", func(t *testing.T) {
		db := newDB(t)

		_, err := billingPeriods.GetById(db, "missing")

		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("lists periods of subscription oldest first", func(t *testing.T) {
		db := newDB(t)
		insertSubscription(t, db, "01", "Netflix")
		insertSubscription(t, db, "02", "Spotify")
		later := insertBillingPeriod(t, db, "b2", "01", "2026-03-01", "")
		earlier := insertBillingPeriod(t, db, "b1", "01", "2026-01-01", "2026-03-01")
		insertBillingPeriod(t, db, "b3", "02", "2026-01-01", "")

		got, err := billingPeriods.ListBySubscriptionID(db, "01")

		require.NoError(t, err)
		assert.Equal(t, []domain.BillingPeriodRecord{earlier, later}, got)
	})

	t.Run("replaces columns and clears end date when updated with nil end", func(t *testing.T) {
		db := newDB(t)
		insertSubscription(t, db, "01", "Netflix")
		insertBillingPeriod(t, db, "b1", "01", "2026-01-01", "2026-07-01")
		updated := domain.BillingPeriodRecord{
			ID: "b1", SubscriptionID: "01", StartOn: date("2026-02-01"), Frequency: 12,
			BillingCurrency: "GBP", BillingCost: "100.00", Cost: "115.00",
		}

		affected, err := billingPeriods.Update(db, updated)

		require.NoError(t, err)
		assert.EqualValues(t, 1, affected)
		got, err := billingPeriods.GetById(db, "b1")
		require.NoError(t, err)
		assert.Equal(t, &updated, got)
	})

	t.Run("changes only cost when updating cost", func(t *testing.T) {
		db := newDB(t)
		insertSubscription(t, db, "01", "Netflix")
		record := insertBillingPeriod(t, db, "b1", "01", "2026-01-01", "")

		affected, err := billingPeriods.UpdateCostById(db, "b1", "11.50")

		require.NoError(t, err)
		assert.EqualValues(t, 1, affected)
		record.Cost = "11.50"
		got, err := billingPeriods.GetById(db, "b1")
		require.NoError(t, err)
		assert.Equal(t, &record, got)
	})

	t.Run("reports affected rows when deleting existing and missing periods", func(t *testing.T) {
		db := newDB(t)
		insertSubscription(t, db, "01", "Netflix")
		insertBillingPeriod(t, db, "b1", "01", "2026-01-01", "")

		deleted, errDeleted := billingPeriods.DeleteById(db, "b1")
		missing, errMissing := billingPeriods.DeleteById(db, "b1")

		require.NoError(t, errDeleted)
		require.NoError(t, errMissing)
		assert.EqualValues(t, 1, deleted)
		assert.Zero(t, missing)
	})

	t.Run("reports zero rows when updating missing period", func(t *testing.T) {
		db := newDB(t)

		updated, errUpdate := billingPeriods.Update(db, billingPeriod("missing", "01", "2026-01-01", ""))
		costUpdated, errCost := billingPeriods.UpdateCostById(db, "missing", "1.00")

		require.NoError(t, errUpdate)
		require.NoError(t, errCost)
		assert.Zero(t, updated)
		assert.Zero(t, costUpdated)
	})
}
