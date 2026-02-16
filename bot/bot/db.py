import asyncpg
from typing import Optional
from bot.config import config


class Database:
    pool: Optional[asyncpg.Pool] = None

    async def connect(self):
        self.pool = await asyncpg.create_pool(config.POSTGRES_DSN, min_size=2, max_size=10)

    async def close(self):
        if self.pool:
            await self.pool.close()

    async def upsert_user(self, telegram_user_id: int, username: str = None,
                          first_name: str = None, last_name: str = None) -> str:
        """Upsert user and return user UUID."""
        row = await self.pool.fetchrow(
            """
            INSERT INTO users (telegram_user_id, username, first_name, last_name)
            VALUES ($1, $2, $3, $4)
            ON CONFLICT (telegram_user_id) DO UPDATE SET
                username = COALESCE(EXCLUDED.username, users.username),
                first_name = COALESCE(EXCLUDED.first_name, users.first_name),
                last_name = COALESCE(EXCLUDED.last_name, users.last_name),
                last_active_at = now()
            RETURNING id
            """,
            telegram_user_id, username, first_name, last_name,
        )
        return str(row["id"])

    async def get_user_id_by_telegram(self, telegram_user_id: int) -> Optional[str]:
        row = await self.pool.fetchrow(
            "SELECT id FROM users WHERE telegram_user_id = $1", telegram_user_id
        )
        return str(row["id"]) if row else None

    async def get_telegram_id_by_user_uuid(self, user_uuid: str) -> Optional[int]:
        """Get telegram_user_id by internal user UUID."""
        row = await self.pool.fetchrow(
            "SELECT telegram_user_id FROM users WHERE id = $1::uuid", user_uuid
        )
        return row["telegram_user_id"] if row else None

    async def upsert_channel(
        self,
        telegram_chat_id: int,
        username: str,
        title: str,
        added_by_user_id: str,
        bot_status: str = "active",
    ):
        """Upsert channel when bot is added."""
        await self.pool.execute(
            """
            INSERT INTO channels (telegram_chat_id, username, title, added_by_user_id, bot_status, bot_added_at)
            VALUES ($1, $2, $3, $4::uuid, $5, now())
            ON CONFLICT (username) DO UPDATE SET
                telegram_chat_id = COALESCE(EXCLUDED.telegram_chat_id, channels.telegram_chat_id),
                title = COALESCE(EXCLUDED.title, channels.title),
                added_by_user_id = COALESCE(EXCLUDED.added_by_user_id, channels.added_by_user_id),
                bot_status = EXCLUDED.bot_status,
                bot_added_at = now(),
                updated_at = now()
            """,
            telegram_chat_id, username, title, added_by_user_id, bot_status,
        )

    async def update_channel_bot_removed(self, username: str):
        await self.pool.execute(
            """
            UPDATE channels SET bot_status = 'removed', bot_removed_at = now(), updated_at = now()
            WHERE username = $1
            """,
            username,
        )

    async def update_userbot_status(self, username: str, status: str):
        """Update userbot_status for a channel."""
        await self.pool.execute(
            """
            UPDATE channels SET userbot_status = $1, updated_at = now()
            WHERE username = $2
            """,
            status,
            username,
        )

    async def update_channel_info(
        self, telegram_chat_id: int, username: str = None, title: str = None
    ):
        """Update channel username and/or title by telegram_chat_id."""
        if username is not None and title is not None:
            await self.pool.execute(
                """
                UPDATE channels SET username = $1, title = $2, updated_at = now()
                WHERE telegram_chat_id = $3
                """,
                username,
                title,
                telegram_chat_id,
            )
        elif username is not None:
            await self.pool.execute(
                """
                UPDATE channels SET username = $1, updated_at = now()
                WHERE telegram_chat_id = $2
                """,
                username,
                telegram_chat_id,
            )
        elif title is not None:
            await self.pool.execute(
                """
                UPDATE channels SET title = $1, updated_at = now()
                WHERE telegram_chat_id = $2
                """,
                title,
                telegram_chat_id,
            )

    async def get_channel_id_by_username(self, username: str) -> Optional[str]:
        """Get channel UUID by username."""
        row = await self.pool.fetchrow(
            "SELECT id FROM channels WHERE username = $1", username
        )
        return str(row["id"]) if row else None

    async def get_active_deals_by_channel_username(self, username: str) -> list:
        """Get all active (non-terminal) deals for a channel."""
        terminal_statuses = (
            "completed",
            "cancelled",
            "refunded",
            "rejected",
        )
        rows = await self.pool.fetch(
            """
            SELECT d.id, d.status, d.advertiser_user_id, d.price_ton
            FROM deals d
            JOIN channels c ON c.id = d.channel_id
            WHERE c.username = $1
              AND d.status != ALL($2)
            """,
            username,
            list(terminal_statuses),
        )
        return rows

    async def cancel_deal_system(self, deal_id: str) -> bool:
        """Cancel a deal (system action). Returns True if status was changed."""
        result = await self.pool.execute(
            """
            UPDATE deals SET status = 'cancelled', updated_at = now()
            WHERE id = $1::uuid
              AND status NOT IN ('completed', 'cancelled', 'refunded', 'rejected')
            """,
            deal_id,
        )
        return result != "UPDATE 0"

    async def refund_deal_system(self, deal_id: str) -> bool:
        """Transition cancelled deal to refunded."""
        result = await self.pool.execute(
            """
            UPDATE deals SET status = 'refunded', updated_at = now()
            WHERE id = $1::uuid AND status = 'cancelled'
            """,
            deal_id,
        )
        return result != "UPDATE 0"

    async def log_audit(
        self,
        action: str,
        entity_type: str,
        entity_id: str,
        actor_type: str = "system",
        details: str = None,
    ):
        """Insert an audit log entry."""
        await self.pool.execute(
            """
            INSERT INTO audit_log (actor_type, action, entity_type, entity_id, details)
            VALUES ($1, $2, $3, $4::uuid, $5)
            """,
            actor_type,
            action,
            entity_type,
            entity_id,
            details,
        )

    async def add_channel_member(self, channel_username: str, user_id: str, role: str, can_post: bool):
        await self.pool.execute(
            """
            INSERT INTO channel_members (channel_id, user_id, role, can_post, last_admin_check_at)
            SELECT c.id, $2::uuid, $3, $4, now()
            FROM channels c WHERE c.username = $1
            ON CONFLICT (channel_id, user_id) DO UPDATE SET
                role = EXCLUDED.role, can_post = EXCLUDED.can_post, last_admin_check_at = now()
            """,
            channel_username, user_id, role, can_post,
        )

    async def get_deal_post_info(self, deal_id: str):
        return await self.pool.fetchrow(
            """
            SELECT dp.*, d.channel_id, c.telegram_chat_id, c.username as channel_username
            FROM deal_posts dp
            JOIN deals d ON d.id = dp.deal_id
            JOIN channels c ON c.id = d.channel_id
            WHERE dp.deal_id = $1::uuid
            """,
            deal_id,
        )


db = Database()
