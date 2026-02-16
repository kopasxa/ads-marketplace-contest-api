-- 007_broadcast_stats.down.sql

ALTER TABLE channel_stats_snapshots
    DROP COLUMN IF EXISTS views_per_post,
    DROP COLUMN IF EXISTS shares_per_post,
    DROP COLUMN IF EXISTS enabled_notifications_percent,
    DROP COLUMN IF EXISTS er_percent;
