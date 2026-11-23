package store_test

import (
	"testing"

	"cloud.google.com/go/civil"
	"github.com/jmoiron/sqlx"
	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/skarabasakis/subtill/internal/core/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptions(t *testing.T) {
	subscriptions := store.Subscriptions{}

	t.Run("GetByName", func(t *testing.T) {
		t.Run("finds subscription when name differs only by case", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix")

			got, err := subscriptions.GetByName(db, "NETFLIX")

			require.NoError(t, err)
			assert.Equal(t, "01", got.ID)
		})

		t.Run("returns ErrNotFound when name is missing", func(t *testing.T) {
			db := newDB(t)

			_, err := subscriptions.GetByName(db, "missing")

			assert.ErrorIs(t, err, store.ErrNotFound)
		})
	})

	t.Run("GetById", func(t *testing.T) {
		t.Run("returns ErrNotFound when id is missing", func(t *testing.T) {
			db := newDB(t)

			_, err := subscriptions.GetById(db, "missing")

			assert.ErrorIs(t, err, store.ErrNotFound)
		})
	})

	t.Run("ListAll", func(t *testing.T) {
		t.Run("lists newest id first without tags when filter is nil", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix", "video")
			insertSubscription(t, db, "02", "Spotify")

			got, err := subscriptions.ListAll(db, nil, civil.Date{})

			require.NoError(t, err)
			assert.Equal(t, []domain.SubscriptionRecord{{ID: "02", Name: "Spotify"}, {ID: "01", Name: "Netflix"}}, got)
		})

		t.Run("returns empty list when there are no subscriptions", func(t *testing.T) {
			db := newDB(t)

			got, err := subscriptions.ListAll(db, &domain.SubscriptionFiltersDb{}, civil.Date{})

			require.NoError(t, err)
			assert.Empty(t, got)
		})
	})

	t.Run("ListAll and Count", func(t *testing.T) {
		today := date("2026-06-15")

		// listNames returns the names matched by filter, checking that Count
		// agrees with ListAll.
		listNames := func(t *testing.T, filter *domain.SubscriptionFiltersDb) []string {
			t.Helper()
			db := seedFilterFixtures(t)
			got, err := subscriptions.ListAll(db, filter, today)
			require.NoError(t, err)
			count, err := subscriptions.Count(db, filter, today)
			require.NoError(t, err)
			assert.Equal(t, len(got), count, "Count disagrees with ListAll")
			return subscriptionNames(got)
		}

		t.Run("matches every subscription when filter is zero", func(t *testing.T) {
			filter := &domain.SubscriptionFiltersDb{}

			got := listNames(t, filter)

			assert.Equal(t, []string{"Gym_100%", "Gym", "Spotify", "Netflix"}, got)
		})

		t.Run("matches name case-insensitively when operator is equals", func(t *testing.T) {
			filter := &domain.SubscriptionFiltersDb{Name: domain.StringFilter{Value: "NETFLIX"}}

			got := listNames(t, filter)

			assert.Equal(t, []string{"Netflix"}, got)
		})

		t.Run("matches name substring case-insensitively when operator is contains", func(t *testing.T) {
			filter := &domain.SubscriptionFiltersDb{Name: domain.StringFilter{
				Value: "gym", Operator: domain.StringFilterOperatorContains,
			}}

			got := listNames(t, filter)

			assert.Equal(t, []string{"Gym_100%", "Gym"}, got)
		})

		t.Run("treats percent and underscore literally when operator is contains", func(t *testing.T) {
			filter := &domain.SubscriptionFiltersDb{Name: domain.StringFilter{
				Value: "_100%", Operator: domain.StringFilterOperatorContains,
			}}

			got := listNames(t, filter)

			assert.Equal(t, []string{"Gym_100%"}, got)
		})

		t.Run("matches exact group when group is set", func(t *testing.T) {
			filter := &domain.SubscriptionFiltersDb{Group: "Media"}

			got := listNames(t, filter)

			assert.Equal(t, []string{"Spotify", "Netflix"}, got)
		})

		t.Run("matches any requested tag when operator is any", func(t *testing.T) {
			filter := &domain.SubscriptionFiltersDb{Tags: domain.ArrayFilter{Values: []string{"VIDEO", "health"}}}

			got := listNames(t, filter)

			assert.Equal(t, []string{"Gym", "Netflix"}, got)
		})

		t.Run("requires every requested tag when operator is all", func(t *testing.T) {
			filter := &domain.SubscriptionFiltersDb{Tags: domain.ArrayFilter{
				Values: []string{"family", "video", "family"}, Operator: domain.ArrayFilterOperatorAll,
			}}

			got := listNames(t, filter)

			assert.Equal(t, []string{"Netflix"}, got)
		})

		t.Run("matches ongoing subscriptions when status is ongoing", func(t *testing.T) {
			filter := &domain.SubscriptionFiltersDb{Status: []domain.SubscriptionStatus{domain.SubscriptionStatusOngoing}}

			got := listNames(t, filter)

			assert.Equal(t, []string{"Netflix"}, got)
		})

		t.Run("matches scheduled subscriptions when status is scheduled", func(t *testing.T) {
			filter := &domain.SubscriptionFiltersDb{Status: []domain.SubscriptionStatus{domain.SubscriptionStatusScheduled}}

			got := listNames(t, filter)

			assert.Equal(t, []string{"Spotify"}, got)
		})

		t.Run("matches ended and period-less subscriptions when status is ended", func(t *testing.T) {
			filter := &domain.SubscriptionFiltersDb{Status: []domain.SubscriptionStatus{domain.SubscriptionStatusEnded}}

			got := listNames(t, filter)

			assert.Equal(t, []string{"Gym_100%", "Gym"}, got)
		})

		t.Run("matches either status when several statuses are requested", func(t *testing.T) {
			filter := &domain.SubscriptionFiltersDb{Status: []domain.SubscriptionStatus{
				domain.SubscriptionStatusOngoing, domain.SubscriptionStatusScheduled,
			}}

			got := listNames(t, filter)

			assert.Equal(t, []string{"Spotify", "Netflix"}, got)
		})

		t.Run("combines predicates when several fields are set", func(t *testing.T) {
			filter := &domain.SubscriptionFiltersDb{
				Group:  "Media",
				Tags:   domain.ArrayFilter{Values: []string{"music", "video"}},
				Status: []domain.SubscriptionStatus{domain.SubscriptionStatusScheduled},
			}

			got := listNames(t, filter)

			assert.Equal(t, []string{"Spotify"}, got)
		})
	})

	t.Run("Insert", func(t *testing.T) {
		t.Run("reads back all columns when subscription is inserted", func(t *testing.T) {
			db := newDB(t)
			record := domain.SubscriptionRecord{
				ID: "01", Name: "Netflix", Group: "Video", Notes: "family", Icon: "data:image/png;base64,iVBORw0KGgo=", URL: "https://netflix.com",
			}

			require.NoError(t, subscriptions.Insert(db, record))
			got, err := subscriptions.GetById(db, "01")

			require.NoError(t, err)
			assert.Equal(t, &record, got)
		})

		t.Run("stores empty strings as NULL when optional columns are empty", func(t *testing.T) {
			db := newDB(t)

			insertSubscription(t, db, "01", "Netflix")

			var nulls int
			require.NoError(t, db.Get(&nulls, `SELECT COUNT(*) FROM subscriptions
				WHERE "group" IS NULL AND notes IS NULL AND icon IS NULL AND url IS NULL`))
			assert.Equal(t, 1, nulls)
			got, err := subscriptions.GetById(db, "01")
			require.NoError(t, err)
			assert.Equal(t, &domain.SubscriptionRecord{ID: "01", Name: "Netflix"}, got)
		})

		t.Run("rejects insert when id is empty", func(t *testing.T) {
			db := newDB(t)

			err := subscriptions.Insert(db, domain.SubscriptionRecord{Name: "Netflix"})

			assert.Error(t, err)
			count, err := subscriptions.Count(db, nil, civil.Date{})
			require.NoError(t, err)
			assert.Zero(t, count)
		})

		t.Run("rejects insert when icon is not a PNG data URI", func(t *testing.T) {
			for name, icon := range map[string]string{
				"url":           "https://netflix.com/icon.png",
				"uppercase":     "DATA:IMAGE/PNG;BASE64,AAAA",
				"jpeg":          "data:image/jpeg;base64,AAAA",
				"empty payload": "data:image/png;base64,",
			} {
				t.Run(name, func(t *testing.T) {
					db := newDB(t)

					err := subscriptions.Insert(db, domain.SubscriptionRecord{ID: "01", Name: "Netflix", Icon: icon})

					assert.Error(t, err)
					count, err := subscriptions.Count(db, nil, civil.Date{})
					require.NoError(t, err)
					assert.Zero(t, count)
				})
			}
		})

		t.Run("discards writes when transaction is rolled back", func(t *testing.T) {
			db := newDB(t)
			tx, err := db.Beginx()
			require.NoError(t, err)

			insertSubscription(t, tx, "01", "Netflix")
			require.NoError(t, tx.Rollback())

			count, err := subscriptions.Count(db, nil, civil.Date{})
			require.NoError(t, err)
			assert.Zero(t, count)
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("replaces columns and reports one row when subscription exists", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix")
			updated := domain.SubscriptionRecord{ID: "01", Name: "Netflix HD", Group: "Video"}

			affected, err := subscriptions.Update(db, updated)

			require.NoError(t, err)
			assert.EqualValues(t, 1, affected)
			got, err := subscriptions.GetById(db, "01")
			require.NoError(t, err)
			assert.Equal(t, &updated, got)
		})

		t.Run("rejects update when icon is not a PNG data URI", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix")

			_, err := subscriptions.Update(db, domain.SubscriptionRecord{ID: "01", Name: "Netflix", Icon: "https://netflix.com/icon.png"})

			assert.Error(t, err)
			got, err := subscriptions.GetById(db, "01")
			require.NoError(t, err)
			assert.Equal(t, &domain.SubscriptionRecord{ID: "01", Name: "Netflix"}, got)
		})

		t.Run("reports zero rows when subscription is missing", func(t *testing.T) {
			db := newDB(t)

			affected, err := subscriptions.Update(db, domain.SubscriptionRecord{ID: "missing", Name: "x"})

			require.NoError(t, err)
			assert.Zero(t, affected)
		})
	})

	t.Run("DeleteById", func(t *testing.T) {
		t.Run("reports zero rows when subscription is missing", func(t *testing.T) {
			db := newDB(t)

			affected, err := subscriptions.DeleteById(db, "missing")

			require.NoError(t, err)
			assert.Zero(t, affected)
		})

		t.Run("cascades to tags, billing periods and reminders when subscription is deleted", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix", "video")
			insertBillingPeriod(t, db, "b1", "01", "2026-01-01", "")
			insertReminder(t, db, "r1", "01", domain.ReminderActionRenew, "2026-02-01")

			affected, err := subscriptions.DeleteById(db, "01")

			require.NoError(t, err)
			assert.EqualValues(t, 1, affected)
			var children int
			require.NoError(t, db.Get(&children, `SELECT
				(SELECT COUNT(*) FROM tags) + (SELECT COUNT(*) FROM billing_periods) + (SELECT COUNT(*) FROM reminders)`))
			assert.Zero(t, children)
		})
	})

	t.Run("WithAssociations", func(t *testing.T) {
		t.Run("loads sorted tags and billing periods without multiplying rows", func(t *testing.T) {
			db := newDB(t)
			netflix := insertSubscription(t, db, "01", "Netflix", "video", "family")
			later := insertBillingPeriod(t, db, "b2", "01", "2026-03-01", "")
			earlier := insertBillingPeriod(t, db, "b1", "01", "2026-01-01", "2026-03-01")

			got, err := subscriptions.WithAssociations(db, netflix)

			require.NoError(t, err)
			assert.Equal(t, []string{"family", "video"}, got.Tags)
			assert.Equal(t, []domain.BillingPeriodRecord{earlier, later}, got.BillingPeriods)
			assert.Equal(t, "Netflix", got.Name)
		})

		t.Run("returns empty slices when subscription has no associations", func(t *testing.T) {
			db := newDB(t)
			netflix := insertSubscription(t, db, "01", "Netflix")

			got, err := subscriptions.WithAssociations(db, netflix)

			require.NoError(t, err)
			assert.Equal(t, []string{}, got.Tags)
			assert.Equal(t, []domain.BillingPeriodRecord{}, got.BillingPeriods)
		})

		t.Run("returns ErrNotFound when subscription does not exist", func(t *testing.T) {
			db := newDB(t)

			_, err := subscriptions.WithAssociations(db, domain.SubscriptionRecord{ID: "missing"})

			assert.ErrorIs(t, err, store.ErrNotFound)
		})
	})

	t.Run("WithAssociationsA", func(t *testing.T) {
		t.Run("keeps input order when loading many subscriptions", func(t *testing.T) {
			db := newDB(t)
			netflix := insertSubscription(t, db, "01", "Netflix", "video")
			spotify := insertSubscription(t, db, "02", "Spotify", "music")
			period := insertBillingPeriod(t, db, "b1", "02", "2026-01-01", "")

			got, err := subscriptions.WithAssociationsA(db, []domain.SubscriptionRecord{spotify, netflix})

			require.NoError(t, err)
			require.Len(t, got, 2)
			assert.Equal(t, "Spotify", got[0].Name)
			assert.Equal(t, []string{"music"}, got[0].Tags)
			assert.Equal(t, []domain.BillingPeriodRecord{period}, got[0].BillingPeriods)
			assert.Equal(t, "Netflix", got[1].Name)
			assert.Equal(t, []string{"video"}, got[1].Tags)
			assert.Empty(t, got[1].BillingPeriods)
		})

		t.Run("returns empty slice when input is empty", func(t *testing.T) {
			db := newDB(t)

			got, err := subscriptions.WithAssociationsA(db, nil)

			require.NoError(t, err)
			assert.Equal(t, []domain.SubscriptionWithAssociations{}, got)
		})
	})
}

