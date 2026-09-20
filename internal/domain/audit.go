package domain

import (
	"time"

	"github.com/google/uuid"
)

type LeakSeverity string

const (
	SeverityLow    LeakSeverity = "low"
	SeverityMedium LeakSeverity = "medium"
	SeverityHigh   LeakSeverity = "high"
)

type MoneyLeakAlert struct {
	ID          string       `json:"id"`
	Type        string       `json:"type"` // e.g. "idle_subscription", "weekend_spike", "unsettled_reimbursement", "duplicate_vendor"
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Severity    LeakSeverity `json:"severity"`
	PotentialLoss float64    `json:"potential_loss"`
	ActionableTip string     `json:"actionable_tip"`
}

type HouseholdAuditReport struct {
	HouseholdID          uuid.UUID            `json:"household_id"`
	HouseholdName        string               `json:"household_name"`
	Month                int                  `json:"month"`
	Year                 int                  `json:"year"`
	TotalSharedSpend     float64              `json:"total_shared_spend"`
	PerMemberAverage     float64              `json:"per_member_average"`
	TopCategories        map[string]float64   `json:"top_categories"`
	MemberContributions  map[string]float64   `json:"member_contributions"`
	PendingSettlements   []DebtResolution     `json:"pending_settlements"`
	SavingsOpportunities []string             `json:"savings_opportunities"`
	GeneratedAt          time.Time            `json:"generated_at"`
}

type AdminPlatformMetrics struct {
	TotalUsers              int       `json:"total_users"`
	TotalHouseholds         int       `json:"total_households"`
	TotalExpensesTracked    int       `json:"total_expenses_tracked"`
	TotalVolumeTracked      float64   `json:"total_volume_tracked"`
	TotalReceivablesPending float64   `json:"total_receivables_pending"`
	ActiveUsers7Days        int       `json:"active_users_7_days"`
	GeneratedAt             time.Time `json:"generated_at"`
}
