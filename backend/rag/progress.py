"""
In-memory indexing-progress tracker, keyed by chat id.

Used so the frontend can poll a short, predefined status message while a
new chat's repository is being fetched and indexed in the background,
instead of the request just hanging until the whole thing is done.

This is process-local (a plain dict), matching how `rag.pipeline`'s
`_pipelines` cache already works - fine for a single backend process.
"""

import threading

# Short, human-readable messages for each stage. Deliberately coarse -
# just enough to reassure the user something is happening, not a log feed.
STAGES = {
    "starting": "Setting up your chat…",
    "fetching": "Downloading the repository from GitHub…",
    "indexing": "Indexing the repository for search…",
    "ready": "Ready!",
}

_lock = threading.Lock()
_progress = {}


def set_stage(chat_id, stage, detail=None, done=None, total=None):
    """
    `done`/`total`, if given (only meaningful during the "indexing" stage),
    are carried as plain numbers alongside the message - so the frontend
    can render a real percentage-filled progress bar instead of having to
    parse them back out of "(done/total chunks)" text.
    """

    with _lock:
        entry = {
            "status": "working",
            "stage": stage,
            "message": detail or STAGES.get(stage, stage),
        }

        if done is not None and total:
            entry["done"] = done
            entry["total"] = total

        _progress[chat_id] = entry


def set_ready(chat_id):
    with _lock:
        _progress[chat_id] = {
            "status": "ready",
            "stage": "ready",
            "message": STAGES["ready"],
        }


def set_error(chat_id, message):
    with _lock:
        _progress[chat_id] = {
            "status": "error",
            "stage": "error",
            "message": message,
        }


def get(chat_id):
    with _lock:
        entry = _progress.get(chat_id)

    # No entry means nothing is tracking this chat right now - either it
    # was indexed before this feature existed, or it's from a previous
    # server run. Callers that need to tell "genuinely ready" apart from
    # "never checked" should use is_tracked() first (see chat.py's
    # index-status endpoint, which does a real check in that case).
    return entry or {"status": "ready", "stage": "ready", "message": STAGES["ready"]}


def is_tracked(chat_id):
    with _lock:
        return chat_id in _progress


def claim(chat_id):
    """
    Atomically claim a chat for indexing: if no job is *actively running*
    for it, marks it "starting" and returns True - the caller now owns
    scheduling the one background indexing job. Returns False only when
    another job already has it claimed and is still working; a chat left in
    "error" (a finished, failed attempt - e.g. the user hadn't configured a
    GitHub token yet) or "ready" can always be re-claimed, so returning to
    a failed chat and retrying (after adding a token) actually retries
    instead of just replaying the same stale error forever.

    This exists because "is_tracked() then set_stage()" is two separate
    calls - under concurrent requests for the same chat (FastAPI runs sync
    endpoints in a thread pool, so this genuinely happens: e.g. the create
    modal's own automatic retry racing a still-in-flight first attempt),
    both could see "not claimed" and each schedule their own indexing job,
    running two full rebuilds against the same Qdrant collection at once.
    """

    with _lock:
        existing = _progress.get(chat_id)
        if existing and existing["status"] == "working":
            return False

        _progress[chat_id] = {
            "status": "working",
            "stage": "starting",
            "message": STAGES["starting"],
        }
        return True


def clear(chat_id):
    with _lock:
        _progress.pop(chat_id, None)
