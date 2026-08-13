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

    vectors_parser = subparsers.add_parser("rebuild-vectors", help="Rebuild normalized feature vectors for all tracks")
    vectors_parser.add_argument("--celery", action="store_true", help="Queue via Celery")

    clusters_parser = subparsers.add_parser("rebuild-clusters", help="Rebuild track clusters")
    clusters_parser.add_argument("--n-clusters", type=int, help="Number of K-Means clusters")
    clusters_parser.add_argument("--celery", action="store_true", help="Queue via Celery")

    args = parser.parse_args()

    if args.command == "scan":
        wait_for_db()
        result = scan_path(args.path, force=args.force)
        print(f"Scanned {result.to_dict()}")

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

    elif args.command == "rebuild-vectors":
        if args.celery:
            from celery_app import celery_app

            @celery_app.task
            def _rebuild_vectors():
                from feature_vector import backfill_library_feature_vectors
                return backfill_library_feature_vectors()

            task = _rebuild_vectors.delay()
            print(f"Queued rebuild: {task.id}")
        else:
            from feature_vector import backfill_library_feature_vectors

            wait_for_db()
            count = backfill_library_feature_vectors()
            print(f"Rebuilt feature vectors for {count} tracks")

    elif args.command == "rebuild-clusters":
        if args.celery:
            from celery_app import celery_app

            @celery_app.task
            def _rebuild_clusters():
                from clustering import backfill_library_clusters
                return backfill_library_clusters(n_clusters=args.n_clusters)

            task = _rebuild_clusters.delay()
            print(f"Queued rebuild: {task.id}")
        else:
            from clustering import backfill_library_clusters

            wait_for_db()
            result = backfill_library_clusters(n_clusters=args.n_clusters)
            print(f"Rebuilt clusters: {result}")


if __name__ == "__main__":
    main()
