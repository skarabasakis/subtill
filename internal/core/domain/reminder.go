package domain

import "cloud.google.com/go/civil"

type ReminderAction string

const (
	ReminderActionCancel ReminderAction = "cancel"
	ReminderActionRenew  ReminderAction = "renew"
	ReminderActionCustom ReminderAction = "custom"
)

type ReminderRecord struct {
	ID             string         `json:"id"`
	SubscriptionID string         `json:"subscriptionId"`
	Action         ReminderAction `json:"action"`
	CustomNote     string         `json:"customNote"`
	TriggerOn      civil.Date     `json:"triggerOn"`
	IsTriggered    bool           `json:"isTriggered"`
}
