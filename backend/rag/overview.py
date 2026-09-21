from collections import Counter
from pathlib import PurePosixPath
from typing import Any


class RepositoryOverview:
    """
    Builds a compact, stack-agnostic overview of a repository that is sent
    to the LLM with every question, regardless of what retrieval returned.

    It only uses what the repository JSON already contains: metadata,
    the file tree, file extensions and the README.
    """

    MAX_TREE_PATHS = 150
    MAX_README_CHARS = 3500
    MAX_TOPICS = 12
    MAX_EXTENSIONS = 6

    def __init__(self, data: Any):
        self.data = data if isinstance(data, dict) else {"files": data}

        repo = self.data.get("repository")
        self.repo = repo if isinstance(repo, dict) else {}
        self.files = [f for f in (self.data.get("files") or []) if isinstance(f, dict)]

    # ------------------------------------------------------------------

    @property
    def name(self) -> str | None:
        return (
            self.repo.get("full_name")
            or self.repo.get("name")
            or (self.data.get("repository") if isinstance(self.data.get("repository"), str) else None)
            or self.data.get("repository_name")
        )

    @property
    def branch(self) -> str | None:
        return self.data.get("branch") or self.repo.get("default_branch")

    # ------------------------------------------------------------------

    def _file_paths(self) -> list[str]:
        """
        Every file path in the repo. Prefer the full git tree (it includes
        files whose content was not indexed); fall back to indexed files.
        """

        tree = self.data.get("tree")

        if isinstance(tree, list):
            paths = [
                t.get("path")
                for t in tree
                if isinstance(t, dict) and t.get("type") == "blob" and t.get("path")
            ]
            if paths:
                return sorted(paths)

        return sorted(f["path"] for f in self.files if f.get("path"))

    def _tree_section(self, paths: list[str]) -> str:
        if not paths:
            return "(file list unavailable)"

        shown = paths[: self.MAX_TREE_PATHS]
        lines = "\n".join(f"- {p}" for p in shown)

        if len(paths) > len(shown):
            lines += f"\n- ... and {len(paths) - len(shown)} more files"

        return lines

    def _extension_section(self, paths: list[str]) -> str:
        counts = Counter(
            PurePosixPath(p).suffix.lower() or PurePosixPath(p).name for p in paths
        )
        return ", ".join(
            f"{ext} ({n})" for ext, n in counts.most_common(self.MAX_EXTENSIONS)
        )

    def _readme(self) -> str:
        # Prefer the shallowest README (the repo's own, not a nested one).
        readmes = [
            f
            for f in self.files
            if PurePosixPath(f.get("path", "")).name.lower().startswith("readme")
            and f.get("content", "").strip()
        ]

        if not readmes:
            return ""

        readmes.sort(key=lambda f: f["path"].count("/"))
        content = readmes[0]["content"].strip()

        if len(content) > self.MAX_README_CHARS:
            content = content[: self.MAX_README_CHARS].rstrip() + "\n... (README truncated)"

        return content

    # ------------------------------------------------------------------

    def build(self) -> str:
        paths = self._file_paths()
        indexed = {f.get("path") for f in self.files}

        parts = []

        header = [f"Repository: {self.name or 'unknown'}"]
        if self.branch:
            header.append(f"Branch: {self.branch}")
        if self.repo.get("description"):
            header.append(f"Description: {self.repo['description']}")

        topics = self.repo.get("topics") or []
        if topics:
            header.append("Topics: " + ", ".join(topics[: self.MAX_TOPICS]))

        extensions = self._extension_section(paths)
        if extensions:
            header.append(f"Most common file types: {extensions}")

        header.append(f"Total files: {len(paths)}")
        parts.append("\n".join(header))

        parts.append(
            "File tree (all files in the repository; contents of some may not be "
            "shown below):\n" + self._tree_section(paths)
        )

        readme = self._readme()
        parts.append(
            "README:\n" + readme if readme else "README: this repository has no README."
        )

        if paths and indexed:
            not_indexed = len([p for p in paths if p not in indexed])
            if not_indexed:
                parts.append(
                    f"Note: {not_indexed} file(s) in the tree were not indexed "
                    "(binary, generated or too large), so their contents are unavailable."
                )

        return "\n\n".join(parts)
