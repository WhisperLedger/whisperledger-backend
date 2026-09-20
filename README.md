# WhisperLedger Backend API Service

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go)](https://golang.org)
[![Database](https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql)](https://postgresql.org)
[![Architecture](https://img.shields.io/badge/Architecture-Clean%20Hexagonal-brightgreen)](#architecture--hexagonal-design)
[![Security](https://img.shields.io/badge/Security-Argon2%20%2B%20JWT%20RBAC-critical)](#security--authentication)

The core enterprise backend service powering **WhisperLedger** — the autonomous money operating system for shared and personal living. Engineered in **Golang** using Clean / Hexagonal Architecture, high-throughput connection pooling via `pgxpool`, and strict cryptographic security.

---

## 🚀 Core Innovations & Capabilities

1. **3-Way Outflow Ledger**
   - **True Personal Outflow**: Direct expenditures that deplete personal net worth.
   - **Shared Household Outflow**: Common bills split dynamically across flatmates.
   - **100% Recoverable Outflow**: Fronted amounts (friend loans, vendor refunds, security deposits) tracked as receivables rather than false budget consumption.

2. **Minimum-Cash-Flow Debt Graph Solver**
   - Resolves multi-party circular debts within a household.
   - Transforms $N$ tangled pairwise debts into the mathematical minimum number of direct transfers, neutralizing redundant payments.

3. **Money Recovery Engine**
   - Comprehensive ledger for tracking lent capital, advance payments, and pending merchant refunds.
   - Computes overdue statuses past due dates with automated reminder metadata.

4. **Money Leak Detective & Safe-to-Spend Autopilot**
   - Detects duplicate merchant charges within 24-hour windows.
   - Computes true available daily spendable budget after shielding fronted obligations and reserving for fixed living costs.

5. **Executive Admin Telemetry**
   - Real-time aggregation of platform volume, transaction throughput, active household clusters, and system diagnostics.

---

## 🏛 Architecture & Hexagonal Design

```
whisperledger-backend/
├── cmd/
│   ├── api/main.go            # Production HTTP server entrypoint & graceful shutdown
│   └── migrate/main.go        # Database schema migration runner (up / down)
├── internal/
│   ├── config/config.go       # Strongly-typed environment configuration
│   ├── domain/                # Pure business entities & repository interfaces (0 dependencies)
│   │   ├── user.go            # User entity & UserRepository interface
│   │   ├── expense.go         # Expense, OutflowType & ExpenseRepository interface
│   │   ├── household.go       # Household, Member, DebtResolution & HouseholdRepository interface
│   │   ├── receivable.go      # Receivable entity & ReceivableRepository interface
│   │   └── audit.go           # LeakAlert, HouseholdAudit & AdminPlatformMetrics
│   ├── pkg/                   # Reusable cross-cutting packages
│   │   ├── hash/password.go   # Argon2 / Bcrypt secure password hashing
│   │   ├── token/jwt.go       # HMAC-SHA256 JWT Token Maker (access + refresh pairs)
│   │   └── response/response.go# Standardized JSON response envelopes & error codes
│   ├── repository/postgres/   # Data persistence using pgxpool
│   │   ├── db.go              # Database connection pool manager
│   │   ├── user_repo.go       # PostgreSQL user queries
│   │   ├── expense_repo.go    # PostgreSQL expense queries & 3-way aggregations
│   │   ├── household_repo.go  # Household queries & net balance aggregations
│   │   └── receivable_repo.go # Receivable queries & settlement tracking
│   ├── service/               # Application use-case orchestrators
│   │   ├── auth_service.go    # Authentication, login, register, token refresh
│   │   ├── ledger_service.go  # 3-Way Outflow computation & expense recording
│   │   ├── household_service.go # Minimum-cash-flow graph resolution algorithm
│   │   ├── recovery_service.go# Receivable creation & settlement
│   │   ├── detective_service.go# Duplicate charge detector & safe-to-spend autopilot
│   │   └── admin_service.go   # Platform metrics & user directory queries
│   ├── middleware/            # HTTP middlewares
│   │   ├── auth_middleware.go # JWT extraction, Context injection & RBAC enforcement
│   │   └── logger_middleware.go# Zap structured request logging
│   └── handler/
│       ├── router.go          # Chi router setup, CORS, timeout, and route groups
│       └── v1/                # Versioned HTTP controllers
│           ├── auth_handler.go
│           ├── expense_handler.go
│           ├── household_handler.go
│           ├── recovery_handler.go
│           ├── detective_handler.go
│           └── admin_handler.go
├── migrations/
│   ├── 000001_init.up.sql     # PostgreSQL tables, relations, and performance indexes
│   └── 000001_init.down.sql   # Rollback migrations
├── Dockerfile                 # Multi-stage optimized Alpine container
├── docker-compose.yml         # Local orchestration (API + PostgreSQL 16)
├── go.mod
└── go.sum
```

---

## 📡 REST API Reference

All API endpoints are versioned under `/api/v1`.

### 1. Authentication (`/api/v1/auth`)
| Method | Endpoint | Description | Auth Required |
|---|---|---|---|
| `POST` | `/api/v1/auth/register` | Register new user account | No |
| `POST` | `/api/v1/auth/login` | Authenticate and obtain JWT token pair | No |
| `POST` | `/api/v1/auth/refresh` | Refresh access token using refresh token | No |
| `GET` | `/api/v1/auth/me` | Fetch authenticated user profile | Yes (Bearer) |

### 2. 3-Way Outflow Ledger (`/api/v1/expenses`)
| Method | Endpoint | Description | Auth Required |
|---|---|---|---|
| `POST` | `/api/v1/expenses` | Record an expense with 3-way outflow splitting | Yes |
| `GET` | `/api/v1/expenses` | List expenses (filterable by date, category, type) | Yes |
| `GET` | `/api/v1/expenses/summary` | Get aggregated 3-way outflow breakdown | Yes |
| `GET` | `/api/v1/expenses/{id}` | Get specific expense details and splits | Yes |
| `DELETE` | `/api/v1/expenses/{id}` | Delete an expense entry | Yes |

### 3. Households & Debt Graph (`/api/v1/households`)
| Method | Endpoint | Description | Auth Required |
|---|---|---|---|
| `POST` | `/api/v1/households` | Create a new household cluster with invite code | Yes |
| `POST` | `/api/v1/households/join` | Join household via unique alphanumeric invite code | Yes |
| `GET` | `/api/v1/households/me` | Get current user's active household | Yes |
| `GET` | `/api/v1/households/{id}/balances` | Get raw net balance per household member | Yes |
| `GET` | `/api/v1/households/{id}/settlements` | Run **Minimum-Cash-Flow** solver to get optimal settlements | Yes |
| `POST` | `/api/v1/households/{id}/settle` | Record a debt settlement between two members | Yes |

### 4. Money Recovery Engine (`/api/v1/recovery`)
| Method | Endpoint | Description | Auth Required |
|---|---|---|---|
| `POST` | `/api/v1/recovery` | Create a new receivable (loan / refund / deposit) | Yes |
| `GET` | `/api/v1/recovery` | List receivables (filterable by status) | Yes |
| `GET` | `/api/v1/recovery/summary`| Total pending recoverable capital | Yes |
| `POST` | `/api/v1/recovery/{id}/settle` | Record full or partial recovery payment | Yes |

### 5. Money Leak Detective (`/api/v1/detective`)
| Method | Endpoint | Description | Auth Required |
|---|---|---|---|
| `GET` | `/api/v1/detective/leaks` | Scan for duplicate vendor debits & overdue receivables | Yes |
| `GET` | `/api/v1/detective/safe-to-spend` | Calculate dynamic daily safe-to-spend budget | Yes |
| `GET` | `/api/v1/detective/household-audit` | Generate monthly household financial audit report | Yes |

### 6. Executive Admin Portal (`/api/v1/admin`)
*Guarded by `platform_admin` or `household_admin` roles.*
| Method | Endpoint | Description | Auth Required |
|---|---|---|---|
| `GET` | `/api/v1/admin/metrics` | Platform overview (users, volume, receivables) | Yes (Admin) |
| `GET` | `/api/v1/admin/users` | Paginated user management directory | Yes (Admin) |

---

## ⚙️ Environment Variables

Create a `.env` file in the root directory:

```env
APP_ENV=development
PORT=8080
DATABASE_URL=postgres://postgres:password123@localhost:5432/whisperledger?sslmode=disable
JWT_SECRET=your-secure-jwt-secret-key-at-least-32-chars
JWT_ACCESS_EXPIRY_HOURS=24
JWT_REFRESH_EXPIRY_DAYS=30
```

---

## 🛠 Local Development & Testing

### 1. Prerequisites
- **Go 1.24+** installed.
- **PostgreSQL 16+** running locally or via Docker.

### 2. Run Database Migrations
```bash
go run cmd/migrate/main.go up
```

### 3. Build & Run the API Server
```bash
# Build binary
go build -o bin/api ./cmd/api

# Run server
./bin/api
```

The server will listen at `http://localhost:8080`. Verify system health:
```bash
curl http://localhost:8080/health
```

### 4. Execute Test Suite
```bash
go test -v ./...
```

---

## 🐳 Docker Deployment

To launch the full backend stack (API + PostgreSQL):

```bash
docker-compose up -d --build
```

To stop:
```bash
docker-compose down
```
