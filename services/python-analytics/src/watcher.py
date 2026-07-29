import threading
import time
from pathlib import Path

from watchdog.events import FileSystemEventHandler
from watchdog.observers import Observer

from celery_app import celery_app
from config import MUSIC_DIR


class MusicEventHandler(FileSystemEventHandler):
    def __init__(self, root_path: str, debounce_seconds: float = 5.0):
        self.root_path = Path(root_path).expanduser().resolve()
        self.debounce_seconds = debounce_seconds
        self._timer: threading.Timer | None = None
        self._lock = threading.Lock()

    def on_any_event(self, event):
        if event.is_directory:
            return
        if not event.src_path or not self._is_music_file(event.src_path):
            return

        with self._lock:
            if self._timer is not None:
                self._timer.cancel()

            self._timer = threading.Timer(self.debounce_seconds, self._trigger_scan)
            self._timer.daemon = True
            self._timer.start()

    def _is_music_file(self, path: str) -> bool:
        return Path(path).suffix.lower() in {".flac", ".mp3"}

    def _trigger_scan(self):
        print(f"[watcher] change detected, queuing scan of {self.root_path}")
        try:
            celery_app.send_task(
                "tasks.trigger_library_scan",
                args=[None, str(self.root_path)],
            )
        except Exception as exc:
            print(f"[watcher] failed to queue scan: {exc}")


def watch_directory(path: str = MUSIC_DIR, debounce_seconds: float = 5.0):
    root = Path(path).expanduser().resolve()
    if not root.exists():
        raise FileNotFoundError(f"Music path does not exist: {root}")

    event_handler = MusicEventHandler(str(root), debounce_seconds=debounce_seconds)
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
