-- 006_userbot_stats.up.sql
-- Track userbot presence per channel for advanced stats collection

ALTER TABLE channels
    ADD COLUMN userbot_status TEXT NOT NULL DEFAULT 'none'
        CHECK (userbot_status IN ('none', 'pending', 'active', 'failed', 'removed'));

COMMENT ON COLUMN channels.userbot_status IS
    'none=not attempted, pending=invite sent, active=userbot is admin, failed=invite failed, removed=userbot was removed';

-- Extend stats snapshots with richer data from userbot
ALTER TABLE channel_stats_snapshots
    ADD COLUMN source TEXT NOT NULL DEFAULT 'tme_parser',
    ADD COLUMN members_online INT,
    ADD COLUMN admins_count INT,
    ADD COLUMN growth_7d INT,
    ADD COLUMN growth_30d INT,
    ADD COLUMN posts_count INT;
