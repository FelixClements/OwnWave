from celery import Celery
from celery.signals import worker_process_init

from config import CELERY_BROKER_URL, CELERY_RESULT_BACKEND


celery_app = Celery(
    "ownwave_analytics",
    broker=CELERY_BROKER_URL,
    backend=CELERY_RESULT_BACKEND,
    include=["tasks"],
)

celery_app.conf.update(
    task_serializer="json",
    accept_content=["json"],
    result_serializer="json",
    timezone="UTC",
    enable_utc=True,
    task_acks_late=True,
    worker_prefetch_multiplier=1,
    task_track_started=True,
    # Bound how long publishing waits on an unreachable Redis (defaults: 4s
    # connect timeout per attempt, 3 retries) so the API answers 503 well
    # within the Go client's 30s timeout.
    broker_connection_timeout=2,
    task_publish_retry_policy={"max_retries": 2, "interval_start": 0, "interval_step": 0.5, "interval_max": 1},
)


@worker_process_init.connect
def init_worker(**kwargs):
    import db

    db.wait_for_db()
