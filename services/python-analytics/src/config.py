import logging
import os
from dotenv import load_dotenv

load_dotenv()

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")

DATABASE_URL = os.environ["DATABASE_URL"]
ANALYTICS_API_SECRET = os.environ.get("ANALYTICS_API_SECRET", "")
MUSIC_DIR = os.environ.get("MUSIC_DIR", "/music")

ENABLE_GENRE_ANALYSIS = os.environ.get("ENABLE_GENRE_ANALYSIS", "false").lower() == "true"
GENRE_MODEL_DIR = os.environ.get("GENRE_MODEL_DIR", "/app/models")
GENRE_SNIPPET_START = float(os.environ.get("GENRE_SNIPPET_START", "30"))
GENRE_SNIPPET_DURATION = float(os.environ.get("GENRE_SNIPPET_DURATION", "30"))
GENRE_MIN_CONFIDENCE = float(os.environ.get("GENRE_MIN_CONFIDENCE", "0.3"))
GENRE_MIN_TRACKS_PER_STATION = int(os.environ.get("GENRE_MIN_TRACKS_PER_STATION", "5"))
GENRE_FULL_TRACK_MODE = os.environ.get("GENRE_FULL_TRACK_MODE", "false").lower() == "true"
