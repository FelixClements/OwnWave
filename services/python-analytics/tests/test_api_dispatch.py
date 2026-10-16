import uuid
from contextlib import contextmanager

import pytest
from fastapi import HTTPException


@pytest.fixture
def api_with_fake_db(monkeypatch, tmp_path):
    import api

    monkeypatch.setattr(api, "MUSIC_DIR", str(tmp_path))
    job_id = uuid.uuid4()
    updates = []

    @contextmanager
    def fake_conn():
        class Conn:
            def commit(self):
                pass

        yield Conn()

    monkeypatch.setattr(api, "_db_conn", fake_conn)
    monkeypatch.setattr(api.db, "create_scan_job", lambda conn, path: job_id)
    monkeypatch.setattr(
        api.db, "update_scan_job", lambda conn, jid, status, error=None: updates.append((jid, status, error))
    )
    return api, job_id, updates


def test_scan_queues_celery_task(api_with_fake_db, monkeypatch, tmp_path):
    api, job_id, updates = api_with_fake_db
    calls = []
    monkeypatch.setattr(
        api.tasks.trigger_library_scan, "apply_async", lambda args, **opts: calls.append((args, opts))
    )

    resp = api.scan(api.ScanRequest(path=str(tmp_path), force=True))

    assert resp == {"job_id": str(job_id), "status": "queued"}
    assert calls == [((str(job_id), str(tmp_path.resolve()), True), {"ignore_result": True})]
    assert updates == []


def test_scan_fails_job_when_broker_down(api_with_fake_db, monkeypatch, tmp_path):
    api, job_id, updates = api_with_fake_db

    def broker_down(*args, **kwargs):
        raise ConnectionError("redis unreachable")

    monkeypatch.setattr(api.tasks.trigger_library_scan, "apply_async", broker_down)

    with pytest.raises(HTTPException) as exc:
        api.scan(api.ScanRequest(path=str(tmp_path)))

    assert exc.value.status_code == 503
    assert updates == [(job_id, "failed", "task queue unavailable")]


def test_rebuild_returns_503_when_broker_down(api_with_fake_db, monkeypatch):
    api, _, _ = api_with_fake_db

    def broker_down(*args, **kwargs):
        raise ConnectionError("redis unreachable")

    monkeypatch.setattr(api.tasks.rebuild_feature_vectors, "apply_async", broker_down)

    with pytest.raises(HTTPException) as exc:
        api.rebuild_vectors()

    assert exc.value.status_code == 503
