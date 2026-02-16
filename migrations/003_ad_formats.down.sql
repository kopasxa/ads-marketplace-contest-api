-- 003_ad_formats.down.sql
ALTER TABLE channel_listings DROP COLUMN IF EXISTS auto_accept;
ALTER TABLE channel_listings DROP COLUMN IF EXISTS hold_hours_story;
ALTER TABLE channel_listings DROP COLUMN IF EXISTS hold_hours_repost;
ALTER TABLE channel_listings DROP COLUMN IF EXISTS hold_hours_post;
ALTER TABLE channel_listings DROP COLUMN IF EXISTS formats_enabled;
ALTER TABLE channel_listings DROP COLUMN IF EXISTS price_story_ton;
ALTER TABLE channel_listings DROP COLUMN IF EXISTS price_repost_ton;
ALTER TABLE channel_listings DROP COLUMN IF EXISTS price_post_ton;
ALTER TABLE deal_posts DROP COLUMN IF EXISTS story_expires_at;
ALTER TABLE deal_posts DROP COLUMN IF EXISTS ad_format;
ALTER TABLE deal_creatives DROP COLUMN IF EXISTS buttons_json;
ALTER TABLE deal_creatives DROP COLUMN IF EXISTS media_urls;
ALTER TABLE deal_creatives DROP COLUMN IF EXISTS repost_from_url;
ALTER TABLE deal_creatives DROP COLUMN IF EXISTS repost_from_msg_id;
ALTER TABLE deal_creatives DROP COLUMN IF EXISTS repost_from_chat_id;
ALTER TABLE deals DROP COLUMN IF EXISTS ad_format;
DROP INDEX IF EXISTS idx_deals_ad_format;
