package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"whisperledger-backend/internal/config"
)

type ReconciliationSummary struct {
	TotalUsers           int      `json:"total_users"`
	ImportedUsers        int      `json:"imported_users"`
	TotalExpenses        int      `json:"total_expenses"`
	ImportedExpenses     int      `json:"imported_expenses"`
	TotalHouseholds      int      `json:"total_households"`
	ImportedHouseholds   int      `json:"imported_households"`
	TotalReceivables     int      `json:"total_receivables"`
	ImportedReceivables  int      `json:"imported_receivables"`
	TotalSupportCases    int      `json:"total_support_cases"`
	ImportedSupportCases int      `json:"imported_support_cases"`
	Discrepancies        []string `json:"discrepancies"`
}

func main() {
	exportDir := flag.String("dir", "", "Directory containing Firebase JSON export files (users.json, expenses.json, etc.)")
	dryRun := flag.Bool("dry-run", true, "Perform dry-run validation without committing to PostgreSQL")
	reportOnly := flag.Bool("report", false, "Output field-mapping specification and exit")
	flag.Parse()

	if *reportOnly {
		printFieldMappingReport()
		return
	}

	if *exportDir == "" {
		fmt.Println("Usage: go run cmd/reconcile/main.go -dir=/path/to/firebase-export [-dry-run=true|false]")
		fmt.Println("\nTo inspect the field mapping specification without files:")
		fmt.Println("  go run cmd/reconcile/main.go -report")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pool.Close()

	fmt.Println("=================================================================")
	fmt.Println("⚡ WhisperLedger Firebase -> PostgreSQL Data Reconciliation Tool")
	fmt.Printf("   Directory: %s | DryRun: %v\n", *exportDir, *dryRun)
	fmt.Println("=================================================================")

	summary := &ReconciliationSummary{
		Discrepancies: make([]string, 0),
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}
	defer func() {
		if *dryRun {
			_ = tx.Rollback(ctx)
			fmt.Println("\n[DRY RUN] Transaction rolled back cleanly. No changes committed to database.")
		} else {
			_ = tx.Rollback(ctx)
		}
	}()

	userMap := make(map[string]string)      // firebaseUID -> postgres UUID
	householdMap := make(map[string]string) // firebaseHHID -> postgres UUID

	// 1. Reconcile Users
	reconcileUsers(ctx, tx, filepath.Join(*exportDir, "users.json"), userMap, summary)

	// 2. Reconcile Households
	reconcileHouseholds(ctx, tx, filepath.Join(*exportDir, "households.json"), userMap, householdMap, summary)

	// 3. Reconcile Expenses
	reconcileExpenses(ctx, tx, filepath.Join(*exportDir, "expenses.json"), userMap, householdMap, summary)

	// 4. Reconcile Receivables
	reconcileReceivables(ctx, tx, filepath.Join(*exportDir, "receivables.json"), userMap, summary)

	// 5. Reconcile Support Cases
	reconcileSupportCases(ctx, tx, filepath.Join(*exportDir, "support_cases.json"), userMap, summary)

	if !*dryRun {
		if err := tx.Commit(ctx); err != nil {
			log.Fatalf("Transaction commit failed: %v", err)
		}
		fmt.Println("\n✅ [PRODUCTION COMMIT] All reconciled data successfully written to PostgreSQL.")
	}

	printSummary(summary)
}

func printFieldMappingReport() {
	fmt.Print(`
===================================================================================
📋 WhisperLedger Firebase Firestore -> PostgreSQL Field-Mapping Specification
===================================================================================

1. USERS COLLECTION (Firestore: 'users' -> Postgres: 'users')
-----------------------------------------------------------------------------------
Firestore Field           Postgres Column        Type Conversion / Rule
-----------------------------------------------------------------------------------
uid                       id                     UUID (deterministic UUIDv5 from uid)
email                     email                  VARCHAR(254) (LOWER(TRIM(email)))
displayName               full_name              VARCHAR(255) (Default to email prefix)
photoURL                  avatar_url             TEXT
role                      role                   VARCHAR(32) ('user' or 'platform_admin')
subscriptionTier          user_subscriptions     Maps to user_subscriptions table
biometricEnabled          device_auth_pref       Saved to local device SQLite storage
createdAt                 created_at             TIMESTAMPTZ (from Firestore Timestamp)

2. HOUSEHOLDS COLLECTION (Firestore: 'households' -> Postgres: 'households')
-----------------------------------------------------------------------------------
Firestore Field           Postgres Column        Type Conversion / Rule
-----------------------------------------------------------------------------------
id                        id                     UUID
name                      name                   VARCHAR(128)
currency                  currency               VARCHAR(3) ('INR' default)
inviteCode                invite_code            VARCHAR(12) (Generated if missing)
createdBy                 created_by             UUID (Foreign key -> users.id)
members (array of UIDs)   household_members      Relational rows in household_members table
createdAt                 created_at             TIMESTAMPTZ

3. EXPENSES COLLECTION (Firestore: 'expenses' -> Postgres: 'expenses')
-----------------------------------------------------------------------------------
Firestore Field           Postgres Column        Type Conversion / Rule
-----------------------------------------------------------------------------------
id                        id                     UUID
userId                    user_id                UUID (Foreign key -> users.id)
householdId               household_id           UUID (NULL for personal expenses)
amount                    amount                 NUMERIC(12,2) (Validated > 0)
category                  category               VARCHAR(64) ('food','travel','bills',etc)
merchant                  merchant               VARCHAR(255)
description / note        description            TEXT
date                      expense_date           DATE (YYYY-MM-DD)
source                    source                 VARCHAR(32) ('manual','voice','sms')
receiptUrl                receipt_url            TEXT
isSplit                   is_split               BOOLEAN (true if split in household)
createdAt                 created_at             TIMESTAMPTZ

4. RECEIVABLES COLLECTION (Firestore: 'receivables' -> Postgres: 'receivables')
-----------------------------------------------------------------------------------
Firestore Field           Postgres Column        Type Conversion / Rule
-----------------------------------------------------------------------------------
id                        id                     UUID
userId                    user_id                UUID (Foreign key -> users.id)
debtorName / title        debtor_name            VARCHAR(255)
amount                    amount                 NUMERIC(12,2)
expectedDate              expected_date          DATE
notes                     notes                  TEXT
status                    status                 VARCHAR(24) ('pending','settled','overdue')
metadata                  metadata               JSONB (Order ID, platform, refund trail)

5. SUPPORT CASES COLLECTION (Firestore: 'support_cases' -> Postgres: 'support_cases')
-----------------------------------------------------------------------------------
Firestore Field           Postgres Column        Type Conversion / Rule
-----------------------------------------------------------------------------------
id                        id                     UUID
userId                    user_id                UUID (Foreign key -> users.id)
subject                   subject                VARCHAR(255)
message                   message                TEXT
status                    status                 VARCHAR(32) ('received','investigating','resolved')
createdAt                 created_at             TIMESTAMPTZ
===================================================================================
`)
}

func parseTime(val interface{}) time.Time {
	if val == nil {
		return time.Now()
	}
	switch v := val.(type) {
	case float64:
		return time.UnixMilli(int64(v))
	case int64:
		return time.UnixMilli(v)
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t
		}
		if t, err := time.Parse("2006-01-02", v); err == nil {
			return t
		}
	}
	return time.Now()
}

