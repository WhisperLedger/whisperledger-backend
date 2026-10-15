-- Email addresses collected from the public marketing site for product availability updates.
-- Keep this isolated from account data: an interest signup is not a product account.
CREATE TABLE IF NOT EXISTS interest_signups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(254) NOT NULL UNIQUE,
    source VARCHAR(32) NOT NULL DEFAULT 'website',
    consented_at TIMESTAMPTZ NOT NULL,
    unsubscribed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT interest_signups_email_lowercase CHECK (email = LOWER(email))
);

CREATE INDEX IF NOT EXISTS idx_interest_signups_active
    ON interest_signups (created_at DESC)
    WHERE unsubscribed_at IS NULL;
