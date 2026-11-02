"""
Postgres-backed version of rag.progress, used with PROGRESS_STORE=db.

On AWS the API runs on many Lambda instances and indexing runs in a
separate Fargate task (backend/jobs/index_chat.py), so progress has to live
somewhere they all share. Same functions and return values as the
in-memory store; rag.progress forwards to these.

A running job refreshes `updated_at` at least every HEARTBEAT_SECONDS (each
stage update, plus touch() from the job's heartbeat). A "working" row older
than STALE_AFTER means the job died without recording it (task stopped,
out of memory, never started): get() reports it as an error and claim()
can take it over, so the user can retry instead of waiting forever.
"""

from datetime import datetime, timedelta, timezone

from sqlalchemy import delete, or_, update
from sqlalchemy.exc import IntegrityError

from backend.database.db import IndexProgress, SessionLocal
from backend.rag.progress import STAGES

HEARTBEAT_SECONDS = 60

# Generous: also covers a Fargate task waiting to start (image pull).
STALE_AFTER = timedelta(minutes=10)

STALE_MESSAGE = "Indexing stopped unexpectedly. Please try again."


def _now():
    return datetime.now(timezone.utc).replace(tzinfo=None)


def _save(chat_id, status, stage, message, done=None, total=None):
    with SessionLocal() as session:
        row = session.get(IndexProgress, chat_id)
        if row is None:
            row = IndexProgress(chat_id=chat_id)
            session.add(row)

        row.status = status
        row.stage = stage
        row.message = message
        row.done = done
        row.total = total
        row.updated_at = _now()
        session.commit()


def set_stage(chat_id, stage, detail=None, done=None, total=None):
    if done is None or not total:
        done = total = None

    _save(chat_id, "working", stage, detail or STAGES.get(stage, stage), done, total)


def set_ready(chat_id):
    _save(chat_id, "ready", "ready", STAGES["ready"])


def set_error(chat_id, message):
    _save(chat_id, "error", "error", message)


def get(chat_id):
    with SessionLocal() as session:
        row = session.get(IndexProgress, chat_id)

    if row is None:
        return {"status": "ready", "stage": "ready", "message": STAGES["ready"]}

    if row.status == "working" and row.updated_at < _now() - STALE_AFTER:
        return {"status": "error", "stage": "error", "message": STALE_MESSAGE}

    entry = {"status": row.status, "stage": row.stage, "message": row.message}

    if row.done is not None and row.total:
        entry["done"] = row.done
        entry["total"] = row.total

    return entry


def is_tracked(chat_id):
    with SessionLocal() as session:
        return session.get(IndexProgress, chat_id) is not None


def claim(chat_id):
    """
    Atomic across instances: the conditional UPDATE only matches a row that
    isn't actively working (Postgres re-checks the condition after waiting
    on a concurrent update), and the INSERT for a new chat fails on the
    primary key if another instance inserted first.
    """

    starting = {
        "status": "working",
        "stage": "starting",
        "message": STAGES["starting"],
        "done": None,
        "total": None,
    }

    with SessionLocal() as session:
        result = session.execute(
            update(IndexProgress)
            .where(
                IndexProgress.chat_id == chat_id,
                or_(
                    IndexProgress.status != "working",
                    IndexProgress.updated_at < _now() - STALE_AFTER,
                ),
            )
            .values(**starting, updated_at=_now())
        )
        session.commit()

        if result.rowcount:
            return True

        try:
            session.add(IndexProgress(chat_id=chat_id, **starting, updated_at=_now()))
            session.commit()
            return True
        except IntegrityError:
            session.rollback()
            return False


def touch(chat_id):
    with SessionLocal() as session:
        session.execute(
            update(IndexProgress)
            .where(IndexProgress.chat_id == chat_id, IndexProgress.status == "working")
            .values(updated_at=_now())
        )
        session.commit()


def clear(chat_id):
    with SessionLocal() as session:
        session.execute(delete(IndexProgress).where(IndexProgress.chat_id == chat_id))
        session.commit()
