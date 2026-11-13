package store

import (
	"cloud.google.com/go/civil"
	"crdx.org/hereduck"
	"github.com/jmoiron/sqlx"
	"github.com/skarabasakis/subtill/internal/core/domain"
)

type Reminders struct{}

var reminderColumns = []string{
	"id",
	"subscription_id",
	"action",
	"custom_note",
	"trigger_on",
	"is_triggered",
}

func (Reminders) ListAll(q sqlx.Ext) ([]domain.ReminderRecord, error) {
	return listReminders(q, "SELECT "+columnList(reminderColumns)+" FROM reminders", map[string]any{})
}

// ListBySubscriptionID returns the reminders of a subscription, earliest
// first.
func (Reminders) ListBySubscriptionID(q sqlx.Ext, subscriptionID string) ([]domain.ReminderRecord, error) {
	return listReminders(q, "SELECT "+columnList(reminderColumns)+" "+hereduck.D(`
		 FROM reminders
		 WHERE subscription_id = :subscription_id
		 ORDER BY trigger_on
	`), map[string]any{"subscription_id": subscriptionID})
}

func (Reminders) GetById(q sqlx.Ext, id string) (*domain.ReminderRecord, error) {
	var row reminderRow
	query := "SELECT " + columnList(reminderColumns) + " FROM reminders WHERE id = :id"
	if err := getOne(q, &row, query, map[string]any{"id": id}); err != nil {
		return nil, err
	}
	record, err := row.record()
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (Reminders) Insert(q sqlx.Ext, reminder domain.ReminderRecord) error {
	return exec(q, hereduck.D(`
		INSERT INTO reminders (id, subscription_id, action, custom_note, trigger_on, is_triggered)
		VALUES (:id, :subscription_id, :action, NULLIF(:custom_note, ''), :trigger_on, :is_triggered)
	`), reminderRowFrom(reminder))
}

// Update replaces the action, note, trigger date and triggered flag.
func (Reminders) Update(q sqlx.Ext, reminder domain.ReminderRecord) (int64, error) {
	return execRowsAffected(q, hereduck.D(`
		UPDATE reminders
		   SET action = :action, custom_note = NULLIF(:custom_note, ''),
		       trigger_on = :trigger_on, is_triggered = :is_triggered
		 WHERE id = :id
	`), reminderRowFrom(reminder))
}

func (Reminders) DeleteById(q sqlx.Ext, id string) (int64, error) {
	return execRowsAffected(q, `DELETE FROM reminders WHERE id = :id`, map[string]any{"id": id})
}

func (Reminders) DeleteBySubscriptionIDAndAction(q sqlx.Ext, subscriptionID string, action domain.ReminderAction) (int64, error) {
	return execRowsAffected(q, `DELETE FROM reminders WHERE subscription_id = :subscription_id AND action = :action`,
		map[string]any{"subscription_id": subscriptionID, "action": action})
}

// ListUntriggeredDue returns the reminders due on or before asOf whose
// notification has not been sent, earliest first.
func (Reminders) ListUntriggeredDue(q sqlx.Ext, asOf civil.Date) ([]domain.ReminderRecord, error) {
	return listReminders(q, "SELECT "+columnList(reminderColumns)+" "+hereduck.D(`
		 FROM reminders
		 WHERE trigger_on <= :as_of AND is_triggered = 0
		 ORDER BY trigger_on
	`), map[string]any{"as_of": asOf.String()})
}

// CountDue returns the number of reminders due on or before asOf.
func (Reminders) CountDue(q sqlx.Ext, asOf civil.Date) (int, error) {
	var count int
	err := getOne(q, &count, `SELECT COUNT(*) FROM reminders WHERE trigger_on <= :as_of`,
		map[string]any{"as_of": asOf.String()})
	return count, err
}

func listReminders(q sqlx.Ext, query string, arg any) ([]domain.ReminderRecord, error) {
	var rows []reminderRow
	if err := selectAll(q, &rows, query, arg); err != nil {
		return nil, err
	}
	return reminderRecords(rows)
}
