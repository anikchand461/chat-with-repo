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
