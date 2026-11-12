import logging
import os
from dotenv import load_dotenv

load_dotenv()

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")

DATABASE_URL = os.environ["DATABASE_URL"]
ANALYTICS_API_SECRET = os.environ.get("ANALYTICS_API_SECRET", "")

# Example values from old compose files and .env.example; never accepted.
_PLACEHOLDER_SECRETS = {
    "local-analytics-secret-change-me",
    "replace-me-generate-analytics-secret-32ch",
    "changeme",
    "change-me",
}
MIN_SECRET_LEN = 16


def analytics_secret_problem(secret: str) -> "str | None":
    """Why the shared secret is unusable, or None when it is fine."""
    s = secret.strip()
    if not s:
        return "ANALYTICS_API_SECRET is not set; generate one with: openssl rand -hex 32"
    if s.lower() in _PLACEHOLDER_SECRETS:
        return "ANALYTICS_API_SECRET is still the example placeholder; generate one with: openssl rand -hex 32"
    if len(s) < MIN_SECRET_LEN:
        return f"ANALYTICS_API_SECRET must be at least {MIN_SECRET_LEN} characters"
    return None
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
