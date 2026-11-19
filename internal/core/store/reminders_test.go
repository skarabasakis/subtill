package store_test

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/skarabasakis/subtill/internal/core/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReminders(t *testing.T) {
	reminders := store.Reminders{}

	t.Run("ListAll", func(t *testing.T) {
		t.Run("lists reminders of every subscription", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix")
			insertSubscription(t, db, "02", "Spotify")
			later := insertReminder(t, db, "r2", "01", domain.ReminderActionRenew, "2026-03-01")
			earlier := insertReminder(t, db, "r1", "01", domain.ReminderActionCancel, "2026-02-01")
			other := insertReminder(t, db, "r3", "02", domain.ReminderActionRenew, "2026-01-01")

			got, err := reminders.ListAll(db)

			require.NoError(t, err)
			assert.ElementsMatch(t, []domain.ReminderRecord{earlier, later, other}, got)
		})
	})

	t.Run("ListBySubscriptionID", func(t *testing.T) {
		t.Run("lists reminders of subscription earliest first", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix")
			insertSubscription(t, db, "02", "Spotify")
			later := insertReminder(t, db, "r2", "01", domain.ReminderActionRenew, "2026-03-01")
			earlier := insertReminder(t, db, "r1", "01", domain.ReminderActionCancel, "2026-02-01")
			insertReminder(t, db, "r3", "02", domain.ReminderActionRenew, "2026-01-01")

			got, err := reminders.ListBySubscriptionID(db, "01")

			require.NoError(t, err)
			assert.Equal(t, []domain.ReminderRecord{earlier, later}, got)
		})
	})

	t.Run("GetById", func(t *testing.T) {
		t.Run("returns ErrNotFound when reminder does not exist", func(t *testing.T) {
			db := newDB(t)

			_, err := reminders.GetById(db, "missing")

			assert.ErrorIs(t, err, store.ErrNotFound)
		})
	})

	t.Run("Insert", func(t *testing.T) {
		t.Run("reads back all columns when reminder is inserted", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix")
			record := domain.ReminderRecord{
				ID: "r1", SubscriptionID: "01", Action: domain.ReminderActionCustom,
				CustomNote: "call bank", TriggerOn: date("2026-02-01"), IsTriggered: true,
			}

			require.NoError(t, reminders.Insert(db, record))
			got, err := reminders.GetById(db, "r1")

			require.NoError(t, err)
			assert.Equal(t, &record, got)
		})

		t.Run("stores NULL note when custom note is empty", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix")

			insertReminder(t, db, "r1", "01", domain.ReminderActionRenew, "2026-02-01")

			var nulls int
			require.NoError(t, db.Get(&nulls, `SELECT COUNT(*) FROM reminders WHERE custom_note IS NULL`))
			assert.Equal(t, 1, nulls)
		})

		t.Run("rejects insert when id is empty", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix")

			err := reminders.Insert(db, domain.ReminderRecord{
				SubscriptionID: "01", Action: domain.ReminderActionRenew, TriggerOn: date("2026-02-01"),
			})

			assert.Error(t, err)
			got, err := reminders.ListAll(db)
			require.NoError(t, err)
			assert.Empty(t, got)
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("snoozes reminder when updated with new date and untriggered", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix")
			record := insertReminder(t, db, "r1", "01", domain.ReminderActionRenew, "2026-02-01")
			record.TriggerOn = date("2026-02-08")
			record.IsTriggered = false

			affected, err := reminders.Update(db, record)

			require.NoError(t, err)
			assert.EqualValues(t, 1, affected)
			got, err := reminders.GetById(db, "r1")
			require.NoError(t, err)
			assert.Equal(t, &record, got)
		})

		t.Run("reports zero rows when reminder is missing", func(t *testing.T) {
			db := newDB(t)

			affected, err := reminders.Update(db, domain.ReminderRecord{ID: "missing", Action: domain.ReminderActionRenew})

			require.NoError(t, err)
			assert.Zero(t, affected)
		})
	})

	t.Run("DeleteById", func(t *testing.T) {
		t.Run("reports zero rows when reminder is missing", func(t *testing.T) {
			db := newDB(t)

			affected, err := reminders.DeleteById(db, "missing")

			require.NoError(t, err)
			assert.Zero(t, affected)
		})
	})

	t.Run("DeleteBySubscriptionIDAndAction", func(t *testing.T) {
		t.Run("deletes only matching action when deleting by subscription and action", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix")
			insertReminder(t, db, "r1", "01", domain.ReminderActionCancel, "2026-02-01")
			insertReminder(t, db, "r2", "01", domain.ReminderActionCancel, "2026-03-01")
			renew := insertReminder(t, db, "r3", "01", domain.ReminderActionRenew, "2026-04-01")

			affected, err := reminders.DeleteBySubscriptionIDAndAction(db, "01", domain.ReminderActionCancel)

			require.NoError(t, err)
			assert.EqualValues(t, 2, affected)
			remaining, err := reminders.ListAll(db)
			require.NoError(t, err)
			assert.Equal(t, []domain.ReminderRecord{renew}, remaining)
		})
	})

	t.Run("ListUntriggeredDue", func(t *testing.T) {
		t.Run("includes reminders due on asOf, excluding triggered", func(t *testing.T) {
			db, due, dueToday := seedDueReminders(t)

			got, err := reminders.ListUntriggeredDue(db, date("2026-02-10"))

			require.NoError(t, err)
			assert.Equal(t, []domain.ReminderRecord{due, dueToday}, got)
		})
	})

	t.Run("CountDue", func(t *testing.T) {
		t.Run("counts triggered and untriggered reminders due on asOf", func(t *testing.T) {
			db, _, _ := seedDueReminders(t)

			got, err := reminders.CountDue(db, date("2026-02-10"))

			require.NoError(t, err)
			assert.Equal(t, 3, got)
		})
	})
}

// seedDueReminders creates, relative to 2026-02-10, an untriggered reminder
// due earlier, one due on that date, one due later and a triggered one due
// earlier. It returns the two untriggered reminders that are due.
func seedDueReminders(t *testing.T) (db *sqlx.DB, due, dueToday domain.ReminderRecord) {
	t.Helper()
	db = newDB(t)
	insertSubscription(t, db, "01", "Netflix")
	due = insertReminder(t, db, "r1", "01", domain.ReminderActionRenew, "2026-02-01")
	dueToday = insertReminder(t, db, "r2", "01", domain.ReminderActionRenew, "2026-02-10")
	insertReminder(t, db, "r3", "01", domain.ReminderActionRenew, "2026-02-11")
	triggered := insertReminder(t, db, "r4", "01", domain.ReminderActionRenew, "2026-01-01")
	triggered.IsTriggered = true
	_, err := store.Reminders{}.Update(db, triggered)
	require.NoError(t, err)
	return db, due, dueToday
}
