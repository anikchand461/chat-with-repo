"""
Fargate task entry point for one indexing job (see backend/services/index_jobs.py):

    python -m backend.jobs.index_chat '<job json>'

Runs the same _index_new_chat as the in-process BackgroundTask, which
records progress, the result and any error itself. A heartbeat keeps the
progress row fresh while a long step (e.g. a large GitHub download) runs,
and stops the task if it exceeds INDEX_MAX_SECONDS so a stuck job can't
run (and bill) forever.
"""

import json
import os
import sys
import threading
from pathlib import Path

# Same as backend/app.py: the rag/ modules import each other as `rag.x`,
# which needs backend/ on sys.path. The web server gets this from app.py;
# this job never imports app.py.
BACKEND_ROOT = Path(__file__).resolve().parent.parent
if str(BACKEND_ROOT) not in sys.path:
    sys.path.insert(0, str(BACKEND_ROOT))

MAX_SECONDS = int(os.getenv("INDEX_MAX_SECONDS", "7200"))


def main(argv):
    if len(argv) != 2:
        print("usage: python -m backend.jobs.index_chat '<job json>'", file=sys.stderr)
        return 2

    job = json.loads(argv[1])
    chat_id = job["chat_id"]

    try:
        from backend.api.chat import _index_new_chat
        from backend.database import SessionLocal, User
        from backend.rag import progress
        from backend.rag.progress_db import HEARTBEAT_SECONDS

        with SessionLocal() as session:
            user = session.get(User, job["user_id"])
            github_token = user.github_token if user else None
    except Exception:
        # Fail the chat now instead of leaving it on "working" until the
        # stale check (progress_db.STALE_AFTER) gives up on it.
        _report_failure(chat_id)
        raise

    finished = threading.Event()

    def heartbeat():
        waited = 0
        while not finished.wait(HEARTBEAT_SECONDS):
            waited += HEARTBEAT_SECONDS
            if waited >= MAX_SECONDS:
                progress.set_error(chat_id, "Indexing took too long and was stopped.")
                print(f"Chat {chat_id}: stopped after {MAX_SECONDS}s", flush=True)
                os._exit(1)
            progress.touch(chat_id)

    threading.Thread(target=heartbeat, daemon=True).start()

    print(f"Indexing chat {chat_id}: {job['owner']}/{job['repo']}@{job['branch']}", flush=True)

    try:
        _index_new_chat(
            chat_id,
            job["owner"],
            job["repo"],
            job["branch"],
            github_token,
            job["collection_name"],
            job["user_id"],
            job["is_free"],
        )
    finally:
        finished.set()

    status = progress.get(chat_id)["status"]
    print(f"Chat {chat_id}: {status}", flush=True)
    return 0 if status == "ready" else 1


def _report_failure(chat_id):
    try:
        from backend.rag import progress

        progress.set_error(chat_id, "Indexing failed to start. Please try again.")
    except Exception as e:
        print(f"Chat {chat_id}: could not record the failure: {type(e).__name__}: {e}", file=sys.stderr)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
