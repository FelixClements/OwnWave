import logging
import threading
import time
from pathlib import Path

from watchdog.events import FileSystemEventHandler
from watchdog.observers import Observer

from audio_metadata import SUPPORTED_EXTS
import db
import tasks
from config import MUSIC_DIR

log = logging.getLogger(__name__)

CHANGE_EVENTS = {"created", "deleted", "modified", "moved"}


class MusicEventHandler(FileSystemEventHandler):
    def __init__(self, root_path: str, debounce_seconds: float = 2.0, cooldown_seconds: float = 30.0):
        self.root_path = Path(root_path).expanduser().resolve()
        self.debounce_seconds = debounce_seconds
        self.cooldown_seconds = cooldown_seconds
        self._lock = threading.Lock()
        self._timer: threading.Timer | None = None
        self._dirty = False
        self._next_allowed = 0.0

    def on_any_event(self, event):
        # Reads (opened/closed_no_write), e.g. by a running scan, are not changes.
        if event.is_directory or event.event_type not in CHANGE_EVENTS:
            return
        path = getattr(event, "dest_path", None) or getattr(event, "src_path", None)
        if not path or not self._is_music_file(path):
            return

        with self._lock:
            self._dirty = True
            # Debounce bursts; changes during the cooldown are deferred to its
            # end rather than dropped.
            delay = max(self.debounce_seconds, self._next_allowed - time.monotonic())
            self._schedule(delay)

    def _is_music_file(self, path: str) -> bool:
        return Path(path).suffix.lower() in SUPPORTED_EXTS

    def _schedule(self, delay: float) -> None:
        if self._timer is not None:
            self._timer.cancel()
        self._timer = threading.Timer(delay, self._maybe_trigger)
        self._timer.daemon = True
        self._timer.start()

    def _maybe_trigger(self):
        with self._lock:
            self._timer = None
            if not self._dirty:
                return
            now = time.monotonic()
            if now < self._next_allowed:
                self._schedule(self._next_allowed - now)
                return
            self._dirty = False
            self._next_allowed = now + self.cooldown_seconds

        log.info("change detected, queuing scan of %s", self.root_path)
        try:
            queue_scan(str(self.root_path))
        except Exception as exc:
            log.error("failed to queue scan: %s", exc)


def queue_scan(path: str) -> None:
    with db.get_conn() as conn:
        tasks.queue_library_scan(conn, path)
        conn.commit()


def start_watching(path: str = MUSIC_DIR, debounce_seconds: float = 2.0, cooldown_seconds: float = 30.0) -> Observer:
    """Queue a library scan whenever music files under path change."""
    root = Path(path).expanduser().resolve()
    if not root.exists():
        raise FileNotFoundError(f"Music path does not exist: {root}")

    event_handler = MusicEventHandler(str(root), debounce_seconds=debounce_seconds, cooldown_seconds=cooldown_seconds)
    observer = Observer()
    observer.daemon = True
    observer.schedule(event_handler, str(root), recursive=True)
    observer.start()
    log.info("watching %s for music file changes", root)
    return observer
