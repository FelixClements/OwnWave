import time
from types import SimpleNamespace

import watcher


def event(path):
    return SimpleNamespace(is_directory=False, event_type="created", src_path=path, dest_path=None)


def test_changes_during_cooldown_are_deferred_not_dropped(monkeypatch, tmp_path):
    sent = []
    monkeypatch.setattr(watcher, "queue_scan", sent.append)
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


def test_steady_changes_wait_until_quiet(monkeypatch, tmp_path):
    sent = []
    monkeypatch.setattr(watcher, "queue_scan", sent.append)
    handler = watcher.MusicEventHandler(str(tmp_path), debounce_seconds=0.2, cooldown_seconds=0)

    for i in range(10):
        handler.on_any_event(event(str(tmp_path / f"{i}.flac")))
        time.sleep(0.05)
    assert sent == []

    time.sleep(0.4)
    assert len(sent) == 1


def test_changes_that_never_go_quiet_still_queue_a_scan(monkeypatch, tmp_path):
    sent = []
    monkeypatch.setattr(watcher, "queue_scan", sent.append)
    handler = watcher.MusicEventHandler(
        str(tmp_path), debounce_seconds=0.2, cooldown_seconds=0, max_wait_seconds=0.3
    )

    for i in range(12):
        handler.on_any_event(event(str(tmp_path / f"{i}.flac")))
        time.sleep(0.05)
    assert len(sent) >= 1


def test_non_music_files_are_ignored(monkeypatch, tmp_path):
    sent = []
    monkeypatch.setattr(watcher, "queue_scan", sent.append)
    handler = watcher.MusicEventHandler(str(tmp_path), debounce_seconds=0.01, cooldown_seconds=0)
    handler.on_any_event(event(str(tmp_path / "cover.jpg")))
    time.sleep(0.05)
    assert sent == []


def test_reads_are_ignored(monkeypatch, tmp_path):
    sent = []
    monkeypatch.setattr(watcher, "queue_scan", sent.append)
    handler = watcher.MusicEventHandler(str(tmp_path), debounce_seconds=0.01, cooldown_seconds=0)
    for event_type in ("opened", "closed_no_write"):
        handler.on_any_event(
            SimpleNamespace(is_directory=False, event_type=event_type, src_path=str(tmp_path / "a.flac"), dest_path=None)
        )
    time.sleep(0.05)
    assert sent == []


def watch(monkeypatch, path):
    """Start a real observer on path; returns it with what its handler received and queued."""
    seen, sent = [], []
    original = watcher.MusicEventHandler.on_any_event

    def spy(self, event):
        seen.append(event.event_type)
        original(self, event)

    monkeypatch.setattr(watcher.MusicEventHandler, "on_any_event", spy)
    monkeypatch.setattr(watcher, "queue_scan", sent.append)
    return watcher.start_watching(str(path), debounce_seconds=0.01, cooldown_seconds=0), seen, sent


def wait_for(condition, timeout=3.0):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline and not condition():
        time.sleep(0.02)
    return condition()


def test_reading_files_is_not_watched(monkeypatch, tmp_path):
    song = tmp_path / "album" / "a.flac"
    song.parent.mkdir()
    song.write_bytes(b"x")
    observer, seen, sent = watch(monkeypatch, tmp_path)
    try:
        song.read_bytes()
        list(tmp_path.rglob("*"))
        time.sleep(0.3)
    finally:
        observer.stop()
        observer.join()
    assert seen == []
    assert sent == []


def test_library_changes_still_queue_a_scan(monkeypatch, tmp_path):
    (tmp_path / "album").mkdir()
    observer, _, sent = watch(monkeypatch, tmp_path)
    try:
        # A new file in a folder that existed when watching started.
        (tmp_path / "album" / "new.flac").write_bytes(b"x")
        assert wait_for(lambda: len(sent) == 1)
        # A file renamed.
        sent.clear()
        (tmp_path / "album" / "new.flac").rename(tmp_path / "album" / "renamed.flac")
        assert wait_for(lambda: len(sent) == 1)
        # A file removed.
        sent.clear()
        (tmp_path / "album" / "renamed.flac").unlink()
        assert wait_for(lambda: len(sent) == 1)
        # A new file in a folder made after watching started.
        sent.clear()
        (tmp_path / "later").mkdir()
        time.sleep(0.1)
        (tmp_path / "later" / "b.flac").write_bytes(b"x")
        assert wait_for(lambda: len(sent) == 1)
        # A whole album folder moved in from outside the library.
        sent.clear()
        outside = tmp_path.parent / f"{tmp_path.name}-outside"
        outside.mkdir()
        (outside / "c.flac").write_bytes(b"x")
        outside.rename(tmp_path / "moved-in")
        assert wait_for(lambda: len(sent) == 1)
    finally:
        observer.stop()
        observer.join()
