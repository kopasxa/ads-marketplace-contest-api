import logging
from pyrogram import Client
from userbot.config import config

logger = logging.getLogger(__name__)

_userbot: Client | None = None


def get_client() -> Client:
    """Get or create the Pyrogram userbot client."""
    global _userbot
    if _userbot is None:
        if config.SESSION_STRING:
            _userbot = Client(
                "userbot_session",
                api_id=config.API_ID,
                api_hash=config.API_HASH,
                session_string=config.SESSION_STRING,
                
            )
        elif config.PHONE_NUMBER:
            _userbot = Client(
                "userbot_session",
                api_id=config.API_ID,
                api_hash=config.API_HASH,
                phone_number=config.PHONE_NUMBER,
            )
        else:
            raise RuntimeError(
                "Either TG_SESSION_STRING or TG_PHONE_NUMBER must be set"
            )
    return _userbot


async def start_client():
    """Start the Pyrogram client."""
    client = get_client()
    if not client.is_connected:
        await client.start()
        me = await client.get_me()
        logger.info(f"Userbot started as @{me.username} (id={me.id})")
    return client


async def stop_client():
    """Stop the Pyrogram client."""
    global _userbot
    if _userbot and _userbot.is_connected:
        await _userbot.stop()
        logger.info("Userbot stopped")
