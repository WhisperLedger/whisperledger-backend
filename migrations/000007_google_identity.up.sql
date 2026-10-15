-- A Google subject is a stable, provider-issued identifier. It is deliberately
-- separate from email, which can be changed by a user or email provider.
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_subject VARCHAR(255);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_google_subject
    ON users(google_subject)
    WHERE google_subject IS NOT NULL;
