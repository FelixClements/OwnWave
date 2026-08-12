import os
from dotenv import load_dotenv

load_dotenv()

DATABASE_URL = os.environ["DATABASE_URL"]
MUSIC_DIR = os.environ.get("MUSIC_DIR", "/music")

REDIS_URL = os.environ.get("REDIS_URL", "redis://localhost:6379/0")
CELERY_BROKER_URL = os.environ.get("CELERY_BROKER_URL", REDIS_URL)
CELERY_RESULT_BACKEND = os.environ.get("CELERY_RESULT_BACKEND", REDIS_URL)

ENABLE_GENRE_ANALYSIS = os.environ.get("ENABLE_GENRE_ANALYSIS", "false").lower() == "true"
GENRE_MODEL_DIR = os.environ.get("GENRE_MODEL_DIR", "/app/models")
GENRE_SNIPPET_START = float(os.environ.get("GENRE_SNIPPET_START", "30"))
GENRE_SNIPPET_DURATION = float(os.environ.get("GENRE_SNIPPET_DURATION", "30"))
GENRE_MIN_CONFIDENCE = float(os.environ.get("GENRE_MIN_CONFIDENCE", "0.3"))
GENRE_MIN_TRACKS_PER_STATION = int(os.environ.get("GENRE_MIN_TRACKS_PER_STATION", "5"))
GENRE_FULL_TRACK_MODE = os.environ.get("GENRE_FULL_TRACK_MODE", "false").lower() == "true"
