import logging
import httpx
from aiogram import Bot, Router, F
from aiogram.types import ChatMemberUpdated, Message, ChatPermissions
from aiogram.filters import ChatMemberUpdatedFilter, IS_NOT_MEMBER, IS_MEMBER, ADMINISTRATOR

from bot.config import config
from bot.db import db

logger = logging.getLogger(__name__)
router = Router()


# ────────────────────────────────────────────
# Bot added / removed from channel
# ────────────────────────────────────────────


@router.my_chat_member(ChatMemberUpdatedFilter(member_status_changed=IS_NOT_MEMBER >> ADMINISTRATOR))
async def bot_added_to_channel(event: ChatMemberUpdated, bot: Bot):
    """Bot was added as admin to a channel."""
    chat = event.chat
    from_user = event.from_user

    logger.info(
        f"Bot added to channel @{chat.username} (id={chat.id}) "
        f"by user {from_user.id} ({from_user.username})"
    )

    if chat.type not in ("channel",):
        logger.info(f"Ignoring non-channel chat type: {chat.type}")
        return

    if not chat.username:
        logger.info(f"Ignoring private channel {chat.id}")
        return

    # Upsert user who added the bot
    user_uuid = await db.upsert_user(
        telegram_user_id=from_user.id,
        username=from_user.username,
        first_name=from_user.first_name,
        last_name=from_user.last_name,
    )

    # Upsert channel
    await db.upsert_channel(
        telegram_chat_id=chat.id,
        username=chat.username.lower(),
        title=chat.title or "",
        added_by_user_id=user_uuid,
        bot_status="active",
    )

    # Add user as owner in channel_members
    await db.add_channel_member(
        channel_username=chat.username.lower(),
        user_id=user_uuid,
        role="owner",
        can_post=True,
    )

    logger.info(f"Channel @{chat.username} registered, owner: {from_user.id}")

    # Try to add userbot for stats collection
    # Check if bot has can_invite_users right
    try:
        bot_member = event.new_chat_member
        can_invite = getattr(bot_member, "can_invite_users", False)

        if can_invite:
            await _try_add_userbot(bot, chat.id, chat.username)
        else:
            logger.info(
                f"Bot doesn't have can_invite_users right in @{chat.username}, "
                f"skipping userbot addition"
            )
    except Exception as e:
        logger.warning(f"Failed to check/add userbot for @{chat.username}: {e}")


@router.my_chat_member(ChatMemberUpdatedFilter(member_status_changed=ADMINISTRATOR >> IS_NOT_MEMBER))
async def bot_removed_from_channel(event: ChatMemberUpdated, bot: Bot):
    """Bot was removed from a channel — deactivate channel + cancel active deals."""
    chat = event.chat
    username = (chat.username or "").lower()

    logger.info(f"Bot removed from channel @{username} (id={chat.id})")

    if not username:
        return

    # Sync latest title/username before deactivation
    await _sync_channel_info(chat)
    await _handle_bot_removed(username, bot)


@router.my_chat_member(ChatMemberUpdatedFilter(member_status_changed=IS_MEMBER >> IS_NOT_MEMBER))
async def bot_kicked_from_channel(event: ChatMemberUpdated, bot: Bot):
    """Bot was kicked from a channel — same logic as removal."""
    chat = event.chat
    username = (chat.username or "").lower()

    logger.info(f"Bot kicked from channel @{username} (id={chat.id})")

    if not username:
        return

    # Sync latest title/username before deactivation
    await _sync_channel_info(chat)
    await _handle_bot_removed(username, bot)


async def _try_add_userbot(bot: Bot, chat_id: int, channel_username: str):
    """
    Try to add the userbot as admin to the channel for stats collection.
    1. Get userbot's telegram user ID from the userbot service
    2. Have the userbot join the channel
    3. Promote userbot as admin with minimal rights
    4. Update userbot_status in DB
    """
    try:
        async with httpx.AsyncClient(timeout=10) as client:
            # Get userbot info
            resp = await client.get(f"{config.USERBOT_INTERNAL_URL}/internal/me")
            if resp.status_code != 200:
                logger.warning(f"Userbot service unavailable: {resp.status_code}")
                await db.update_userbot_status(channel_username.lower(), "failed")
                return

            userbot_info = resp.json()
            userbot_user_id = userbot_info["user_id"]

            # Have the userbot join the channel first
            join_resp = await client.post(
                f"{config.USERBOT_INTERNAL_URL}/internal/join-channel",
                json={"username": channel_username},
            )
            if join_resp.status_code != 200:
                logger.warning(
                    f"Userbot failed to join @{channel_username}: {join_resp.text}"
                )
                await db.update_userbot_status(channel_username.lower(), "failed")
                return

        # Mark as pending
        await db.update_userbot_status(channel_username.lower(), "pending")

        # Promote userbot as admin with minimal rights
        # can_manage_chat=True is the minimum required for admin status
        # (needed for GetBroadcastStats / native Telegram statistics)
        await bot.promote_chat_member(
            chat_id=chat_id,
            user_id=userbot_user_id,
            can_manage_chat=True,
            can_post_messages=False,
            can_edit_messages=False,
            can_delete_messages=False,
            can_invite_users=False,
            can_restrict_members=False,
            can_pin_messages=False,
            can_promote_members=False,
            can_manage_video_chats=False,
        )

        await db.update_userbot_status(channel_username.lower(), "active")
        logger.info(
            f"Userbot (id={userbot_user_id}) promoted in @{channel_username}"
        )

    except Exception as e:
        logger.warning(f"Failed to add userbot to @{channel_username}: {e}")
        await db.update_userbot_status(channel_username.lower(), "failed")


