-- This subject is set only after a successful, server-verified Firebase ID
-- token exchange for an imported account. It prevents a different Firebase
-- identity from being attached later based only on a matching email address.
ALTER TABLE users ADD COLUMN IF NOT EXISTS firebase_subject VARCHAR(255);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_firebase_subject
    ON users(firebase_subject)
    WHERE firebase_subject IS NOT NULL;
