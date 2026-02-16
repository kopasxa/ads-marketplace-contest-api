# Ads Marketplace Backend

Backend for a Telegram Mini-App marketplace for advertising in public channels.

## Architecture

```
┌─────────────┐    ┌──────────┐    ┌───────────┐
│  Mini-App   │───▶│  API     │───▶│ Postgres  │
│  (Frontend) │    │  (Fiber) │    │           │
└─────────────┘    └────┬─────┘    └───────────┘
                        │                │
                   ┌────▼─────┐    ┌─────▼─────┐
                   │  Redis   │    │  Workers  │
                   │ (cache/  │    │ (timeouts,│
                   │  pubsub) │    │  monitor) │
                   └──────────┘    └───────────┘
                        │
              ┌─────────┼─────────┐
              │         │         │
         ┌────▼───┐ ┌───▼────┐ ┌─▼──────────┐
         │  Bot   │ │ Stats  │ │ TON Indexer │
         │(Python)│ │Fetcher │ │ (lite srv)  │
         └────────┘ └────────┘ └─────────────┘
```

## Services

| Service | Language | Description |
|---------|----------|-------------|
| `api` | Go (Fiber) | REST API + WebSocket |
| `worker` | Go | Background jobs: timeouts, hold release, post monitoring |
| `stats` | Go | Channel stats fetcher (HTML parsing t.me/s/) |
| `ton-indexer` | Go | TON blockchain indexer for payment detection |
| `bot` | Python (aiogram + FastAPI) | Telegram Bot: events, admin checks, posting, notifications |

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.24+ (for local dev)
- Python 3.12+ (for bot local dev)

### 1. Configure environment

```bash
cp .env.example .env
# Edit .env — set BOT_TOKEN and other values
```

### 2. Run with Docker Compose

```bash
cd deploy
docker-compose up --build
```

This starts: Postgres, Redis, API, Worker, Stats, TON Indexer, Bot.

### 3. Run locally (dev)

```bash
# Start Postgres & Redis
docker-compose -f deploy/docker-compose.yml up postgres redis -d

# Run API
go run ./cmd/api

# Run Worker
go run ./cmd/worker

# Run Stats Fetcher
go run ./cmd/stats

# Run TON Indexer
go run ./cmd/ton-indexer

# Run Bot
cd bot && pip install -r requirements.txt && python main.py
```

### 4. Apply migrations

Migrations run automatically on API startup. To run manually:

```bash
go run ./cmd/api  # migrations in /migrations/ applied on start
```

## API Endpoints

Base URL: `http://localhost:3000/api/v1`

### Auth
| Method | Path | Description |
|--------|------|-------------|
| POST | `/auth/telegram` | Authenticate via Telegram WebApp initData |

### User
| Method | Path | Description |
|--------|------|-------------|
| GET | `/me` | Get current user |
| POST | `/me/ping` | Update last_active_at |

### Channels
| Method | Path | Description |
|--------|------|-------------|
| POST | `/channels` | Create channel draft by @username |
| GET | `/channels` | Search/filter channels |
| GET | `/channels/:id` | Get channel by ID |
| POST | `/channels/:id/invite-bot` | Get bot invite instructions |
| POST | `/channels/:id/managers` | Add manager (max 3 total) |
| GET | `/channels/:id/admins` | List channel admins via Bot API |

### Listings
| Method | Path | Description |
|--------|------|-------------|
| PUT | `/listings/:channelId` | Update listing (pricing, status, desc) |
| GET | `/listings/:channelId` | Get listing |

### Deals
| Method | Path | Description |
|--------|------|-------------|
| POST | `/deals` | Create deal (advertiser) |
| GET | `/deals` | List deals (filter by role) |
| GET | `/deals/:id` | Get deal |
| POST | `/deals/:id/submit` | Submit deal to owner |
| POST | `/deals/:id/accept` | Owner accepts deal |
| POST | `/deals/:id/reject` | Owner rejects deal |
| POST | `/deals/:id/cancel` | Cancel deal |
| POST | `/deals/:id/creative` | Submit creative (owner) |
| POST | `/deals/:id/creative/approve` | Approve creative (advertiser) |
| POST | `/deals/:id/creative/request-changes` | Request changes (advertiser) |
| POST | `/deals/:id/post/mark-manual` | Mark manual post URL (owner) |
| POST | `/deals/:id/finance/set-withdraw-wallet` | Set withdraw wallet (owner only, re-check) |
| GET | `/deals/:id/payment` | Get payment info (wallet + memo) |

### WebSocket
| Path | Description |
|------|-------------|
| `ws://localhost:3000/ws?token=JWT` | Real-time deal status updates |

## Deal Flow

```
draft → submitted → accepted → awaiting_payment → funded
  → creative_pending → creative_submitted → creative_approved
  → scheduled → posted → hold_verification → completed

Branches:
  submitted → rejected
  * → cancelled → refunded
  hold_verification → hold_verification_failed → refunded
  creative_submitted → creative_changes_requested → creative_submitted
```

## Stats Parsing

Channel statistics are fetched by parsing `https://t.me/s/<username>`.

Test manually:
```bash
curl -s "https://t.me/s/durov" | head
curl -s "https://t.me/s/telegram" | head
```

Extracted metrics:
- Subscriber count
- Verified badge
- Last N posts with views
- Average views (last 20 posts)
- Language guess (Unicode range heuristic)

## Environment Variables

See `.env.example` for full list with defaults.

Key variables:
- `BOT_TOKEN` — Telegram Bot API token
- `POSTGRES_DSN` — PostgreSQL connection string
- `REDIS_URL` — Redis connection string
- `TON_HOT_WALLET_ADDRESS` — TON hot wallet for escrow
- `PLATFORM_FEE_BPS` — Platform fee in basis points (300 = 3%)
- `HOLD_PERIOD_SECONDS` — Post hold verification period
- `JWT_SECRET` — JWT signing secret
- `ADMIN_TELEGRAM_IDS` — Comma-separated admin Telegram IDs

## Project Structure

```
├── cmd/
│   ├── api/              # Go Fiber API server
│   ├── worker/           # Background jobs
│   ├── stats/            # Stats fetcher + parser
│   ├── ton-indexer/      # TON blockchain indexer
│   └── bot-notify-bridge/
├── internal/
│   ├── config/           # Config loader
│   ├── db/               # Postgres pool + Redis + migrations
│   ├── models/           # Data models
│   ├── repositories/     # Database access layer
│   ├── services/         # Business logic (DealService, ChannelService)
│   ├── http/             # Fiber handlers + router + DTOs
│   ├── middleware/        # Auth, rate limit, logging, request ID
│   ├── events/           # Redis pub/sub
│   ├── auth/             # Telegram WebApp validation + JWT
│   ├── ton/              # TON lite client placeholder
│   ├── statsparser/      # HTML parser for t.me/s/
│   └── rbac/             # Role-based access (via channel_members)
├── migrations/           # SQL migrations
├── bot/                  # Python bot service
│   ├── main.py
│   ├── requirements.txt
│   └── bot/
│       ├── config.py
│       ├── db.py
│       ├── handlers.py   # my_chat_member events
│       ├── permissions.py # Admin checks via Bot API
│       ├── tasks.py      # Post scheduling + notifications
│       └── telegram.py   # FastAPI internal API
├── deploy/               # Docker Compose + Dockerfiles
└── scripts/
```
