-- 006_userbot_stats.down.sql
ALTER TABLE channels DROP COLUMN IF EXISTS userbot_status;

ALTER TABLE channel_stats_snapshots
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS members_online,
    DROP COLUMN IF EXISTS admins_count,
    DROP COLUMN IF EXISTS growth_7d,
    DROP COLUMN IF EXISTS growth_30d,
    DROP COLUMN IF EXISTS posts_count;