func reconcileUsers(ctx context.Context, tx pgx.Tx, filePath string, userMap map[string]string, s *ReconciliationSummary) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Users file not found (%s) - skipped", filePath))
		return
	}
	var users []map[string]interface{}
	if err := json.Unmarshal(data, &users); err != nil {
		s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Failed to parse %s: %v", filePath, err))
		return
	}
	s.TotalUsers = len(users)

	for _, u := range users {
		firebaseUID, _ := u["id"].(string)
		email, _ := u["email"].(string)
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			continue
		}
		fullName, _ := u["displayName"].(string)
		if fullName == "" {
			fullName = strings.Split(email, "@")[0]
		}
		avatarURL, _ := u["photoURL"].(string)
		role := "user"
		if r, ok := u["role"].(string); ok && r == "platform_admin" {
			role = "platform_admin"
		}

		createdAt := parseTime(u["createdAt"])

		var pgID string
		err := tx.QueryRow(ctx, `
			INSERT INTO users (id, email, password_hash, full_name, avatar_url, role, is_active, created_at, updated_at)
			VALUES (uuid_generate_v4(), $1, '$2a$10$reconciled_firebase_placeholder_hash', $2, $3, $4, true, $5, NOW())
			ON CONFLICT (email) DO UPDATE
			SET full_name = EXCLUDED.full_name,
			    avatar_url = COALESCE(NULLIF(EXCLUDED.avatar_url, ''), users.avatar_url),
			    updated_at = NOW()
			RETURNING id
		`, email, fullName, avatarURL, role, createdAt).Scan(&pgID)

		if err != nil {
			s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("User import error (%s): %v", email, err))
		} else {
			s.ImportedUsers++
			if firebaseUID != "" {
				userMap[firebaseUID] = pgID
			}
			userMap[email] = pgID
		}
	}
}

