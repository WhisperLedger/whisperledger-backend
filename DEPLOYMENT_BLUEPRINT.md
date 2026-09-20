# WhisperLedger: Production & Free-Tier Deployment Blueprint

> **Complete Zero-Cost to Cloud Scale Infrastructure Guide**  
> Go 1.24/1.25 API · PostgreSQL (Neon / Cloud SQL) · React/Next Web · React Native (Expo) Android/iOS

---

## 📑 Table of Contents
1. [Architecture Overview](#1-architecture-overview)
2. [Phase 1: 100% Free-Tier Setup (₹0 / Month)](#2-phase-1-100-free-tier-setup-0--month)
3. [Go Backend on Render Free](#3-go-backend-on-render-free)
4. [PostgreSQL on Neon Free](#4-postgresql-on-neon-free)
5. [Web Dashboard & Landing Page Deployment](#5-web-dashboard--landing-page-deployment)
6. [Mobile App Build via Expo EAS & Local Gradle](#6-mobile-app-build-via-expo-eas--local-gradle)
7. [GitHub Actions CI/CD Across All Repositories](#7-github-actions-cicd-across-all-repositories)
8. [Phase 2: Seamless Zero-Code Migration to Google Cloud Platform (GCP)](#8-phase-2-seamless-zero-code-migration-to-google-cloud-platform-gcp)
9. [Pre-Release Verification Checklist (10/10)](#9-pre-release-verification-checklist-1010)

---

## 1. Architecture Overview

WhisperLedger is split into three decoupled, independently deployable services:

```mermaid
flowchart TD
    subgraph Clients["Clients Layer"]
        Mobile["📱 Mobile App (React Native/Expo)\nAPK / AAB"]
        WebLanding["🌐 WhisperLedger Web\nLanding Page & Admin Console"]
    end

    subgraph HostingPhase1["Phase 1: Free Tier"]
        Render["Render Web Service (Go Docker)\nPort 10000 • /healthz"]
        Neon[("Neon Serverless PostgreSQL\n0.5 GB • 100 CU hrs")]
    end

    subgraph HostingPhase2["Phase 2: Google Cloud Platform"]
        CloudRunAPI["Google Cloud Run (Go API)\nAuto-scaling 0 to N"]
        CloudRunWeb["Google Cloud Run (Web)\nContainerized"]
        CloudSQL[("Google Cloud SQL / Neon\nPostgreSQL 16")]
    end

    Mobile -->|REST API| Render
    WebLanding -->|REST API| Render
    Render --> Neon

    Mobile -.->|Switch DNS| CloudRunAPI
    WebLanding -.-> CloudRunWeb
    CloudRunAPI -.-> CloudSQL
```

---

## 2. Phase 1: 100% Free-Tier Setup (₹0 / Month)

| Component | Provider / Service | Free Allowance Details |
|---|---|---|
| **Go REST API** | **Render Free Web Service** | 512 MB RAM, spins down after 15m inactivity, 750 free hrs/mo. |
| **PostgreSQL** | **Neon Free Tier** | 0.5 GB storage, 100 compute-unit hours/mo, SSL pooling. |
| **Web Portal** | **Cloudflare / Render / Cloud Run** | 100,000 requests/day, instant global CDN edge. |
| **Mobile App** | **Expo EAS / Local Gradle** | Free internal preview APKs, standalone local builds. |
| **CI/CD** | **GitHub Actions** | 2,000 free runner minutes per month. |

---

## 3. Go Backend on Render Free

### Configuration Files Present
- [`Dockerfile`](file:///Users/tanmayagarwal/TanmayProjects/whisperledger-backend/Dockerfile): Multi-stage, non-root Alpine container (`EXPOSE 10000`).
- [`.dockerignore`](file:///Users/tanmayagarwal/TanmayProjects/whisperledger-backend/.dockerignore): Ignores `.git`, `.env`, `node_modules`, `bin`.
- [`render.yaml`](file:///Users/tanmayagarwal/TanmayProjects/whisperledger-backend/render.yaml): Blueprint definition with health check at `/healthz`.

### Step-by-Step Deployment
1. Push `whisperledger-backend` to your GitHub organization (`pitcher/whisperledger-backend`).
2. Go to **[Render Dashboard](https://dashboard.render.com)** -> Click **New +** -> **Blueprint**.
3. Select the repository. Render will automatically parse `render.yaml`.
4. Enter your secrets:
   - `DATABASE_URL`: `postgresql://USER:PASSWORD@HOST/DB?sslmode=require` (from Neon).
   - `JWT_SECRET`: High-entropy 32+ character random string.
   - `CORS_ORIGINS`: `https://whisperledger-web.pages.dev,https://whisperledger.com`.
5. Click **Apply**. Once live, test the endpoint:
   ```bash
   curl -i https://YOUR-API.onrender.com/healthz
   ```

---

## 4. PostgreSQL on Neon Free

1. Open **[Neon Console](https://console.neon.tech)** and create a new project named `whisperledger-staging`.
2. Select PostgreSQL 16 in your preferred region (e.g. `ap-southeast-1` or `eu-central-1`).
3. Copy the **Pooled Connection String**:
   ```
   postgresql://alex:AbCdEf123@ep-cool-fog-123456-pooler.region.aws.neon.tech/neondb?sslmode=require
   ```
4. Run migrations from your terminal:
   ```bash
   DATABASE_URL="your-neon-connection-string" go run cmd/migrate/main.go up
   ```
5. Connection pool in [`internal/repository/postgres/db.go`](file:///Users/tanmayagarwal/TanmayProjects/whisperledger-backend/internal/repository/postgres/db.go) is automatically configured with:
   - `MaxConns = 10`
   - `MinConns = 2`
   - `MaxConnLifetime = 30m`

---

## 5. Web Dashboard & Landing Page Deployment

### Repository: `pitcher/whisperledger-web`
- Contains the unified React 18 / TypeScript / Vite / Tailwind web app.
- Multi-stage [`Dockerfile`](file:///Users/tanmayagarwal/TanmayProjects/whisperledger-web/Dockerfile) + [`nginx.conf`](file:///Users/tanmayagarwal/TanmayProjects/whisperledger-web/nginx.conf) for production containerization.

### Option A: Cloudflare Pages / Workers
```bash
cd whisperledger-web
npm install
npm run build
npx wrangler pages deploy dist --project-name=whisperledger-web
```

### Option B: Render Web Service (Docker)
Deploy as a Docker web service using the root `Dockerfile`.

---

## 6. Mobile App Build via Expo EAS & Local Gradle

### Repository: `pitcher/whisperledger-frontend`

### Option A: Cloud EAS Preview Build (APK)
```bash
# Login to Expo
npx eas-cli login

# Trigger internal testing APK build
npx eas-cli build --platform android --profile preview
```

### Option B: Zero-Cost Local Gradle Build (No Cloud Queue)
```bash
cd whisperledger-frontend/android
./gradlew assembleRelease
```
The output APK is generated at:
`android/app/build/outputs/apk/release/app-release.apk`

---

## 7. GitHub Actions CI/CD Across All Repositories

### Repository Secrets Summary
| Secret / Variable Name | Where to Add | Purpose |
|---|---|---|
| `DATABASE_URL` | Render / GitHub | PostgreSQL connection string |
| `JWT_SECRET` | Render / GitHub | Token signing key |
| `EXPO_TOKEN` | `whisperledger-frontend` Secrets | Automated EAS builds |
| `NEXT_PUBLIC_API_URL` | `whisperledger-web` Variables | Backend API base URL |

### CI Workflow Files:
- Backend: `.github/workflows/ci.yml` (Tests Go code, builds binary, validates Docker build).
- Web: `.github/workflows/deploy.yml` (Runs typecheck, Vite build, validates container).
- Mobile: `.github/workflows/mobile.yml` (Runs `tsc --noEmit`, validates Expo configuration).

---

## 8. Phase 2: Seamless Zero-Code Migration to Google Cloud Platform (GCP)

When ready to use your **$300 Google Cloud free trial credits** or startup grant:

### Why Zero Code Changes Are Required:
1. **Container Port Portability**: The Go backend binds to `0.0.0.0:$PORT`, working identically on Render ($PORT=10000) and Google Cloud Run ($PORT=8080).
2. **Standard SQL**: Pure PostgreSQL syntax without proprietary extensions or vendor lock-in.
3. **Stable Domain Indirection**: Point your mobile app to a custom domain (e.g. `https://api.whisperledger.com`). When migrating from Render to Cloud Run, simply update the DNS CNAME record; no new mobile app release is needed!

### Cloud Run Deployment Commands:
```bash
# 1. Build & Push image to Google Artifact Registry
gcloud builds submit --tag gcr.io/YOUR_PROJECT_ID/whisperledger-backend

# 2. Deploy to Cloud Run (min-instances=0 for zero idle cost)
gcloud run deploy whisperledger-backend \
  --image gcr.io/YOUR_PROJECT_ID/whisperledger-backend \
  --platform managed \
  --region asia-south1 \
  --allow-unauthenticated \
  --min-instances 0 \
  --max-instances 10 \
  --set-env-vars APP_ENV=production,DATABASE_URL="your_db_url",JWT_SECRET="your_jwt_secret"
```

---

## 9. Pre-Release Verification Checklist (10/10)

- [ ] 1. Go API health check returns HTTP 200 at `/health` and `/healthz`.
- [ ] 2. Database migrations applied (`000001_init.up.sql`).
- [ ] 3. User authentication creates valid access & refresh token pair.
- [ ] 4. Mobile app can fetch transactions over HTTPS.
- [ ] 5. Web dashboard loads and authenticates admin users.
- [ ] 6. RBAC prevents non-admin users from accessing `/api/v1/admin/*`.
- [ ] 7. Household minimum-cash-flow graph solver simplifies tangled debts.
- [ ] 8. CORS permits only designated web origins.
- [ ] 9. No raw OTPs or financial SMS texts leaked in logs.
- [ ] 10. Database backup script verified.
