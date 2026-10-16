import time
from types import SimpleNamespace

import watcher


def event(path):
    return SimpleNamespace(is_directory=False, event_type="created", src_path=path, dest_path=None)


def test_changes_during_cooldown_are_deferred_not_dropped(monkeypatch, tmp_path):
    sent = []
    monkeypatch.setattr(watcher.celery_app, "send_task", lambda *a, **kw: sent.append(a))
    handler = watcher.MusicEventHandler(str(tmp_path), debounce_seconds=0.05, cooldown_seconds=0.3)

    handler.on_any_event(event(str(tmp_path / "a.flac")))
    time.sleep(0.15)
    assert len(sent) == 1

    # Arrives inside the cooldown window: must trigger once it ends.
    handler.on_any_event(event(str(tmp_path / "b.flac")))
    time.sleep(0.1)
    assert len(sent) == 1
    time.sleep(0.4)
    assert len(sent) == 2


def test_non_music_files_are_ignored(monkeypatch, tmp_path):
    sent = []
    monkeypatch.setattr(watcher.celery_app, "send_task", lambda *a, **kw: sent.append(a))
    handler = watcher.MusicEventHandler(str(tmp_path), debounce_seconds=0.01, cooldown_seconds=0)
    handler.on_any_event(event(str(tmp_path / "cover.jpg")))
    time.sleep(0.05)
    assert sent == []
