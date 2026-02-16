from aiogram import Bot
from aiogram.types import ChatMemberAdministrator, ChatMemberOwner
from typing import Optional


async def get_channel_admins(bot: Bot, channel_username: str) -> list[dict]:
    """Get list of admins for a channel."""
    try:
        chat = await bot.get_chat(f"@{channel_username}")
        admins = await bot.get_chat_administrators(chat.id)
    except Exception:
        return []

    result = []
    for admin in admins:
        if admin.user.is_bot:
            continue

        info = {
            "telegram_user_id": admin.user.id,
            "username": admin.user.username or "",
            "display_name": admin.user.full_name,
            "can_post_messages": False,
            "is_owner": False,
        }

        if isinstance(admin, ChatMemberOwner):
            info["is_owner"] = True
            info["can_post_messages"] = True
        elif isinstance(admin, ChatMemberAdministrator):
            info["can_post_messages"] = admin.can_post_messages or False

        result.append(info)

    return result


async def check_admin(bot: Bot, channel_username: str, telegram_user_id: int) -> Optional[dict]:
    """Check if a specific user is an admin with posting rights."""
    admins = await get_channel_admins(bot, channel_username)
    for admin in admins:
        if admin["telegram_user_id"] == telegram_user_id:
            return {
                "is_admin": True,
                "can_post_messages": admin["can_post_messages"],
            }
    return {"is_admin": False, "can_post_messages": False}