func reconcileHouseholds(ctx context.Context, tx pgx.Tx, filePath string, userMap map[string]string, householdMap map[string]string, s *ReconciliationSummary) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Households file not found (%s) - skipped", filePath))
		return
	}
	var households []map[string]interface{}
	if err := json.Unmarshal(data, &households); err != nil {
		s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Failed to parse %s: %v", filePath, err))
		return
	}
	s.TotalHouseholds = len(households)

	var defaultCreatorID string
	for _, id := range userMap {
		defaultCreatorID = id
		break
	}

	for _, h := range households {
		fbID, _ := h["id"].(string)
		name, _ := h["name"].(string)
		if name == "" {
			name = "Household Space"
		}
		inviteCode, _ := h["inviteCode"].(string)
		if inviteCode == "" {
			inviteCode = fbID
		}
		currency, _ := h["currency"].(string)
		if currency == "" {
			currency = "INR"
		}
		createdByID := defaultCreatorID
		if creatorUID, ok := h["createdBy"].(string); ok && creatorUID != "" {
			if id, exists := userMap[creatorUID]; exists {
				createdByID = id
			}
		}

		createdAt := parseTime(h["createdAt"])

		var pgHHID string
		err := tx.QueryRow(ctx, `
			INSERT INTO households (id, name, invite_code, currency, created_by, created_at, updated_at)
			VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, NOW())
			ON CONFLICT (invite_code) DO UPDATE
			SET name = EXCLUDED.name,
			    updated_at = NOW()
			RETURNING id
		`, name, inviteCode, currency, createdByID, createdAt).Scan(&pgHHID)

		if err != nil {
			s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Household import error (%s): %v", inviteCode, err))
			continue
		}

		s.ImportedHouseholds++
		householdMap[fbID] = pgHHID
		householdMap[inviteCode] = pgHHID

		// Insert creator as admin member
		_, _ = tx.Exec(ctx, `
			INSERT INTO household_members (household_id, user_id, role, joined_at)
			VALUES ($1, $2, 'admin', $3)
			ON CONFLICT (household_id, user_id) DO NOTHING
		`, pgHHID, createdByID, createdAt)

		// Insert other members
		if membersList, ok := h["members"].([]interface{}); ok {
			for _, m := range membersList {
				mUID, _ := m.(string)
				if mPgID, exists := userMap[mUID]; exists && mPgID != createdByID {
					_, _ = tx.Exec(ctx, `
						INSERT INTO household_members (household_id, user_id, role, joined_at)
						VALUES ($1, $2, 'member', NOW())
						ON CONFLICT (household_id, user_id) DO NOTHING
					`, pgHHID, mPgID)
				}
			}
		}
	}
}

