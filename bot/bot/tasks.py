import asyncio
import logging
import hashlib
from datetime import datetime
from typing import Optional

from aiogram import Bot

from bot.db import db

logger = logging.getLogger(__name__)

# In-memory scheduled posts queue (MVP)
# In production, use Redis queue + persistent scheduling
_scheduled_posts: dict[str, dict] = {}


async def schedule_post(bot: Bot, deal_id: str, chat_id: int, text: str,
                        scheduled_at: Optional[datetime] = None, channel_username: str = ""):
    """Schedule a post to be sent to a channel."""
    if scheduled_at and scheduled_at > datetime.utcnow():
        delay = (scheduled_at - datetime.utcnow()).total_seconds()
        _scheduled_posts[deal_id] = {
            "chat_id": chat_id,
            "text": text,
            "scheduled_at": scheduled_at,
            "channel_username": channel_username,
        }
        asyncio.create_task(_delayed_post(bot, deal_id, delay))
        logger.info(f"Post for deal {deal_id} scheduled in {delay:.0f}s")
    else:
        await _send_post(bot, deal_id, chat_id, text, channel_username)


async def _delayed_post(bot: Bot, deal_id: str, delay: float):
    await asyncio.sleep(delay)
    info = _scheduled_posts.pop(deal_id, None)
    if info:
        await _send_post(bot, deal_id, info["chat_id"], info["text"], info.get("channel_username", ""))


async def _send_post(bot: Bot, deal_id: str, chat_id: int, text: str, channel_username: str = ""):
    try:
        msg = await bot.send_message(chat_id=chat_id, text=text)

        # Build post URL
        post_url = ""
        if channel_username:
            post_url = f"https://t.me/{channel_username}/{msg.message_id}"

        # Compute content hash
        content_hash = hashlib.sha256(text.encode()).hexdigest()

        # Save to DB
        await db.pool.execute(
            """
            INSERT INTO deal_posts (deal_id, telegram_message_id, telegram_chat_id, post_url, content_hash, posted_at)
            VALUES ($1::uuid, $2, $3, $4, $5, now())
            ON CONFLICT (deal_id) DO UPDATE SET
                telegram_message_id = EXCLUDED.telegram_message_id,
                telegram_chat_id = EXCLUDED.telegram_chat_id,
                post_url = EXCLUDED.post_url,
                content_hash = EXCLUDED.content_hash,
                posted_at = now()
            """,
            deal_id, msg.message_id, chat_id, post_url, content_hash,
        )

        # Update deal status to posted
        await db.pool.execute(
            "UPDATE deals SET status = 'posted', updated_at = now() WHERE id = $1::uuid AND status IN ('scheduled', 'creative_approved')",
            deal_id,
        )
        # Then to hold_verification
        await db.pool.execute(
            "UPDATE deals SET status = 'hold_verification', updated_at = now() WHERE id = $1::uuid AND status = 'posted'",
            deal_id,
        )

        logger.info(f"Post sent for deal {deal_id}: msg_id={msg.message_id}")
        return {"message_id": msg.message_id, "chat_id": chat_id, "post_url": post_url}

    except Exception as e:
        logger.error(f"Failed to send post for deal {deal_id}: {e}")
        raise


async def send_notification(bot: Bot, telegram_user_id: int, text: str):
    """Send a notification message to a user."""
    try:
        await bot.send_message(chat_id=telegram_user_id, text=text)
    except Exception as e:
        logger.warning(f"Failed to notify user {telegram_user_id}: {e}")
