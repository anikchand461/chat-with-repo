import json
from pathlib import PurePosixPath

# Dependency lockfiles and generated artifacts: large, machine-written and
# useless for answering questions about a codebase. Language-agnostic.
NOISE_FILENAMES = {
    "package-lock.json",
    "npm-shrinkwrap.json",
    "yarn.lock",
    "pnpm-lock.yaml",
    "bun.lockb",
    "poetry.lock",
    "pipfile.lock",
    "uv.lock",
    "composer.lock",
    "gemfile.lock",
    "cargo.lock",
    "go.sum",
    "pubspec.lock",
    "packages.lock.json",
    "podfile.lock",
    "gradle.lockfile",
}

NOISE_SUFFIXES = (".lock", ".min.js", ".min.css", ".map", ".snap")


def is_noise_path(path: str) -> bool:
    name = PurePosixPath(path).name.lower()
    return name in NOISE_FILENAMES or name.endswith(NOISE_SUFFIXES)


def is_repo_dump_json(content: str) -> bool:
    """
    True when `content` is one of this app's own repository-fetch JSON
    caches (backend/storage/json_writer.py's format: repository metadata +
    git tree + file contents). If a repo being chatted about happens to
    have one of these committed (e.g. leftover local test data, as
    backend/data/*.json was in this project's own repo), it must not be
    indexed - it describes a *different*, unrelated repository, and its
    content (READMEs, code, etc. from that other project) would otherwise
    pollute semantic retrieval for the repo actually being asked about.
    """

    stripped = content.lstrip()
    if not stripped.startswith("{"):
        return False

    try:
        data = json.loads(content)
    except (ValueError, TypeError):
        return False

    return (
        isinstance(data, dict)
        and isinstance(data.get("repository"), dict)
        and isinstance(data.get("files"), list)
        and "tree" in data
    )
