package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"whisperledger-backend/internal/domain"
)

type SafeToSpendResponse struct {
	MonthlyBudget         float64 `json:"monthly_budget"`
	TruePersonalOutflow   float64 `json:"true_personal_outflow"`
	PendingHouseholdDebt  float64 `json:"pending_household_debt"`
	ExpectedRecoverables  float64 `json:"expected_recoverables"`
	SafeToSpendDaily      float64 `json:"safe_to_spend_daily"`
	SafeToSpendRemaining  float64 `json:"safe_to_spend_remaining"`
	DaysRemainingInMonth  int     `json:"days_remaining_in_month"`
}

type DetectiveService struct {
	expenseRepo    domain.ExpenseRepository
	receivableRepo domain.ReceivableRepository
	householdRepo  domain.HouseholdRepository
}

func NewDetectiveService(
	expenseRepo domain.ExpenseRepository,
	receivableRepo domain.ReceivableRepository,
	householdRepo domain.HouseholdRepository,
) *DetectiveService {
	return &DetectiveService{
		expenseRepo:    expenseRepo,
		receivableRepo: receivableRepo,
		householdRepo:  householdRepo,
	}
}

func (s *DetectiveService) DetectMoneyLeaks(ctx context.Context, userID uuid.UUID) ([]domain.MoneyLeakAlert, error) {
	var alerts []domain.MoneyLeakAlert
	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	// 1. Check for unrecovered receivables past due or older than 14 days
	pendingStatus := domain.ReceivablePending
	receivables, _, err := s.receivableRepo.List(ctx, domain.ReceivableFilter{
		UserID: userID,
		Status: &pendingStatus,
		Limit:  50,
	})
	if err == nil {
		for _, r := range receivables {
			unsettled := r.Amount - r.SettledAmount
			if unsettled > 0 {
				isPastDue := r.DueDate != nil && now.After(*r.DueDate)
				isOld := now.Sub(r.CreatedAt) > 14*24*time.Hour
				if isPastDue || isOld {
					alerts = append(alerts, domain.MoneyLeakAlert{
						ID:            fmt.Sprintf("rec_leak_%s", r.ID.String()),
						Type:          "unrecovered_receivable",
						Title:         fmt.Sprintf("Pending recovery: ₹%.2f from %s", unsettled, r.DebtorName),
						Description:   fmt.Sprintf("Money lent or refund for '%s' is overdue. Send a quick 1-click friendly ping.", r.Title),
						Severity:      domain.SeverityHigh,
						PotentialLoss: unsettled,
						ActionableTip: fmt.Sprintf("Tap to share UPI payment reminder with %s", r.DebtorName),
					})
				}
			}
		}
	}

	// 2. Check for duplicate merchant transactions within 24h
	expenses, _, err := s.expenseRepo.List(ctx, domain.ExpenseFilter{
		UserID:    userID,
		StartDate: &startOfMonth,
		EndDate:   &now,
		Limit:     100,
	})
	if err == nil && len(expenses) > 1 {
		seenMerchants := make(map[string]domain.Expense)
		for _, e := range expenses {
			key := fmt.Sprintf("%s_%.2f", e.Merchant, e.Amount)
			if prev, exists := seenMerchants[key]; exists {
				if math.Abs(prev.Date.Sub(e.Date).Hours()) < 24 {
					alerts = append(alerts, domain.MoneyLeakAlert{
						ID:            fmt.Sprintf("dup_%s_%s", prev.ID.String(), e.ID.String()),
						Type:          "duplicate_charge",
						Title:         fmt.Sprintf("Potential double charge: ₹%.2f at %s", e.Amount, e.Merchant),
						Description:   "Two transactions of identical amount recorded at this vendor within 24 hours.",
						Severity:      domain.SeverityMedium,
						PotentialLoss: e.Amount,
						ActionableTip: "Verify bank SMS or statement to confirm if duplicate deduction occurred.",
					})
				}
			} else {
				seenMerchants[key] = *e
			}
		}
	}

	return alerts, nil
}

func (s *DetectiveService) CalculateSafeToSpend(ctx context.Context, userID uuid.UUID, monthlyBudget float64) (*SafeToSpendResponse, error) {
	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, -1)
	daysRemaining := endOfMonth.Day() - now.Day() + 1
	if daysRemaining < 1 {
		daysRemaining = 1
	}

	if monthlyBudget <= 0 {
		monthlyBudget = 50000 // Default baseline budget
	}

	summary, err := s.expenseRepo.GetSummary(ctx, userID, startOfMonth, now)
	if err != nil {
		return nil, err
	}

	pendingReceivables, err := s.receivableRepo.GetTotalPending(ctx, userID)
	if err != nil {
		pendingReceivables = 0
	}

	remaining := monthlyBudget - summary.TruePersonalTotal
	daily := remaining / float64(daysRemaining)
	if daily < 0 {
		daily = 0
	}

	return &SafeToSpendResponse{
		MonthlyBudget:        monthlyBudget,
		TruePersonalOutflow:  summary.TruePersonalTotal,
		ExpectedRecoverables: pendingReceivables,
		SafeToSpendRemaining: remaining,
		SafeToSpendDaily:     math.Round(daily*100) / 100,
		DaysRemainingInMonth: daysRemaining,
	}, nil
}

func (s *DetectiveService) GenerateHouseholdMonthlyAudit(ctx context.Context, householdID uuid.UUID, month, year int) (*domain.HouseholdAuditReport, error) {
	household, err := s.householdRepo.GetByID(ctx, householdID)
	if err != nil || household == nil {
		return nil, ErrHouseholdNotFound
	}

	if month == 0 {
		month = int(time.Now().UTC().Month())
	}
	if year == 0 {
		year = time.Now().UTC().Year()
	}

	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, -1)

	isSharedFilter := domain.OutflowSharedHousehold
	expenses, _, err := s.expenseRepo.List(ctx, domain.ExpenseFilter{
		HouseholdID: &householdID,
		OutflowType: &isSharedFilter,
		StartDate:   &startDate,
		EndDate:     &endDate,
		Limit:       500,
	})
	if err != nil {
		return nil, err
	}

	var totalShared float64
	topCategories := make(map[string]float64)
	memberSpend := make(map[string]float64)

	for _, e := range expenses {
		totalShared += e.Amount
		topCategories[string(e.Category)] += e.Amount
	}

	members, err := s.householdRepo.ListMembers(ctx, householdID)
	if err == nil {
		for _, m := range members {
			memberSpend[m.User.FullName] = 0
		}
	}

	for _, e := range expenses {
		for _, sp := range e.Splits {
			for _, m := range members {
				if m.UserID == sp.UserID {
					memberSpend[m.User.FullName] += sp.Amount
				}
			}
		}
	}

	perMemberAvg := 0.0
	if len(members) > 0 {
		perMemberAvg = totalShared / float64(len(members))
	}

	savings := []string{
		"Bulk grocery ordering could reduce household grocery expenses by an estimated 12-15%.",
		"Review shared streaming subscriptions; consolidate individual Spotify or Netflix slots into household family plans.",
		"Zero-fee instant settlement through UPI avoids payment gateway overheads.",
	}

	return &domain.HouseholdAuditReport{
		HouseholdID:          householdID,
		HouseholdName:        household.Name,
		Month:                month,
		Year:                 year,
		TotalSharedSpend:     totalShared,
		PerMemberAverage:     math.Round(perMemberAvg*100) / 100,
		TopCategories:        topCategories,
		MemberContributions:  memberSpend,
		SavingsOpportunities: savings,
		GeneratedAt:          time.Now().UTC(),
	}, nil
}
