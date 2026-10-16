import argparse

from config import MUSIC_DIR
from db import wait_for_db
from library_scan import scan_library
from library_scan.options import ScanOptions


def main():
    parser = argparse.ArgumentParser(description="OwnWave analytics CLI")
    subparsers = parser.add_subparsers(dest="command", required=True)

    scan_parser = subparsers.add_parser("scan", help="Scan a music directory")
    scan_parser.add_argument("path", nargs="?", default=MUSIC_DIR, help="Path to scan")
    scan_parser.add_argument("--force", action="store_true", help="Re-analyze existing tracks")

    worker_parser = subparsers.add_parser("worker", help="Run queued tasks and watch the music directory")
    worker_parser.add_argument("-c", "--concurrency", type=int, default=2, help="Worker processes")

    serve_parser = subparsers.add_parser("serve", help="Run the API server")
    serve_parser.add_argument("--host", default="0.0.0.0")
    serve_parser.add_argument("--port", type=int, default=8000)

    subparsers.add_parser("rebuild-vectors", help="Rebuild normalized feature vectors for all tracks")

    clusters_parser = subparsers.add_parser("rebuild-clusters", help="Rebuild track clusters")
    clusters_parser.add_argument("--n-clusters", type=int, help="Number of K-Means clusters")

    args = parser.parse_args()

    if args.command == "scan":
        wait_for_db()
        result = scan_library(args.path, options=ScanOptions(force=args.force))
        print(f"Scanned {result.to_dict()}")

    elif args.command == "worker":
        from task_queue import run_worker

        run_worker(args.concurrency)

    elif args.command == "serve":
        import uvicorn
        from api import app

        uvicorn.run(app, host=args.host, port=args.port)

    elif args.command == "rebuild-vectors":
        from feature_vector import backfill_library_feature_vectors

        wait_for_db()
        count = backfill_library_feature_vectors()
        print(f"Rebuilt feature vectors for {count} tracks")

    elif args.command == "rebuild-clusters":
        from clustering import backfill_library_clusters

        wait_for_db()
        result = backfill_library_clusters(n_clusters=args.n_clusters)
        print(f"Rebuilt clusters: {result}")


if __name__ == "__main__":
    main()
