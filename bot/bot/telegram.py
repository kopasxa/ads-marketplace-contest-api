import logging
from contextlib import asynccontextmanager
from datetime import datetime
from typing import Optional

from aiogram import Bot, Dispatcher
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from bot.config import config
from bot.db import db
from bot.handlers import router
from bot.permissions import get_channel_admins, check_admin
from bot.tasks import schedule_post, send_notification

logger = logging.getLogger(__name__)

# ---- Bot setup ----
bot = Bot(token=config.BOT_TOKEN)
dp = Dispatcher()
dp.include_router(router)


# ---- FastAPI Internal API ----


class PostRequest(BaseModel):
    chat_id: int
    text: str
    scheduled_at: Optional[datetime] = None
    channel_username: Optional[str] = ""


class NotifyRequest(BaseModel):
    telegram_user_id: int
    text: str


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup
    await db.connect()
    logger.info("Database connected")

    if not config.USE_WEBHOOK:
        import asyncio
        asyncio.create_task(start_polling())

    yield

    # Shutdown
    await db.close()
    await bot.session.close()


app = FastAPI(title="Ads Marketplace Bot Internal API", lifespan=lifespan)


async def start_polling():
    logger.info("Starting bot long polling...")
    await dp.start_polling(bot)


# ---- Internal API endpoints ----


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.get("/internal/channels/{username}/admins")
async def api_get_admins(username: str):
    """Get channel admins via Bot API."""
    admins = await get_channel_admins(bot, username)
    if not admins:
        raise HTTPException(status_code=404, detail="Channel not found or no admins")
    return admins


@app.get("/internal/channels/{username}/check_admin")
async def api_check_admin(username: str, telegram_user_id: int):
    """Check if a user is an admin with posting rights."""
    result = await check_admin(bot, username, telegram_user_id)
    return result


@app.post("/internal/deals/{deal_id}/post")
async def api_post_to_deal(deal_id: str, req: PostRequest):
    """Post content to a channel for a deal."""
    try:
        result = await schedule_post(
            bot=bot,
            deal_id=deal_id,
            chat_id=req.chat_id,
            text=req.text,
            scheduled_at=req.scheduled_at,
            channel_username=req.channel_username or "",
        )
        return result or {"status": "scheduled"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/internal/notify")
async def api_notify(req: NotifyRequest):
    """Send notification to a user."""
    await send_notification(bot, req.telegram_user_id, req.text)
    return {"status": "ok"}
