import os
from dotenv import load_dotenv

load_dotenv()


class Config:
    # Pyrogram session
    API_ID: int = int(os.getenv("TG_API_ID", "0"))
    API_HASH: str = os.getenv("TG_API_HASH", "")
    SESSION_STRING: str = os.getenv("TG_SESSION_STRING", "")
    PHONE_NUMBER: str = os.getenv("TG_PHONE_NUMBER", "")

    # Database
    POSTGRES_DSN: str = os.getenv(
        "POSTGRES_DSN",
        "postgresql://postgres:postgres@localhost:5432/ads_marketplace",
    )

    # Server
    USERBOT_PORT: int = int(os.getenv("USERBOT_PORT", "8082"))


config = Config()
