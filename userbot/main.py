#!/usr/bin/env python3
"""
Ads Marketplace Userbot Service
- Runs a Pyrogram userbot for rich channel statistics collection
- Exposes internal HTTP API for the Go backend and stats worker
"""

import logging
import uvicorn

from userbot.config import config
from userbot.server import app

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)

if __name__ == "__main__":
    uvicorn.run(
        app,
        host="0.0.0.0",
        port=config.USERBOT_PORT,
        log_level="info",
    )
