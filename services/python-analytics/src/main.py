import argparse

import db
from config import MUSIC_DIR
from db import get_conn, wait_for_db
from scanner import scan_path
from station_builder import build_station


def main():
    parser = argparse.ArgumentParser(description="OwnWave analytics CLI")
    subparsers = parser.add_subparsers(dest="command", required=True)

    scan_parser = subparsers.add_parser("scan", help="Scan a music directory")
    scan_parser.add_argument("path", nargs="?", default=MUSIC_DIR, help="Path to scan")
    scan_parser.add_argument("--force", action="store_true", help="Re-analyze existing tracks")

    scan_celery_parser = subparsers.add_parser("scan-celery", help="Queue a library scan via Celery")
    scan_celery_parser.add_argument("path", nargs="?", default=MUSIC_DIR, help="Path to scan")
    scan_celery_parser.add_argument("--force", action="store_true", help="Re-analyze existing tracks")

    watch_parser = subparsers.add_parser("watch", help="Watch a music directory and queue scans on change")
    watch_parser.add_argument("path", nargs="?", default=MUSIC_DIR, help="Path to watch")

    station_parser = subparsers.add_parser("build-station", help="Build a station queue")
    station_parser.add_argument("name", help="Station name")
    station_parser.add_argument("--length", type=int, default=50)
    station_parser.add_argument("--min-bpm", type=float)
    station_parser.add_argument("--max-bpm", type=float)
    station_parser.add_argument("--min-energy", type=float)
    station_parser.add_argument("--max-energy", type=float)

    serve_parser = subparsers.add_parser("serve", help="Run the API server")
    serve_parser.add_argument("--host", default="0.0.0.0")
    serve_parser.add_argument("--port", type=int, default=8000)

    args = parser.parse_args()

    if args.command == "scan":
        wait_for_db()
        track_ids = scan_path(args.path, force=args.force)
        print(f"Scanned {len(track_ids)} tracks")

    elif args.command == "scan-celery":
        wait_for_db()
        from celery.result import AsyncResult
        from tasks import trigger_library_scan

        with get_conn() as conn:
            job_id = db.create_scan_job(conn, args.path)
            conn.commit()

        res = trigger_library_scan.delay(str(job_id), args.path, args.force)
        # trigger_library_scan returns {"finalize_id": ...}
        result = res.get()
        finalize_id = result.get("finalize_id")
        if finalize_id:
            AsyncResult(finalize_id).get()
        print(f"Celery scan finished: {result}")

    elif args.command == "watch":
        from watcher import watch_directory

        watch_directory(args.path)

    elif args.command == "build-station":
        wait_for_db()
        filters = {}
        if args.min_bpm is not None:
            filters["min_bpm"] = args.min_bpm
        if args.max_bpm is not None:
            filters["max_bpm"] = args.max_bpm
        if args.min_energy is not None:
            filters["min_energy"] = args.min_energy
        if args.max_energy is not None:
            filters["max_energy"] = args.max_energy

        with get_conn() as conn:
            station_id = build_station(conn, args.name, filters or None, args.length)
            print(f"Created station {station_id}")

    elif args.command == "serve":
        import uvicorn
        from api import app

        uvicorn.run(app, host=args.host, port=args.port)


if __name__ == "__main__":
    main()
