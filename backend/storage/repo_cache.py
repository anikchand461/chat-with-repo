"""
S3 copy of the downloaded repository JSON files (DATA_DIR).

The repo JSON is needed for every question (file tree + README overview),
but on Lambda DATA_DIR is /tmp: per-container and temporary. Without a
shared copy, every fresh container would re-download the whole repository
from GitHub - one request per file. So each JSON file is uploaded after it
is written and downloaded again only when it's missing locally.

Separate from Qdrant (vectors). Disabled when REPO_CACHE_BUCKET is unset
(local development, other hosts): every function is then a no-op. Cache
problems are logged and never fail a request - the callers fall back to
fetching from GitHub as before.
"""

import os
from pathlib import Path

from backend.config import DATA_DIR

BUCKET = os.getenv("REPO_CACHE_BUCKET")
PREFIX = "repos/"

_client = None


def _s3():
    global _client
    if _client is None:
        import boto3

        _client = boto3.client("s3")
    return _client


def _key(path):
    path = Path(path).resolve()
    try:
        relative = path.relative_to(Path(DATA_DIR).resolve())
    except ValueError:
        relative = Path(path.name)
    return PREFIX + relative.as_posix()


def upload(path):
    if not BUCKET:
        return

    try:
        _s3().upload_file(str(path), BUCKET, _key(path))
    except Exception as e:
        print(f"Repo cache: upload failed for {Path(path).name}: {type(e).__name__}: {e}")


def download(path):
    """
    Fetch `path` from S3 if it doesn't exist locally. Returns True only when
    a file was downloaded.
    """

    path = Path(path)

    if not BUCKET or path.exists():
        return False

    partial = path.with_name(path.name + ".partial")

    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        _s3().download_file(BUCKET, _key(path), str(partial))
        os.replace(partial, path)
        return True
    except Exception as e:
        partial.unlink(missing_ok=True)
        code = getattr(e, "response", {}).get("Error", {}).get("Code")
        if code not in ("404", "NoSuchKey"):
            print(f"Repo cache: download failed for {path.name}: {type(e).__name__}: {e}")
        return False
