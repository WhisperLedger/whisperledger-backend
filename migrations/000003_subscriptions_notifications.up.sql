-- Add user subscription plan and status columns
ALTER TABLE users ADD COLUMN IF NOT EXISTS plan VARCHAR(32) DEFAULT 'starter_free';
ALTER TABLE users ADD COLUMN IF NOT EXISTS status VARCHAR(32) DEFAULT 'active';
ALTER TABLE users ADD COLUMN IF NOT EXISTS push_token TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS push_ready BOOLEAN DEFAULT false;

-- Create broadcast notifications history table
CREATE TABLE IF NOT EXISTS broadcast_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_name VARCHAR(64) DEFAULT 'WhisperLedger',
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    image_url TEXT,
    target VARCHAR(32) NOT NULL DEFAULT 'all',
    target_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'system',
    delivered_count INT DEFAULT 0,
    dispatch_status VARCHAR(32) DEFAULT 'delivered',
    sent_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);
