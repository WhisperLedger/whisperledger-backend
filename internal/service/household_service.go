package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"whisperledger-backend/internal/domain"
)

var (
	ErrHouseholdNotFound  = errors.New("household not found")
	ErrAlreadyInHousehold = errors.New("user is already a member of a household")
	ErrNotInHousehold     = errors.New("user is not a member of any household")
)

type HouseholdService struct {
	householdRepo domain.HouseholdRepository
	userRepo      domain.UserRepository
}

func NewHouseholdService(householdRepo domain.HouseholdRepository, userRepo domain.UserRepository) *HouseholdService {
	return &HouseholdService{
		householdRepo: householdRepo,
		userRepo:      userRepo,
	}
}

func (s *HouseholdService) CreateHousehold(ctx context.Context, userID uuid.UUID, name, currency string) (*domain.Household, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	if user.HouseholdID != nil {
		return nil, ErrAlreadyInHousehold
	}

	if currency == "" {
		currency = "INR"
	}

	codeBytes := make([]byte, 4)
	_, _ = rand.Read(codeBytes)
	inviteCode := strings.ToUpper(hex.EncodeToString(codeBytes))

	household := &domain.Household{
		ID:         uuid.New(),
		Name:       name,
		InviteCode: inviteCode,
		Currency:   currency,
		CreatedBy:  userID,
	}

	if err := s.householdRepo.Create(ctx, household); err != nil {
		return nil, err
	}

	return s.householdRepo.GetByID(ctx, household.ID)
}

func (s *HouseholdService) JoinByInviteCode(ctx context.Context, userID uuid.UUID, inviteCode string) (*domain.Household, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	if user.HouseholdID != nil {
		return nil, ErrAlreadyInHousehold
	}

	household, err := s.householdRepo.GetByInviteCode(ctx, strings.TrimSpace(inviteCode))
	if err != nil {
		return nil, err
	}
	if household == nil {
		return nil, ErrHouseholdNotFound
	}

	member := &domain.HouseholdMember{
		HouseholdID: household.ID,
		UserID:      userID,
		Role:        domain.MemberRoleMember,
	}

	if err := s.householdRepo.AddMember(ctx, member); err != nil {
		return nil, err
	}

	return s.householdRepo.GetByID(ctx, household.ID)
}

func (s *HouseholdService) GetHousehold(ctx context.Context, householdID uuid.UUID) (*domain.Household, error) {
	h, err := s.householdRepo.GetByID(ctx, householdID)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, ErrHouseholdNotFound
	}
	return h, nil
}

func (s *HouseholdService) RequireMember(ctx context.Context, userID, householdID uuid.UUID) error {
	members, err := s.householdRepo.ListMembers(ctx, householdID)
	if err != nil {
		return err
	}
	for _, member := range members {
		if member.UserID == userID {
			return nil
		}
	}
	return ErrHouseholdNotFound
}

func (s *HouseholdService) LeaveHousehold(ctx context.Context, userID, householdID uuid.UUID) error {
	return s.householdRepo.RemoveMember(ctx, householdID, userID)
}

func (s *HouseholdService) GetBalances(ctx context.Context, householdID uuid.UUID) ([]domain.HouseholdBalance, error) {
	return s.householdRepo.GetHouseholdBalances(ctx, householdID)
}

// CalculateOptimalSettlements computes the minimum cash flow debt graph
func (s *HouseholdService) CalculateOptimalSettlements(ctx context.Context, householdID uuid.UUID) ([]domain.DebtResolution, error) {
	balances, err := s.householdRepo.GetHouseholdBalances(ctx, householdID)
	if err != nil {
		return nil, err
	}

	type PersonBalance struct {
		UserID   uuid.UUID
		UserName string
		Balance  float64
	}

	var debtors []PersonBalance
	var creditors []PersonBalance

	for _, b := range balances {
		rounded := math.Round(b.NetOwed*100) / 100
		if rounded > 0.01 {
			creditors = append(creditors, PersonBalance{
				UserID:   b.UserID,
				UserName: b.UserName,
				Balance:  rounded,
			})
		} else if rounded < -0.01 {
			debtors = append(debtors, PersonBalance{
				UserID:   b.UserID,
				UserName: b.UserName,
				Balance:  -rounded, // Store positive amount owed
			})
		}
	}

	var resolutions []domain.DebtResolution

	// Greedily match maximum debtor with maximum creditor
	for len(debtors) > 0 && len(creditors) > 0 {
		sort.Slice(debtors, func(i, j int) bool {
			return debtors[i].Balance > debtors[j].Balance
		})
		sort.Slice(creditors, func(i, j int) bool {
			return creditors[i].Balance > creditors[j].Balance
		})

		d := &debtors[0]
		c := &creditors[0]

		settleAmount := math.Min(d.Balance, c.Balance)
		settleAmount = math.Round(settleAmount*100) / 100

		if settleAmount > 0.01 {
			resolutions = append(resolutions, domain.DebtResolution{
				FromUserID:   d.UserID,
				FromUserName: d.UserName,
				ToUserID:     c.UserID,
				ToUserName:   c.UserName,
				Amount:       settleAmount,
			})
		}

		d.Balance -= settleAmount
		c.Balance -= settleAmount

		if d.Balance < 0.01 {
			debtors = debtors[1:]
		}
		if c.Balance < 0.01 {
			creditors = creditors[1:]
		}
	}

	return resolutions, nil
}

func (s *HouseholdService) SettleDebt(ctx context.Context, householdID, fromUser, toUser uuid.UUID, amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if err := s.householdRepo.RecordSettlement(ctx, householdID, fromUser, toUser, amount); err != nil {
		return err
	}
	go s.notifySettlement(householdID, fromUser, toUser, amount)
	return nil
}

func (s *HouseholdService) notifySettlement(householdID, fromUser, toUser uuid.UUID, amount float64) {
	if s.userRepo == nil || s.householdRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	household, _ := s.householdRepo.GetByID(ctx, householdID)
	householdName := "Your flat"
	if household != nil && household.Name != "" {
		householdName = household.Name
	}

	payer, _ := s.userRepo.GetByID(ctx, fromUser)
	payerName := "A flatmate"
	if payer != nil && payer.FullName != "" {
		payerName = payer.FullName
	}

	recipient, err := s.userRepo.GetByID(ctx, toUser)
	if err != nil || recipient == nil || recipient.PushToken == nil || *recipient.PushToken == "" {
		return
	}
	token := strings.TrimSpace(*recipient.PushToken)
	if token == "" {
		return
	}

	type expoPushMessage struct {
		To        string                 `json:"to"`
		Title     string                 `json:"title"`
		Body      string                 `json:"body"`
		Sound     string                 `json:"sound"`
		Priority  string                 `json:"priority"`
		ChannelID string                 `json:"channelId"`
		Data      map[string]interface{} `json:"data,omitempty"`
	}

	msg := expoPushMessage{
		To:        token,
		Title:     "✅ Flatmate Settle Up",
		Body:      fmt.Sprintf("%s recorded a settlement of ₹%.0f with you in %s.", payerName, amount, householdName),
		Sound:     "default",
		Priority:  "high",
		ChannelID: "default",
		Data: map[string]interface{}{
			"type":         "settle_up",
			"household_id": householdID.String(),
		},
	}

	payload, err := json.Marshal([]expoPushMessage{msg})
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://exp.host/--/api/v2/push/send", bytes.NewBuffer(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}
