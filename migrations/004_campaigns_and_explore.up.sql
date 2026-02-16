-- 004_campaigns_and_explore.up.sql
-- Campaigns table + explore support

-- ============================================
-- CAMPAIGNS
-- ============================================
CREATE TABLE campaigns (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    advertiser_user_id  UUID NOT NULL REFERENCES users(id),
    title               TEXT NOT NULL,
    objective           TEXT NOT NULL,
    target_audience     TEXT NOT NULL,
    key_messages        TEXT,
    guidelines          TEXT,
    budget_ton          NUMERIC(30, 9) NOT NULL,
    preferred_date      TIMESTAMPTZ,
    status              TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'paused', 'completed', 'cancelled')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_campaigns_advertiser ON campaigns(advertiser_user_id);
CREATE INDEX idx_campaigns_status ON campaigns(status);
