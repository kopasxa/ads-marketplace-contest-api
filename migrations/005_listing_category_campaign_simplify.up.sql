-- 005_listing_category_campaign_simplify.up.sql
-- Add category/language to channel_listings; simplify campaigns

-- Channel listing: category + language
ALTER TABLE channel_listings
    ADD COLUMN category TEXT,
    ADD COLUMN language TEXT;

CREATE INDEX idx_listings_category ON channel_listings(category);
CREATE INDEX idx_listings_language ON channel_listings(language);

-- Campaigns: drop objective + guidelines (no longer needed)
ALTER TABLE campaigns
    DROP COLUMN IF EXISTS objective,
    DROP COLUMN IF EXISTS guidelines;
