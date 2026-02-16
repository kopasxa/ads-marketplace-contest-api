-- 001_initial.up.sql
-- Ads Marketplace — initial schema

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================
-- USERS
-- ============================================
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_user_id BIGINT UNIQUE NOT NULL,
    username        TEXT,
    first_name      TEXT,
    last_name       TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_telegram_user_id ON users(telegram_user_id);

-- ============================================
-- CHANNELS
-- ============================================
CREATE TABLE channels (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_chat_id BIGINT UNIQUE,
    username        TEXT UNIQUE NOT NULL,
    title           TEXT,
    added_by_user_id UUID REFERENCES users(id),
    bot_status      TEXT NOT NULL DEFAULT 'pending',  -- pending/active/removed
    bot_added_at    TIMESTAMPTZ,
    bot_removed_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_channels_username ON channels(username);
CREATE INDEX idx_channels_bot_status ON channels(bot_status);

-- ============================================
-- CHANNEL MEMBERS
-- ============================================
CREATE TABLE channel_members (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id      UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK (role IN ('owner', 'manager')),
    can_post        BOOLEAN NOT NULL DEFAULT false,
    last_admin_check_at TIMESTAMPTZ,
    UNIQUE (channel_id, user_id)
);

CREATE INDEX idx_channel_members_channel ON channel_members(channel_id);
CREATE INDEX idx_channel_members_user ON channel_members(user_id);

-- ============================================
-- CHANNEL LISTINGS
-- ============================================
CREATE TABLE channel_listings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id      UUID UNIQUE NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'paused')),
    pricing_json    JSONB NOT NULL DEFAULT '{}',
    min_lead_time_minutes INT NOT NULL DEFAULT 0,
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================
-- CHANNEL STATS SNAPSHOTS
-- ============================================
CREATE TABLE channel_stats_snapshots (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id      UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    fetched_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    subscribers     INT,
    verified_badge  BOOLEAN NOT NULL DEFAULT false,
    avg_views_20    INT,
    last_post_id    BIGINT,
    raw_json        JSONB,
    premium_count   INT  -- placeholder for future
);

CREATE INDEX idx_stats_channel_fetched ON channel_stats_snapshots(channel_id, fetched_at DESC);

-- ============================================
-- DEALS
-- ============================================
CREATE TABLE deals (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id          UUID NOT NULL REFERENCES channels(id),
    advertiser_user_id  UUID NOT NULL REFERENCES users(id),
    status              TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN (
            'draft', 'submitted', 'rejected', 'accepted',
            'awaiting_payment', 'funded',
            'creative_pending', 'creative_submitted',
            'creative_changes_requested', 'creative_approved',
            'scheduled', 'posted',
            'hold_verification', 'hold_verification_failed',
            'completed', 'refunded', 'cancelled'
        )),
    brief               TEXT,
    scheduled_at        TIMESTAMPTZ,
    price_ton           NUMERIC(30, 9) NOT NULL,
    platform_fee_bps    INT NOT NULL DEFAULT 300,
    hold_period_seconds INT NOT NULL DEFAULT 3600,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_deals_channel ON deals(channel_id);
CREATE INDEX idx_deals_advertiser ON deals(advertiser_user_id);
CREATE INDEX idx_deals_status ON deals(status);

-- ============================================
-- DEAL CREATIVES
-- ============================================
CREATE TABLE deal_creatives (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deal_id                 UUID NOT NULL REFERENCES deals(id) ON DELETE CASCADE,
    version                 INT NOT NULL DEFAULT 1,
    owner_composed_text     TEXT,
    advertiser_materials_text TEXT,
    status                  TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'submitted', 'changes_requested', 'approved')),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_deal_creatives_deal ON deal_creatives(deal_id);

-- ============================================
-- DEAL POSTS
-- ============================================
CREATE TABLE deal_posts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deal_id             UUID UNIQUE NOT NULL REFERENCES deals(id) ON DELETE CASCADE,
    telegram_message_id BIGINT,
    telegram_chat_id    BIGINT,
    post_url            TEXT,
    content_hash        TEXT,
    posted_at           TIMESTAMPTZ,
    last_checked_at     TIMESTAMPTZ,
    is_deleted          BOOLEAN NOT NULL DEFAULT false,
    is_edited           BOOLEAN NOT NULL DEFAULT false
);

-- ============================================
-- ESCROW LEDGER
-- ============================================
CREATE TABLE escrow_ledger (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deal_id             UUID UNIQUE NOT NULL REFERENCES deals(id) ON DELETE CASCADE,
    deposit_expected_ton NUMERIC(30, 9) NOT NULL,
    deposit_address     TEXT NOT NULL,
    deposit_memo        TEXT NOT NULL UNIQUE,
    funded_at           TIMESTAMPTZ,
    funding_tx_hash     TEXT,
    payer_address       TEXT,
    release_amount_ton  NUMERIC(30, 9),
    release_tx_hash     TEXT,
    refunded_at         TIMESTAMPTZ,
    refund_tx_hash      TEXT,
    status              TEXT NOT NULL DEFAULT 'awaiting'
        CHECK (status IN ('awaiting', 'funded', 'released', 'refunded'))
);

CREATE INDEX idx_escrow_deal ON escrow_ledger(deal_id);
CREATE INDEX idx_escrow_memo ON escrow_ledger(deposit_memo);

-- ============================================
-- AUDIT LOG
-- ============================================
CREATE TABLE audit_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id   UUID REFERENCES users(id),
    actor_type      TEXT NOT NULL CHECK (actor_type IN ('user', 'admin', 'system', 'bot')),
    action          TEXT NOT NULL,
    entity_type     TEXT NOT NULL,
    entity_id       UUID,
    meta            JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_actor ON audit_log(actor_user_id);
CREATE INDEX idx_audit_created ON audit_log(created_at DESC);

-- ============================================
-- WITHDRAW WALLETS (per channel owner)
-- ============================================
CREATE TABLE withdraw_wallets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id  UUID UNIQUE NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    owner_user_id UUID NOT NULL REFERENCES users(id),
    wallet_address TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
