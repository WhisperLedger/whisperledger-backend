# GCP deployment: Cloud Run + Cloud SQL

This backend is designed to run with Cloud Run, Cloud SQL for PostgreSQL, Artifact Registry, Secret Manager, and Cloud Build. The API and migration job share one immutable container image. Financial data never lives in a container filesystem.

## One-time setup

1. Create separate GCP projects for staging and production, and enable billing with a budget alert. Free credits are appropriate for initial deployment; they are not an availability guarantee.
2. Enable Cloud Run, Cloud Build, Artifact Registry, Secret Manager, Cloud SQL Admin, and Service Usage APIs.
3. Create a PostgreSQL Cloud SQL instance and a `whisperledger` database. Require TLS or use the Cloud SQL connector/private connection.
4. Create a least-privilege runtime service account. It needs only Secret Manager Secret Accessor and Cloud SQL Client. It must not have Owner, Editor, or Firebase Admin.
5. Create these Secret Manager secrets. Do not put any value in Git, Cloud Build substitutions, EAS config, or `EXPO_PUBLIC_*` variables:
   - `whisperledger-database-url`
   - `whisperledger-jwt-secret` (at least 32 random characters)
   - `whisperledger-cors-origins` (exact comma-separated web origins)
6. Create Artifact Registry repository `whisperledger` in your chosen region.

## Deploy

Trigger `cloudbuild.yaml` from `main` after CI succeeds. It builds an immutable image, runs migrations as a Cloud Run Job, waits for success, and only then deploys the API service.

Do not enable the frontend API mode until data migration and API parity are complete. The current application remains in `EXPO_PUBLIC_DATA_BACKEND=firebase` mode, so there is no dual-write risk.

## Operational safeguards

- Cloud Run API starts only if `DATABASE_URL` and a 32+ character `JWT_SECRET` are present.
- Run migrations only through the migration job; API startup never alters schema.
- Keep `min-instances=0` for credit-conscious staging. Set it to `1` for production only after budget approval.
- Restrict CORS to exact app/admin origins and rotate the JWT secret through Secret Manager.
- Take automated Cloud SQL backups before every schema migration.
