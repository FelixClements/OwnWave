import logging
import os
from dotenv import load_dotenv

load_dotenv()

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")

DATABASE_URL = os.environ["DATABASE_URL"]
ANALYTICS_API_SECRET = os.environ.get("ANALYTICS_API_SECRET", "")
MUSIC_DIR = os.environ.get("MUSIC_DIR", "/music")

# Audio genre model (Discogs400). Off by default: it needs the model files.
ENABLE_GENRE_ANALYSIS = os.environ.get("ENABLE_GENRE_ANALYSIS", "false").lower() == "true"
GENRE_MODEL_DIR = os.environ.get("GENRE_MODEL_DIR", "/app/models")
# Length of each of the three analysis windows (at 20%, 50% and 75% of a track).
GENRE_WINDOW_SECONDS = float(os.environ.get("GENRE_WINDOW_SECONDS", "20"))

# Artist genre tags from musicbrainz.org (sends artist names to MusicBrainz).
MUSICBRAINZ_ENABLED = os.environ.get("MUSICBRAINZ_ENABLED", "true").lower() == "true"
MUSICBRAINZ_URL = os.environ.get("MUSICBRAINZ_URL", "https://musicbrainz.org")
MUSICBRAINZ_USER_AGENT = os.environ.get(
    "MUSICBRAINZ_USER_AGENT", "OwnWave/1.0 ( https://github.com/FelixClements/OwnWave )"
)