// seedFilterFixtures creates, relative to 2026-06-15:
//   - Netflix (Media; video, family): ongoing, since a period ended earlier
//     and the current one is open-ended
//   - Spotify (Media; music): scheduled to start later
//   - Gym (Health; health): ended on the current date
//   - Gym_100% (no group, no tags): no billing periods, so ended
func seedFilterFixtures(t *testing.T) *sqlx.DB {
	t.Helper()
	db := newDB(t)
	require.NoError(t, store.Subscriptions{}.Insert(db, domain.SubscriptionRecord{ID: "01", Name: "Netflix", Group: "Media"}))
	require.NoError(t, store.Subscriptions{}.Insert(db, domain.SubscriptionRecord{ID: "02", Name: "Spotify", Group: "Media"}))
	require.NoError(t, store.Subscriptions{}.Insert(db, domain.SubscriptionRecord{ID: "03", Name: "Gym", Group: "Health"}))
	insertSubscription(t, db, "04", "Gym_100%")
	insertTag(t, db, "01", "video")
	insertTag(t, db, "01", "family")
	insertTag(t, db, "02", "music")
	insertTag(t, db, "03", "health")
	insertBillingPeriod(t, db, "b1", "01", "2025-01-01", "2026-01-01")
	insertBillingPeriod(t, db, "b2", "01", "2026-01-01", "")
	insertBillingPeriod(t, db, "b3", "02", "2026-07-01", "")
	insertBillingPeriod(t, db, "b4", "03", "2026-01-01", "2026-06-15")
	return db
}
