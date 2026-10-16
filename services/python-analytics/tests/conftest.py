import os
from pathlib import Path

import pytest

MIGRATIONS_DIR = Path(__file__).resolve().parents[3] / "database" / "migrations"


@pytest.fixture
def db_url(monkeypatch):
    """A freshly migrated database. Set TEST_DATABASE_URL to enable DB tests."""
    url = os.environ.get("TEST_DATABASE_URL")
    if not url:
        pytest.skip("TEST_DATABASE_URL not set")

    import psycopg

    with psycopg.connect(url, autocommit=True) as conn:
        conn.execute("DROP SCHEMA public CASCADE")
        conn.execute("CREATE SCHEMA public")
        for migration in sorted(MIGRATIONS_DIR.glob("*.up.sql")):
            conn.execute(migration.read_text())

    import db

    monkeypatch.setattr(db, "DATABASE_URL", url)
    return url
