-- 007_broadcast_stats.up.sql
-- Extended stats fields from Telegram GetBroadcastStats API

ALTER TABLE channel_stats_snapshots
    ADD COLUMN views_per_post DOUBLE PRECISION,
    ADD COLUMN shares_per_post DOUBLE PRECISION,
    ADD COLUMN enabled_notifications_percent DOUBLE PRECISION,
    ADD COLUMN er_percent DOUBLE PRECISION;
