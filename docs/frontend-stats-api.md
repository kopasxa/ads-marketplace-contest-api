# Channel Statistics API — Frontend Documentation

## Overview

Channel statistics are now collected via **Telegram's native GetBroadcastStats API** through a Pyrogram userbot, providing significantly richer data than the previous `t.me` page parser.

**Data source priority:**
1. **`userbot` (GetBroadcastStats)** — native Telegram stats, requires admin + ≥500 subscribers
2. **`userbot` (fallback)** — message history + GetFullChannel for channels < 500 subs
3. **`tme_parser`** — HTML parser from `t.me/channel` page (legacy fallback)

Check the `source` field in the response to know which method was used.

---

## Endpoints

### `GET /api/channels/:id/stats`

Returns detailed statistics for a single channel.

**Auth:** required (JWT)

#### Response

```json
{
  "ok": true,
  "data": {
    "subscribers": 15230,
    "avg_views": 4200,
    "premium_count": null,
    "last_updated": "2026-02-17T01:30:00Z",
    "source": "userbot",
    "members_online": 312,
    "admins_count": 5,
    "growth_7d": 420,
    "growth_30d": 1580,
    "posts_count": 1847,
    "views_per_post": 4215.3,
    "shares_per_post": 87.2,
    "enabled_notifications_percent": 38.45,
    "er_percent": 27.68
  }
}
```

#### Fields

| Field | Type | Nullable | Source | Description |
|-------|------|----------|--------|-------------|
| `subscribers` | `int` | yes | all | Total subscriber count |
| `avg_views` | `int` | yes | all | Average views on last 20 posts |
| `premium_count` | `int` | yes | tme_parser | Premium subscribers (only from t.me parser) |
| `last_updated` | `string` | yes | all | ISO 8601 timestamp of when stats were fetched |
| `source` | `string` | no | all | `"userbot"` or `"tme_parser"` |
| `members_online` | `int` | yes | userbot | Members currently online |
| `admins_count` | `int` | yes | userbot | Total admin count |
| `growth_7d` | `int` | yes | userbot | Subscriber growth over ~7 days |
| `growth_30d` | `int` | yes | computed | Subscriber growth over ~30 days (from snapshot history) |
| `posts_count` | `int` | yes | userbot | Approximate total post count |
| `views_per_post` | `float` | yes | userbot/broadcast | Average views per post (from GetBroadcastStats) |
| `shares_per_post` | `float` | yes | userbot/broadcast | Average shares/forwards per post |
| `enabled_notifications_percent` | `float` | yes | userbot/broadcast | % of subscribers with notifications enabled |
| `er_percent` | `float` | yes | userbot | Engagement Rate = (views_per_post / subscribers) × 100 |

> **Note:** Fields marked `userbot/broadcast` are only available for channels with ≥500 subscribers where the userbot has admin access. For smaller channels or when `source = "tme_parser"`, these fields will be `null`.

---

### `GET /api/explore/channels`

Returns a list of channels for the marketplace/explore page.

**Auth:** required (JWT)

**Query params:**
- `limit` — max results (default 20, max 100)
- `offset` — pagination offset
- `category` — filter by category
- `language` — filter by language
- `status` — filter by listing status

#### Response

```json
{
  "ok": true,
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "username": "testchannel",
      "title": "Test Channel",
      "bot_status": "active",
      "subscribers": 15230,
      "avg_views": 4200,
      "er_percent": 27.68,
      "category": "crypto",
      "language": "ru",
      "listing": {
        "status": "active",
        "price_post_ton": "5.00",
        "price_repost_ton": "3.00",
        "price_story_ton": "2.00",
        "description": "Advertising on our crypto channel"
      }
    }
  ]
}
```

#### New field in explore

| Field | Type | Nullable | Description |
|-------|------|----------|-------------|
| `er_percent` | `float` | yes | Engagement Rate % — useful for sorting/filtering channels by quality |

---

## What changed (2026-02-17)

### New fields added

All new fields are **nullable** (`null` when data is unavailable) and **backward-compatible** — existing frontend code won't break.

| Field | Where | Description |
|-------|-------|-------------|
| `source` | `/channels/:id/stats` | Shows which data source was used |
| `members_online` | `/channels/:id/stats` | Live online member count |
| `admins_count` | `/channels/:id/stats` | Channel admin count |
| `growth_7d` | `/channels/:id/stats` | ~7-day subscriber growth delta |
| `growth_30d` | `/channels/:id/stats` | ~30-day growth (from snapshot diffs) |
| `posts_count` | `/channels/:id/stats` | Approximate total posts |
| `views_per_post` | `/channels/:id/stats` | Precise avg views (from Telegram stats) |
| `shares_per_post` | `/channels/:id/stats` | Avg shares/forwards per post |
| `enabled_notifications_percent` | `/channels/:id/stats` | % with notifications on |
| `er_percent` | `/channels/:id/stats`, `/explore/channels` | Engagement Rate % |

### Userbot integration

When a channel owner adds the bot to their channel:
1. The **userbot** (a real Telegram account) automatically joins the channel
2. The bot promotes the userbot as a minimal admin (`can_manage_chat` only)
3. Stats are collected via Telegram's native `GetBroadcastStats` API
4. The `userbot_status` field on the channel tracks this: `none` → `pending` → `active` / `failed`

### Migration required

Migration `007_broadcast_stats.up.sql` adds 4 new columns to `channel_stats_snapshots`:
- `views_per_post DOUBLE PRECISION`
- `shares_per_post DOUBLE PRECISION`
- `enabled_notifications_percent DOUBLE PRECISION`
- `er_percent DOUBLE PRECISION`

---

## Frontend usage suggestions

### Channel card (explore page)
Display `er_percent` as a badge or indicator (e.g., "ER 27.7%") to help advertisers compare channels.

### Channel detail page
- **Key metrics row:** `subscribers`, `avg_views`, `er_percent`, `growth_7d`
- **Detailed stats section:** `views_per_post`, `shares_per_post`, `enabled_notifications_percent`, `members_online`, `admins_count`, `posts_count`
- **Growth:** `growth_7d` (with + or − sign), `growth_30d`
- **Data quality badge:** Show "Verified stats" when `source = "userbot"`, "Estimated" when `source = "tme_parser"`

### Handling null values
All new fields can be `null`. Display a dash (`—`) or hide the section when the value is not available.
