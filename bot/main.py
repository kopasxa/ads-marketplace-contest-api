#!/usr/bin/env python3
"""
Ads Marketplace Bot Service
- Handles Telegram raw updates (bot added/removed from channels)
- Provides internal HTTP API for the Go backend
"""

import logging
import uvicorn

from bot.config import config
from bot.telegram import app

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)

if __name__ == "__main__":
    uvicorn.run(
        app,
        host="0.0.0.0",
        port=config.INTERNAL_API_PORT,
        log_level="info",
    )