async def _handle_bot_removed(username: str, bot: Bot):
    """Deactivate channel and cancel all active deals with refund."""
    # 1. Mark channel as removed (also reset userbot status)
    await db.update_channel_bot_removed(username)
    await db.update_userbot_status(username, "removed")

    # 2. Find and cancel all active deals for this channel
    active_deals = await db.get_active_deals_by_channel_username(username)

    for deal in active_deals:
        deal_id = str(deal["id"])
        deal_status = deal["status"]
        advertiser_id = deal["advertiser_user_id"]

        logger.info(
            f"Cancelling deal {deal_id} (status={deal_status}) "
            f"due to bot removal from @{username}"
        )

        # Cancel the deal
        cancelled = await db.cancel_deal_system(deal_id)
        if not cancelled:
            logger.warning(f"Could not cancel deal {deal_id}, status: {deal_status}")
            continue

        # Log audit event
        await db.log_audit(
            action="deal_cancelled_bot_removed",
            entity_type="deal",
            entity_id=deal_id,
            actor_type="system",
            details=f"Bot removed from channel @{username}",
        )

        # If the deal was funded, transition to refunded
        funded_statuses = (
            "funded",
            "creative_pending",
            "creative_submitted",
            "creative_changes_requested",
            "creative_approved",
            "scheduled",
            "posted",
            "hold_verification",
        )
        if deal_status in funded_statuses:
            refunded = await db.refund_deal_system(deal_id)
            if refunded:
                await db.log_audit(
                    action="deal_refunded_bot_removed",
                    entity_type="deal",
                    entity_id=deal_id,
                    actor_type="system",
                    details=f"Auto-refund: bot removed from @{username}",
                )
                logger.info(f"Deal {deal_id} refunded due to bot removal")

        # Notify advertiser (resolve telegram_user_id from UUID)
        try:
            from bot.tasks import send_notification

            tg_user_id = await db.get_telegram_id_by_user_uuid(str(advertiser_id))
            if tg_user_id:
                await send_notification(
                    bot,
                    tg_user_id,
                    f"⚠️ Deal cancelled: the bot was removed from channel @{username}. "
                    f"Your deal has been cancelled and a refund has been initiated.",
                )
        except Exception as e:
            logger.error(f"Failed to notify advertiser for deal {deal_id}: {e}")

    if active_deals:
        logger.info(
            f"Processed {len(active_deals)} active deals for removed channel @{username}"
        )


# ────────────────────────────────────────────
# Channel info changes (title, username)
# ────────────────────────────────────────────


async def _sync_channel_info(chat):
    """Sync channel username and title from a Telegram Chat object."""
    if chat.type != "channel" or not chat.id:
        return

    new_username = chat.username.lower() if chat.username else None
    new_title = chat.title or None

    if new_username or new_title:
        await db.update_channel_info(
            telegram_chat_id=chat.id,
            username=new_username,
            title=new_title,
        )
        logger.info(
            f"Synced channel info: chat_id={chat.id}, "
            f"username=@{chat.username}, title={chat.title}"
        )


@router.my_chat_member()
async def channel_info_updated_on_member_change(event: ChatMemberUpdated, bot: Bot):
    """
    Catch-all for any other my_chat_member updates (e.g. permission changes).
    Syncs channel username/title from the chat object.
    """
    await _sync_channel_info(event.chat)


@router.channel_post(F.new_chat_title)
async def channel_title_changed(message: Message, bot: Bot):
    """Channel title was changed — update in DB."""
    chat = message.chat
    new_title = message.new_chat_title

    logger.info(
        f"Channel title changed: chat_id={chat.id}, "
        f"@{chat.username} -> title='{new_title}'"
    )

    if chat.id:
        await db.update_channel_info(
            telegram_chat_id=chat.id,
            title=new_title,
            username=chat.username.lower() if chat.username else None,
        )
