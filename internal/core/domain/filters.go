package domain

const (
	StringFilterOperatorEquals   = "equals"
	StringFilterOperatorContains = "contains"

	ArrayFilterOperatorAny = "any"
	ArrayFilterOperatorAll = "all"
)

type StringFilter struct {
	Value    string `json:"value,omitempty"`
	Operator string `json:"operator,omitempty" validate:"omitempty,oneof=contains equals"`
}

type ArrayFilter struct {
	Values   []string `json:"values,omitempty"`
	Operator string   `json:"operator,omitempty" validate:"omitempty,oneof=all any"`
}

// SubscriptionFiltersDb is the subset of subscription filters that
// store.Subscriptions applies in SQL. The zero value matches every
// subscription.
type SubscriptionFiltersDb struct {
	Name   StringFilter         `json:"name"   validate:"dive"`
	Group  string               `json:"group"`
	Tags   ArrayFilter          `json:"tags"   validate:"dive"`
	Status []SubscriptionStatus `json:"status" validate:"dive,omitempty"`
}
