# AWS migration plan

## Scope and current state

This plan moves the deployed WhisperLedger API and its operational tooling from
Google Cloud to AWS. It does **not** delete GCP resources or migrate user data
until the AWS environment has passed a staged validation and explicit cutover
approval has been given.

The repository shows these current dependencies:

| Current dependency | Evidence | AWS target |
| --- | --- | --- |
| Cloud Run API | `cloudbuild.yaml`, `DEPLOY_GCP.md` | ECS Fargate service behind an Application Load Balancer |
| Cloud Run migration job | `cloudbuild.yaml` | ECS Fargate one-off task, run before each service deployment |
| Artifact Registry | `cloudbuild.yaml` | Amazon ECR |
| Cloud Build | `cloudbuild.yaml` | GitHub Actions with AWS OIDC, or CodeBuild; select one before implementation |
| GCP Secret Manager | `DEPLOY_GCP.md` | AWS Secrets Manager with a task-execution role |
| Cloud SQL or Neon PostgreSQL | deployment documentation | Amazon RDS for PostgreSQL, only if the live `DATABASE_URL` is Cloud SQL/Neon and a database move is approved |
| Cloud Run metadata and console links | `internal/service/admin_service.go`, admin UI | ECS task metadata, CloudWatch links, and AWS console links |
| Firebase Auth and Firestore | mobile client and legacy migration bridge | Separate, later migration work; it is not safely replaced by ECS/RDS alone |

The Go API is already container-portable: it reads `PORT`, `DATABASE_URL`,
`JWT_SECRET`, CORS origins, and feature flags from environment variables. Its
PostgreSQL repositories do not depend on Cloud SQL APIs. The Docker image can
be retained with only an ECS port mapping change.

## Recommended target architecture

Deploy into `ap-south-1` unless latency, residency, or the current database
region requires another region.

```text
Internet
  -> Route 53 / existing DNS
  -> ACM certificate + Application Load Balancer
  -> ECS Fargate service (Go API, private subnets)
  -> RDS PostgreSQL (private subnets)

ECR <- CI build
ECS one-off migration task <- CI release gate
Secrets Manager -> ECS task execution role
CloudWatch Logs, alarms, and dashboards <- ALB / ECS / RDS
```

Use separate AWS accounts or, at minimum, isolated VPCs and IAM roles for
staging and production. Store `DATABASE_URL`, `JWT_SECRET`, CORS origins, and
WhatsApp credentials in Secrets Manager. Inject secrets at task start through
the ECS task definition; do not put them in CI variables, task-definition
plaintext environment values, or mobile bundles.

## Required decisions before provisioning

1. Confirm the live database source: Cloud SQL, Neon, or another PostgreSQL
   provider. Repository documentation mentions both Cloud SQL and Neon, while
   the actual runtime value is intentionally not committed.
2. Choose PostgreSQL target: RDS PostgreSQL is the lowest-risk equivalent;
   Aurora PostgreSQL should be selected only for a demonstrated availability or
   scaling requirement.
3. Choose CI: GitHub Actions with AWS OIDC is the recommended default because
   it avoids long-lived AWS credentials. CodeBuild is acceptable if AWS-native
   CI is preferred.
4. Confirm the public API hostname. Preserve `api.whisperledger.com` (or the
   current production hostname) so mobile binaries do not need an emergency
   release during cutover.
5. Decide whether Firebase is in this migration's scope. The mobile client
   still directly uses Firebase Authentication and Firestore for several
   contexts. Replacing it requires an application migration to the existing API
   and PostgreSQL, not an infrastructure substitution.

## Delivery phases

### 1. Inventory and guardrails

- Export a read-only inventory of the live Cloud Run revision, Artifact
  Registry image digest, Cloud SQL/Neon database version, Firebase projects,
  DNS records, secrets names (never values), domains, and monitoring alerts.
- Record a restore point and validate database backup restoration before any
  data movement.
- Add cost budgets and alarms for ECS, ALB, NAT gateways, RDS, and data
  transfer. NAT gateway cost must be explicitly reviewed; use VPC endpoints
  where appropriate.

### 2. Infrastructure as code

- Add a reviewed IaC stack (Terraform is recommended) for VPC, subnets,
  security groups, ECR, ECS cluster/service/task definitions, ALB, ACM, Route
  53 records, CloudWatch log groups/alarms, Secrets Manager, and RDS.
- Define distinct task execution and task roles. The execution role receives
  only ECR pull, log-write, and specific-secret read permissions. The runtime
  task role gets no AWS permissions unless a new integration needs them.
- Keep the API and database private; the ALB is the only public entry point.

### 3. Delivery pipeline and migrations

- Replace the Cloud Build workflow with a pipeline that runs `go test ./...`
  and `go vet ./...`, builds one immutable ECR image digest, and deploys it to
  staging.
- Run `/app/migrate up` as a separately tracked ECS task. The API service must
  never run schema migrations at startup.
- Health-check the staging API, then update the ECS service only after the
  migration task succeeds.

### 4. Database move (only when required)

- For an existing Cloud SQL or Neon source, choose a planned maintenance
  window for a `pg_dump`/`pg_restore` cutover when the dataset is small and a
  brief write pause is acceptable.
- Use AWS DMS full-load plus change-data-capture only when the source supports
  logical replication and downtime requirements justify the operational
  complexity.
- Reconcile row counts, checksums for financial tables, migration version, and
  application smoke tests before accepting the RDS target. Never dual-write
  ledger data.

### 5. Staged release and cutover

- Deploy an AWS staging environment against a staging database and validate
  registration, password login, Google login, expenses, household splits,
  receivables, recovery, notifications, WhatsApp webhook verification, and
  admin authorization.
- Deploy production ECS with no public DNS change. Exercise the ALB health
  endpoint and a synthetic authenticated API flow.
- During the approved window: pause writes if a database final sync is needed,
  run/reconcile the final sync, update the API DNS record with a low TTL, and
  monitor error rate, latency, database connections, and auth failures.
- Roll back DNS to Cloud Run if the release gate fails. Retain GCP and its
  database read-only until the agreed retention period ends.

### 6. Firebase retirement (separate project)

- The backend's legacy Firebase token bridge and the frontend's direct
  Firestore access mean Firebase cannot be disabled at AWS cutover.
- First migrate every active mobile data path to the API/PostgreSQL contract,
  ship a mobile release, migrate user identity/reset flows, and verify a
  signed Firestore export.
- Only then disable Firebase writes and, later, decommission the project.

## Repository changes anticipated

- Add AWS IaC and an ECS task definition/service configuration.
- Replace `cloudbuild.yaml` with the chosen CI workflow; retain it only while
  rollback to Cloud Run remains supported.
- Replace GCP-specific telemetry fields and Cloud Run console links in
  `internal/service/admin_service.go` and both admin interfaces.
- Update hard-coded Cloud Run API origins and CSP `connect-src` entries in the
  web/admin projects to the stable custom API hostname.
- Refresh the GCP deployment blueprints after AWS staging is proven.

## Release gates

- No production data copy without a verified, restorable backup.
- No DNS cutover without a successful staging release, migration task,
  application smoke test, and observed healthy production target.
- No Firebase shutdown as part of the infrastructure cutover.
- No GCP deletion until rollback retention and financial-data reconciliation
  are complete.
