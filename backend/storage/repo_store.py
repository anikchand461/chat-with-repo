"""
Neon (Postgres) copy of the downloaded repository JSON files (DATA_DIR).

The repo JSON is needed for every question (file tree + README overview),
but Render's disk is wiped on every restart, so the first question after
a restart used to download the whole repository from GitHub again - one
request per file. Now each JSON file is also saved to the repository_json
table, and copied back to disk only when it's missing locally. While the
server is up, the file on disk is the cache; the database is read once.

Cache problems are logged and never fail a request - the callers fall back
to fetching from GitHub as before.
"""

import json
import os
from datetime import datetime, timezone
from pathlib import Path

# Neon's free plan has 0.5 GB in total; don't let one huge repository fill it.
MAX_BYTES = int(os.getenv("REPO_JSON_DB_MAX_MB", "25")) * 1024 * 1024


def save(path, data):
    path = Path(path)

    try:
        from backend.database.db import RepositoryJson, SessionLocal

        body = json.dumps(data, ensure_ascii=False)

        # Postgres JSONB can't hold NUL characters (possible in odd files).
        if "\\u0000" in body:
            data = _without_nul(data)
            body = json.dumps(data, ensure_ascii=False)

        if len(body.encode("utf-8")) > MAX_BYTES:
            print(f"Repo store: {path.name} is too large to save in the database")
            return

        with SessionLocal() as session:
            # merge() = insert or update by primary key (the file name).
            session.merge(
                RepositoryJson(
                    name=path.name,
                    data=data,
                    updated_at=datetime.now(timezone.utc).replace(tzinfo=None),
                )
            )
            session.commit()
    except Exception as e:
        # First line only: the full error repeats the whole repository.
        reason = str(getattr(e, "orig", None) or e).splitlines()[0][:200]
        print(f"Repo store: could not save {path.name}: {type(e).__name__}: {reason}")


def restore(path):
    """
    Write `path` from the database if it doesn't exist locally. Returns True
    only when a file was restored.
    """

    path = Path(path)

    if path.exists():
        return False

    partial = path.with_name(path.name + ".partial")

    try:
        from backend.database.db import RepositoryJson, SessionLocal

        with SessionLocal() as session:
            row = session.get(RepositoryJson, path.name)
            data = row.data if row else None

        if data is None:
            return False

        path.parent.mkdir(parents=True, exist_ok=True)
        with open(partial, "w", encoding="utf-8", newline="\n") as f:
            json.dump(data, f, indent=4, ensure_ascii=False)
            f.write("\n")
        os.replace(partial, path)

        print(f"Repo store: restored {path.name} from the database")
        return True
    except Exception as e:
        partial.unlink(missing_ok=True)
        print(f"Repo store: could not restore {path.name}: {type(e).__name__}: {e}")
        return False


def _without_nul(value):
    if isinstance(value, str):
        return value.replace("\x00", "")
    if isinstance(value, list):
        return [_without_nul(v) for v in value]
    if isinstance(value, dict):
        return {_without_nul(k): _without_nul(v) for k, v in value.items()}
    return value