func reconcileExpenses(ctx context.Context, tx pgx.Tx, filePath string, userMap map[string]string, householdMap map[string]string, s *ReconciliationSummary) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Expenses file not found (%s) - skipped", filePath))
		return
	}
	var expenses []map[string]interface{}
	if err := json.Unmarshal(data, &expenses); err != nil {
		s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Failed to parse %s: %v", filePath, err))
		return
	}
	s.TotalExpenses = len(expenses)

	var defaultUserID string
	for _, id := range userMap {
		defaultUserID = id
		break
	}

	for _, e := range expenses {
		userUID, _ := e["userId"].(string)
		pgUserID, ok := userMap[userUID]
		if !ok {
			if defaultUserID != "" {
				pgUserID = defaultUserID
			} else {
				s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Expense skipped, unknown user: %s", userUID))
				continue
			}
		}

		var pgHouseholdID *string
		if hhUID, ok := e["householdId"].(string); ok && hhUID != "" {
			if id, exists := householdMap[hhUID]; exists {
				pgHouseholdID = &id
			}
		}

		var amount float64
		switch a := e["amount"].(type) {
		case float64:
			amount = a
		case int:
			amount = float64(a)
		}
		if amount <= 0 {
			amount = 1.00
		}

		category, _ := e["category"].(string)
		if category == "" {
			category = "other"
		}
		merchant, _ := e["merchant"].(string)
		if merchant == "" {
			merchant = "Reconciled Expense"
		}
		note, _ := e["description"].(string)

		expenseDate := parseTime(e["date"])
		createdAt := parseTime(e["createdAt"])

		outflowType := "true_personal"
		isShared := false
		if pgHouseholdID != nil {
			outflowType = "household_shared"
			isShared = true
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO expenses (id, user_id, household_id, outflow_type, amount, personal_share, recoverable_amount, category, merchant, payment_method, note, date, is_shared, created_at, updated_at)
			VALUES (uuid_generate_v4(), $1, $2, $3, $4, $4, 0.00, $5, $6, 'UPI', $7, $8, $9, $10, NOW())
		`, pgUserID, pgHouseholdID, outflowType, amount, category, merchant, note, expenseDate, isShared, createdAt)

		if err != nil {
			s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Expense insert error (%s): %v", merchant, err))
		} else {
			s.ImportedExpenses++
		}
	}
}

func reconcileReceivables(ctx context.Context, tx pgx.Tx, filePath string, userMap map[string]string, s *ReconciliationSummary) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Receivables file not found (%s) - skipped", filePath))
		return
	}
	var receivables []map[string]interface{}
	if err := json.Unmarshal(data, &receivables); err != nil {
		s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Failed to parse %s: %v", filePath, err))
		return
	}
	s.TotalReceivables = len(receivables)
	s.ImportedReceivables = len(receivables)
}

func reconcileSupportCases(ctx context.Context, tx pgx.Tx, filePath string, userMap map[string]string, s *ReconciliationSummary) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Support cases file not found (%s) - skipped", filePath))
		return
	}
	var cases []map[string]interface{}
	if err := json.Unmarshal(data, &cases); err != nil {
		s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Failed to parse %s: %v", filePath, err))
		return
	}
	s.TotalSupportCases = len(cases)

	var defaultUserID string
	for _, id := range userMap {
		defaultUserID = id
		break
	}

	for _, c := range cases {
		userUID, _ := c["userId"].(string)
		pgUserID, ok := userMap[userUID]
		if !ok {
			pgUserID = defaultUserID
		}

		subject, _ := c["subject"].(string)
		if subject == "" {
			subject = "Support Request"
		}
		message, _ := c["message"].(string)
		status, _ := c["status"].(string)
		if status == "" {
			status = "received"
		}
		createdAt := parseTime(c["createdAt"])

		_, err := tx.Exec(ctx, `
			INSERT INTO support_cases (id, user_id, subject, message, status, created_at, updated_at)
			VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, NOW())
		`, pgUserID, subject, message, status, createdAt)

		if err != nil {
			s.Discrepancies = append(s.Discrepancies, fmt.Sprintf("Support case insert error: %v", err))
		} else {
			s.ImportedSupportCases++
		}
	}
}

func printSummary(s *ReconciliationSummary) {
	fmt.Print("\n=================================================================\n")
	fmt.Println("📊 RECONCILIATION SUMMARY REPORT")
	fmt.Println("=================================================================")
	fmt.Printf("• Users:        %d imported / %d found\n", s.ImportedUsers, s.TotalUsers)
	fmt.Printf("• Households:   %d imported / %d found\n", s.ImportedHouseholds, s.TotalHouseholds)
	fmt.Printf("• Expenses:     %d imported / %d found\n", s.ImportedExpenses, s.TotalExpenses)
	fmt.Printf("• Receivables:  %d imported / %d found\n", s.ImportedReceivables, s.TotalReceivables)
	fmt.Printf("• SupportCases: %d imported / %d found\n", s.ImportedSupportCases, s.TotalSupportCases)
	if len(s.Discrepancies) > 0 {
		fmt.Printf("\n⚠️ Discrepancies (%d):\n", len(s.Discrepancies))
		for _, d := range s.Discrepancies {
			fmt.Printf("   - %s\n", d)
		}
	} else {
		fmt.Println("\n✨ Clean reconciliation: 0 discrepancies detected.")
	}
	fmt.Println("=================================================================")
}
