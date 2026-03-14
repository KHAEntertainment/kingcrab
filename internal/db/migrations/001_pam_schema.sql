-- KingCrab PAM Schema Migration
-- Run with: psql -U kingcrab -d kingcrab -f 001_pam_schema.sql

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================
-- AUTHORIZED USERS
-- ============================================

-- Users who can receive/approve elevation requests
CREATE TABLE IF NOT EXISTS authorized_users (
    id              SERIAL PRIMARY KEY,
    
    -- Identity (at least one required)
    telegram_id     BIGINT UNIQUE,
    clawvault_id    VARCHAR(255) UNIQUE,
    
    -- Profile
    username        VARCHAR(255),
    display_name    VARCHAR(255),
    
    -- Settings
    auth_mode       VARCHAR(20) DEFAULT 'biometric' CHECK (auth_mode IN ('biometric', 'clawvault', 'none')),
    is_active       BOOLEAN DEFAULT TRUE,
    
    -- Timestamps
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Constraint: must have at least one identity
    CONSTRAINT one_identity CHECK (
        (telegram_id IS NOT NULL) OR (clawvault_id IS NOT NULL)
    )
);

-- Index for lookups
CREATE INDEX IF NOT EXISTS idx_users_telegram ON authorized_users(telegram_id);
CREATE INDEX IF NOT EXISTS idx_users_clawvault ON authorized_users(clawvault_id);
CREATE INDEX IF NOT EXISTS idx_users_active ON authorized_users(is_active);

-- ============================================
-- ENROLLED DEVICES
-- ============================================

-- Biometric devices enrolled for PAM
CREATE TABLE IF NOT EXISTS enrolled_devices (
    id              SERIAL PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES authorized_users(id) ON DELETE CASCADE,
    
    -- Token storage reference (encrypted value stored elsewhere)
    token_ref       VARCHAR(512),                    -- Key/path to retrieve encrypted token
    token_storage   VARCHAR(20) DEFAULT 'local' CHECK (token_storage IN ('local', 'clawvault')),
    
    -- Device info
    device_info     VARCHAR(255),                    -- "iPhone 15 Pro", etc.
    device_hash     VARCHAR(64),                     -- Unique device identifier
    
    -- Status
    is_active       BOOLEAN DEFAULT TRUE,
    
    -- Timestamps
    enrolled_at     TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_used_at    TIMESTAMP WITH TIME ZONE,
    expires_at      TIMESTAMP WITH TIME ZONE        -- For temp tokens
);

CREATE INDEX IF NOT EXISTS idx_devices_user ON enrolled_devices(user_id);
CREATE INDEX IF NOT EXISTS idx_devices_active ON enrolled_devices(is_active);
CREATE INDEX IF NOT EXISTS idx_devices_hash ON enrolled_devices(device_hash);

-- ============================================
-- ELEVATION REQUESTS
-- ============================================

-- The actual elevation requests
CREATE TABLE IF NOT EXISTS elevation_requests (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Request details
    requester       VARCHAR(255) NOT NULL,          -- Who requested
    target_system   VARCHAR(255) NOT NULL,          -- Target server/system
    command         TEXT NOT NULL,                  -- Command to execute
    reason          TEXT,                           -- Justification
    
    -- Status tracking
    status          VARCHAR(20) DEFAULT 'pending' 
                    CHECK (status IN ('pending', 'approved', 'denied', 'expired', 'executing', 'failed')),
    
    -- Timing
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at      TIMESTAMP WITH TIME ZONE NOT NULL,
    approved_at     TIMESTAMP WITH TIME ZONE,
    executed_at     TIMESTAMP WITH TIME ZONE,
    
    -- Approval info
    approved_by     TEXT,                     -- User identity string (e.g., "tg:12345")
    
    -- Network info (for audit)
    ip_address      VARCHAR(45),                    -- IPv6 compatible
    user_agent      VARCHAR(512),
    
    -- Result
    output          TEXT,
    exit_code       INTEGER
);

CREATE INDEX IF NOT EXISTS idx_requests_status ON elevation_requests(status);
CREATE INDEX IF NOT EXISTS idx_requests_created ON elevation_requests(created_at);
CREATE INDEX IF NOT EXISTS idx_requests_expires ON elevation_requests(expires_at);
CREATE INDEX IF NOT EXISTS idx_requests_requester ON elevation_requests(requester);
CREATE INDEX IF NOT EXISTS idx_requests_approved ON elevation_requests(approved_at);

-- ============================================
-- APPROVAL AUDIT LOG
-- ============================================

-- All approval attempts (success + failure)
CREATE TABLE IF NOT EXISTS approval_audit (
    id              SERIAL PRIMARY KEY,
    
    -- References
    request_id      UUID REFERENCES elevation_requests(id) ON DELETE SET NULL,
    device_id       INTEGER REFERENCES enrolled_devices(id) ON DELETE SET NULL,
    user_id         INTEGER REFERENCES authorized_users(id) ON DELETE SET NULL,
    
    -- Action details
    action          VARCHAR(30) NOT NULL 
                    CHECK (action IN (
                        'approve', 'deny', 
                        'fail_expire', 'fail_invalid_token', 'fail_invalid_init', 
                        'fail_unauthorized', 'fail_not_found'
                    )),
    
    -- Network info
    ip_address      VARCHAR(45),
    user_agent      VARCHAR(512),
    
    -- Additional context
    details         JSONB,
    
    -- Timestamp
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_request ON approval_audit(request_id);
CREATE INDEX IF NOT EXISTS idx_audit_device ON approval_audit(device_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON approval_audit(created_at);

-- ============================================
-- TRIGGERS
-- ============================================

-- Auto-update updated_at on user changes
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON authorized_users 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================
-- CLEANUP JOBS (run via cron)
-- ============================================

-- Delete old completed requests using approved_at (terminal timestamp)
CREATE OR REPLACE FUNCTION cleanup_completed_requests(retention_days INTEGER DEFAULT 30)
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    -- Mark expired requests
    UPDATE elevation_requests
    SET status = 'expired',
        approved_at = NOW()  -- Set terminal timestamp when transitioning to terminal state
    WHERE status = 'pending'
    AND expires_at < NOW()
    AND approved_at IS NULL;  -- Only update if not already set

    -- Delete old completed requests based on terminal timestamp (approved_at)
    -- For terminal statuses, approved_at is used as the timestamp when status became terminal
    DELETE FROM elevation_requests
    WHERE status IN ('approved', 'denied', 'expired', 'failed')
    AND approved_at IS NOT NULL
    AND approved_at < NOW() - (retention_days || ' days')::INTERVAL;

    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- ============================================
-- SEEDS (example data)
-- ============================================

-- Insert example authorized user (Billy)
-- INSERT INTO authorized_users (telegram_id, username, display_name, auth_mode)
-- VALUES (6778651323, 'bbrenner2217', 'Billy Brenner', 'biometric')
-- ON CONFLICT (telegram_id) DO NOTHING;

-- ============================================
-- ROLES & PERMISSIONS (for production)
-- ============================================

-- Example: Create read-only role for monitoring
-- CREATE ROLE kingcrab_ro LOGIN PASSWORD '...';
-- GRANT CONNECT ON DATABASE kingcrab TO kingcrab_ro;
-- GRANT USAGE ON SCHEMA public TO kingcrab_ro;
-- GRANT SELECT ON ALL TABLES IN SCHEMA public TO kingcrab_ro;
-- GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO kingcrab_ro;