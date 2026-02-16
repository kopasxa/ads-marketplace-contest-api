"""
Channel statistics collection via Pyrogram userbot.
Uses GetBroadcastStats (native Telegram stats, requires admin + ≥500 subs)
with fallback to GetFullChannel + message history for smaller channels.
"""

import logging
from datetime import datetime, timezone
from typing import Optional

from pyrogram import Client
from pyrogram.raw.functions.channels import GetFullChannel
from pyrogram.raw.functions.stats import GetBroadcastStats
from pyrogram.raw.types import InputChannel, ChannelFull

logger = logging.getLogger(__name__)


class ChannelStatsResult:
    """Rich channel statistics collected via userbot."""

    def __init__(self):
        self.subscribers: Optional[int] = None
        self.admins_count: Optional[int] = None
        self.members_online: Optional[int] = None
        self.posts_count: Optional[int] = None
        self.verified: bool = False
        self.title: Optional[str] = None
        self.username: Optional[str] = None
        self.description: Optional[str] = None
        self.avg_views_20: Optional[int] = None
        self.growth_7d: Optional[int] = None
        self.growth_30d: Optional[int] = None
        self.fetched_at: datetime = datetime.now(timezone.utc)
        self.source: str = "userbot"
        # Extended fields from GetBroadcastStats
        self.views_per_post: Optional[float] = None
        self.shares_per_post: Optional[float] = None
        self.enabled_notifications_percent: Optional[float] = None
        self.er_percent: Optional[float] = None

    def to_dict(self) -> dict:
        return {
            "subscribers": self.subscribers,
            "admins_count": self.admins_count,
            "members_online": self.members_online,
            "posts_count": self.posts_count,
            "verified": self.verified,
            "title": self.title,
            "username": self.username,
            "description": self.description,
            "avg_views_20": self.avg_views_20,
            "growth_7d": self.growth_7d,
            "growth_30d": self.growth_30d,
            "fetched_at": self.fetched_at.isoformat(),
            "source": self.source,
            "views_per_post": self.views_per_post,
            "shares_per_post": self.shares_per_post,
            "enabled_notifications_percent": self.enabled_notifications_percent,
            "er_percent": self.er_percent,
        }


async def collect_channel_stats(
    client: Client, channel_identifier: str | int
) -> ChannelStatsResult:
    """
    Collect full channel statistics using Pyrogram.

    Strategy:
    1. get_chat() for basic info (title, username, members_count)
    2. GetFullChannel for admins_count, online_count
    3. GetBroadcastStats for rich stats (requires admin + ≥500 subs)
    4. Fallback to message history for avg_views if BroadcastStats unavailable
    """
    result = ChannelStatsResult()

    # Resolve the channel peer
    peer = await client.resolve_peer(channel_identifier)
    chat = await client.get_chat(channel_identifier)

    result.title = chat.title
    result.username = chat.username
    result.subscribers = chat.members_count
    result.description = getattr(chat, "description", None)

    input_channel = None

    # GetFullChannel — admins_count, online_count
    try:
        if hasattr(peer, "channel_id") and hasattr(peer, "access_hash"):
            input_channel = InputChannel(
                channel_id=peer.channel_id,
                access_hash=peer.access_hash,
            )
            full = await client.invoke(GetFullChannel(channel=input_channel))

            if hasattr(full, "full_chat") and isinstance(full.full_chat, ChannelFull):
                fc = full.full_chat
                result.subscribers = fc.participants_count
                result.admins_count = fc.admins_count
                result.members_online = getattr(fc, "online_count", None)
    except Exception as e:
        logger.warning(f"GetFullChannel failed for {channel_identifier}: {e}")

    # GetBroadcastStats — rich native stats (requires admin + ≥500 subs)
    broadcast_stats_ok = False
    if input_channel is not None:
        try:
            stats = await client.invoke(
                GetBroadcastStats(channel=input_channel, dark=False)
            )
            broadcast_stats_ok = True

            # Follower growth: current - previous gives the delta for the period
            if hasattr(stats, "followers") and stats.followers:
                current = getattr(stats.followers, "current", 0)
                previous = getattr(stats.followers, "previous", 0)
                delta = current - previous
                # Use the period's delta as approximate growth
                # The period is typically ~7 days
                result.growth_7d = delta

            # Views per post
            if hasattr(stats, "views_per_post") and stats.views_per_post:
                vpp = getattr(stats.views_per_post, "current", None)
                if vpp is not None:
                    result.views_per_post = round(vpp, 1)
                    result.avg_views_20 = int(vpp)

            # Shares per post
            if hasattr(stats, "shares_per_post") and stats.shares_per_post:
                spp = getattr(stats.shares_per_post, "current", None)
                if spp is not None:
                    result.shares_per_post = round(spp, 1)

            # Enabled notifications percentage
            if hasattr(stats, "enabled_notifications") and stats.enabled_notifications:
                part = getattr(stats.enabled_notifications, "part", 0)
                total = getattr(stats.enabled_notifications, "total", 0)
                if total > 0:
                    result.enabled_notifications_percent = round(
                        (part / total) * 100, 2
                    )

            # ER% = (views_per_post / subscribers) * 100
            if result.views_per_post and result.subscribers and result.subscribers > 0:
                result.er_percent = round(
                    (result.views_per_post / result.subscribers) * 100, 2
                )

            # Avg views from recent_message_interactions (more accurate)
            if hasattr(stats, "recent_message_interactions"):
                interactions = stats.recent_message_interactions or []
                if interactions:
                    total_views = sum(
                        getattr(m, "views", 0) for m in interactions
                    )
                    result.avg_views_20 = total_views // len(interactions)

                    # Posts count from the most recent message ID
                    first_msg = interactions[0]
                    msg_id = getattr(first_msg, "msg_id", None)
                    if msg_id:
                        result.posts_count = msg_id

            logger.info(
                f"GetBroadcastStats OK for {channel_identifier}: "
                f"subs={result.subscribers}, views/post={result.views_per_post}, "
                f"ER={result.er_percent}%"
            )

        except Exception as e:
            logger.info(
                f"GetBroadcastStats unavailable for {channel_identifier}: {e} "
                f"(falling back to message history)"
            )

    # Fallback: compute avg views from message history
    if not broadcast_stats_ok:
        try:
            messages = []
            async for msg in client.get_chat_history(chat.id, limit=20):
                if msg.views is not None:
                    messages.append(msg)

            if messages:
                total_views = sum(m.views for m in messages if m.views)
                result.avg_views_20 = (
                    total_views // len(messages) if messages else None
                )

                if messages[0].id:
                    result.posts_count = messages[0].id

                # ER% from manual avg views
                if (
                    result.avg_views_20
                    and result.subscribers
                    and result.subscribers > 0
                ):
                    result.er_percent = round(
                        (result.avg_views_20 / result.subscribers) * 100, 2
                    )

        except Exception as e:
            logger.warning(
                f"Failed to get message history for {channel_identifier}: {e}"
            )

    return result
