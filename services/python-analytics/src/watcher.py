import threading
import time
from pathlib import Path

from watchdog.events import FileSystemEventHandler
from watchdog.observers import Observer

from celery_app import celery_app
from config import MUSIC_DIR


MUSIC_EXTS = {".flac", ".mp3"}


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
        if event.is_directory:
            return
        if event.event_type == "closed":
            return
        path = getattr(event, "dest_path", None) or getattr(event, "src_path", None)
        if not path or not self._is_music_file(path):
            return

        with self._lock:
            now = time.monotonic()
            if now < self._next_allowed:
                return

            self._dirty = True
            if self._timer is not None:
                self._timer.cancel()

            self._timer = threading.Timer(self.debounce_seconds, self._maybe_trigger)
            self._timer.daemon = True
            self._timer.start()

    def _is_music_file(self, path: str) -> bool:
        return Path(path).suffix.lower() in MUSIC_EXTS

    def _maybe_trigger(self):
        with self._lock:
            self._timer = None
            if not self._dirty:
                return
            self._dirty = False
            now = time.monotonic()
            if now < self._next_allowed:
                return
            self._next_allowed = now + self.cooldown_seconds

        print(f"[watcher] change detected, queuing scan of {self.root_path}")
        try:
            celery_app.send_task(
                "tasks.trigger_library_scan",
                args=[None, str(self.root_path)],
            )
        except Exception as exc:
            print(f"[watcher] failed to queue scan: {exc}")


def watch_directory(path: str = MUSIC_DIR, debounce_seconds: float = 2.0, cooldown_seconds: float = 30.0):
    root = Path(path).expanduser().resolve()
    if not root.exists():
        raise FileNotFoundError(f"Music path does not exist: {root}")

    event_handler = MusicEventHandler(str(root), debounce_seconds=debounce_seconds, cooldown_seconds=cooldown_seconds)
    observer = Observer()
    observer.schedule(event_handler, str(root), recursive=True)
    observer.start()

    print(f"[watcher] watching {root} for music file changes")
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        observer.stop()
    observer.join()
