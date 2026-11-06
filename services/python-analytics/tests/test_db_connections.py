import gc

import numpy as np
from pgvector import Vector
from psycopg.adapt import AdaptersMap

import db


def test_connections_do_not_leak_pgvector_classes(db_url):
    with db.get_conn() as conn:
        conn.execute("SELECT 1")
    gc.collect()
    before = len(AdaptersMap._optimised)

    for _ in range(20):
        with db.get_conn() as conn:
            conn.execute("SELECT 1")
    gc.collect()

    assert len(AdaptersMap._optimised) == before


def test_vectors_round_trip_on_every_connection(db_url):
    value = np.array([0.5, 1.5, 2.5], dtype=np.float32)
    for _ in range(3):
        with db.get_conn() as conn:
            result = conn.execute("SELECT %s::vector", (value,)).fetchone()[0]
        assert isinstance(result, Vector)
        assert np.allclose(result.to_numpy(), value)


def test_connections_share_one_registration(db_url, monkeypatch):
    import pgvector.psycopg

    calls = []
    register = pgvector.psycopg.register_vector
    monkeypatch.setattr(pgvector.psycopg, "register_vector", lambda conn: (calls.append(conn), register(conn)))

    for _ in range(3):
        with db.get_conn():
            pass

    assert len(calls) == 1
