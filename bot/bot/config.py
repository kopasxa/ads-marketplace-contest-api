import os
from dotenv import load_dotenv

load_dotenv()


class Config:
    BOT_TOKEN: str = os.getenv("BOT_TOKEN", "")
    POSTGRES_DSN: str = os.getenv(
        "POSTGRES_DSN",
        "postgresql://postgres:postgres@localhost:5432/ads_marketplace",
    )
    REDIS_URL: str = os.getenv("REDIS_URL", "redis://localhost:6379/0")
    INTERNAL_API_PORT: int = int(os.getenv("BOT_INTERNAL_PORT", "8081"))
    WEBHOOK_URL: str = os.getenv("WEBHOOK_URL", "")
    USE_WEBHOOK: bool = os.getenv("USE_WEBHOOK", "false").lower() == "true"
    USERBOT_INTERNAL_URL: str = os.getenv("USERBOT_INTERNAL_URL", "http://localhost:8082")


config = Config()
