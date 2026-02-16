-- 005_listing_category_campaign_simplify.down.sql
ALTER TABLE channel_listings
    DROP COLUMN IF EXISTS category,
    DROP COLUMN IF EXISTS language;

DROP INDEX IF EXISTS idx_listings_category;
DROP INDEX IF EXISTS idx_listings_language;

ALTER TABLE campaigns
    ADD COLUMN objective TEXT NOT NULL DEFAULT '',
    ADD COLUMN guidelines TEXT;
