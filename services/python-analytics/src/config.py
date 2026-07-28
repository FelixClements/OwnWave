import os
from dotenv import load_dotenv

load_dotenv()

DATABASE_URL = os.environ["DATABASE_URL"]
MUSIC_DIR = os.environ.get("MUSIC_DIR", "/music")
