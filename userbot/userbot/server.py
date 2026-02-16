"""
Userbot Internal HTTP API.
Exposes stats collection and admin management endpoints for the Go backend.
"""

import logging
from contextlib import asynccontextmanager
from typing import Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from userbot.client import start_client, stop_client, get_client
from userbot.stats import collect_channel_stats

logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup
    try:
        await start_client()
        logger.info("Userbot client started")
    except Exception as e:
        logger.error(f"Failed to start userbot: {e}")
        # Don't crash — stats will fall back to t.me parser

    yield

    # Shutdown
    await stop_client()


app = FastAPI(title="Ads Marketplace Userbot Service", lifespan=lifespan)


class StatsResponse(BaseModel):
    subscribers: Optional[int] = None
    admins_count: Optional[int] = None
    members_online: Optional[int] = None
    posts_count: Optional[int] = None
    verified: bool = False
    title: Optional[str] = None
    username: Optional[str] = None
    description: Optional[str] = None
    avg_views_20: Optional[int] = None
    growth_7d: Optional[int] = None
    growth_30d: Optional[int] = None
    fetched_at: str = ""
    source: str = "userbot"


class PromoteRequest(BaseModel):
    channel_id: int
    userbot_user_id: int


@app.get("/health")
async def health():
    client = get_client()
    connected = client.is_connected if client else False
    return {"status": "ok", "connected": connected}


@app.get("/internal/stats/by-username/{username}")
async def get_stats_by_username(username: str):
    """Collect channel stats by username."""
    client = get_client()
    if not client or not client.is_connected:
        raise HTTPException(status_code=503, detail="Userbot not connected")

    try:
        stats = await collect_channel_stats(client, f"@{username}")
        return stats.to_dict()
    except Exception as e:
        logger.error(f"Stats collection failed for @{username}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/internal/stats/by-chat-id/{chat_id}")
async def get_stats_by_chat_id(chat_id: int):
    """Collect channel stats by telegram chat_id."""
    client = get_client()
    if not client or not client.is_connected:
        raise HTTPException(status_code=503, detail="Userbot not connected")

    try:
        stats = await collect_channel_stats(client, chat_id)
        return stats.to_dict()
    except Exception as e:
        logger.error(f"Stats collection failed for chat_id={chat_id}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


class JoinChannelRequest(BaseModel):
    username: str


@app.post("/internal/join-channel")
async def join_channel(req: JoinChannelRequest):
    """Have the userbot join a channel by username."""
    client = get_client()
    if not client or not client.is_connected:
        raise HTTPException(status_code=503, detail="Userbot not connected")

    try:
        chat = await client.join_chat(f"@{req.username}")
        return {"ok": True, "chat_id": chat.id}
    except Exception as e:
        logger.error(f"Failed to join @{req.username}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/internal/me")
async def get_me():
    """Get userbot's own Telegram user info."""
    client = get_client()
    if not client or not client.is_connected:
        raise HTTPException(status_code=503, detail="Userbot not connected")

    me = await client.get_me()
    return {
        "user_id": me.id,
        "username": me.username,
        "first_name": me.first_name,
    }
