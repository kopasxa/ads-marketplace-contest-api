-- 003_ad_formats.up.sql
-- Ad format types + structured pricing + format-specific fields

-- ============================================
-- 1. Тип рекламного формата в deals
-- ============================================
ALTER TABLE deals ADD COLUMN ad_format TEXT NOT NULL DEFAULT 'post'
    CHECK (ad_format IN ('post', 'repost', 'story'));

-- Индекс для фильтрации
CREATE INDEX idx_deals_ad_format ON deals(ad_format);

-- ============================================
-- 2. Структурированные цены в листинге
-- ============================================
-- Заменяем аморфный pricing_json на нормальные колонки
-- Старый pricing_json остаётся для обратной совместимости

ALTER TABLE channel_listings
    ADD COLUMN price_post_ton      NUMERIC(30, 9),   -- цена за пост
    ADD COLUMN price_repost_ton    NUMERIC(30, 9),   -- цена за репост
    ADD COLUMN price_story_ton     NUMERIC(30, 9),   -- цена за историю
    ADD COLUMN formats_enabled     TEXT[] NOT NULL DEFAULT ARRAY['post']; -- какие форматы включены

-- ============================================
-- 3. Формат-специфичные поля в креативах
-- ============================================
ALTER TABLE deal_creatives
    ADD COLUMN repost_from_chat_id   BIGINT,          -- для repost: откуда репостить
    ADD COLUMN repost_from_msg_id    BIGINT,          -- для repost: какое сообщение
    ADD COLUMN repost_from_url       TEXT,             -- для repost: ссылка на оригинал
    ADD COLUMN media_urls            JSONB DEFAULT '[]', -- ссылки на медиа (картинки/видео)
    ADD COLUMN buttons_json          JSONB DEFAULT '[]'; -- inline кнопки [{text, url}]

-- ============================================
-- 4. Формат-специфичные поля в deal_posts
-- ============================================
ALTER TABLE deal_posts
    ADD COLUMN ad_format             TEXT,             -- дублируем для удобства мониторинга
    ADD COLUMN story_expires_at      TIMESTAMPTZ;      -- когда стори истечёт (24ч)

-- ============================================
-- 5. Настройки канала: hold per format, etc
-- ============================================
ALTER TABLE channel_listings
    ADD COLUMN hold_hours_post     INT NOT NULL DEFAULT 24,   -- hold для постов (часы)
    ADD COLUMN hold_hours_repost   INT NOT NULL DEFAULT 24,
    ADD COLUMN hold_hours_story    INT NOT NULL DEFAULT 0,    -- стори и так исчезает через 24ч
    ADD COLUMN auto_accept         BOOLEAN NOT NULL DEFAULT false; -- авто-одобрение сделок
