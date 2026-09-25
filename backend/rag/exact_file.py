import re
from pathlib import PurePosixPath


LANGUAGE_MAP = {
    ".py": "python", ".pyw": "python",
    ".js": "javascript", ".mjs": "javascript", ".cjs": "javascript",
    ".jsx": "jsx",
    ".ts": "typescript", ".tsx": "tsx",
    ".java": "java",
    ".go": "go", ".mod": "go", ".sum": "go",
    ".rb": "ruby",
    ".php": "php",
    ".c": "c", ".h": "c",
    ".cpp": "cpp", ".cc": "cpp", ".cxx": "cpp", ".hpp": "cpp",
    ".cs": "csharp",
    ".rs": "rust",
    ".kt": "kotlin", ".kts": "kotlin",
    ".swift": "swift",
    ".scala": "scala",
    ".vue": "vue",
    ".svelte": "svelte",
    ".html": "html", ".htm": "html",
    ".css": "css",
    ".scss": "scss",
    ".sass": "sass",
    ".less": "less",
    ".json": "json",
    ".yaml": "yaml", ".yml": "yaml",
    ".toml": "toml",
    ".ini": "ini",
    ".cfg": "ini",
    ".conf": "conf",
    ".md": "markdown", ".mdx": "markdown",
    ".sql": "sql",
    ".sh": "bash", ".bash": "bash", ".zsh": "bash",
    ".ps1": "powershell",
    ".xml": "xml",
    ".graphql": "graphql", ".gql": "graphql",
    ".proto": "protobuf",
    ".txt": "text",
    ".env": "bash",
    ".gradle": "groovy",
    ".r": "r",
    ".lua": "lua",
    ".dart": "dart",
    ".ex": "elixir", ".exs": "elixir",
    ".elm": "elm",
    ".hs": "haskell",
    ".jl": "julia",
    ".pl": "perl", ".pm": "perl",
    ".vb": "vbnet",
    ".bat": "batch",
}

# Well-known files that have no extension.
NO_EXT_LANGUAGE = {
    "dockerfile": "dockerfile",
    "makefile": "makefile",
    "gemfile": "ruby",
    "rakefile": "ruby",
    "procfile": "",
    "license": "",
    ".gitignore": "",
    ".dockerignore": "",
    ".env": "bash",
}

_PATH_TOKEN_RE = re.compile(r"[A-Za-z0-9_][A-Za-z0-9_\-./]*\.[A-Za-z0-9]{1,12}")
_BARE_WORD_RE = re.compile(r"[A-Za-z0-9_\-./.]+")

# Phrases that signal "give me the raw/whole file", used together with a
# path-like token to positively identify an exact-file request.
_FULL_CONTENT_PHRASES = (
    "complete code", "full code", "entire code", "whole code",
    "full content", "entire content", "raw content", "source code of",
    "full file", "entire file", "whole file", "complete file",
    "content of", "contents of", "code of", "code for", "code in",
)

# Verbs that, combined with a bare path/filename and no negative-intent
# phrasing, still indicate the user wants the raw file (e.g. "show me
# backend/api/chat.py", "print frontend/js/app.js").
_ACTION_VERBS = (
    "give me", "show me", "print", "display", "open", "get me",
    "output", "paste", "cat ", "fetch", "read", "dump", "send me", "share",
)

# Phrases that indicate a semantic / explanatory question, even if a path
# happens to be mentioned - these must never be routed to exact retrieval.
_NEGATIVE_INTENT_PHRASES = (
    "how does", "how do", "how is", "how are",
    "why ", "why is", "why does",
    "where is", "where are", "where does",
    "what does", "what is the purpose", "what happens",
    "explain", "difference between", "compare", "comparison",
    "architecture", "workflow", "overview of", "summariz", "summary of",
    "relationship between", "interact with", "depend on", "dependency",
)


def _known_language(path: str) -> str:
    name = PurePosixPath(path).name
    ext = PurePosixPath(path).suffix.lower()

    if ext:
        return LANGUAGE_MAP.get(ext, "")

    return NO_EXT_LANGUAGE.get(name.lower(), "")


def _is_known_file_token(token: str) -> bool:
    ext = PurePosixPath(token).suffix.lower()

    if ext and ext in LANGUAGE_MAP:
        return True

    if "/" in token:
        return True

    return PurePosixPath(token).name.lower() in NO_EXT_LANGUAGE


def _extract_path_candidates(text: str) -> list[str]:
    candidates = []

    for match in _PATH_TOKEN_RE.finditer(text):
        token = match.group(0)
        if _is_known_file_token(token):
            candidates.append(token)

    for word in _BARE_WORD_RE.findall(text):
        base = word.rstrip("/").rsplit("/", 1)[-1]
        if "." not in base and base.lower() in NO_EXT_LANGUAGE:
            candidates.append(word)

    # De-duplicate while preserving order, prefer longer/deeper paths first.
    seen = set()
    unique = []
    for c in candidates:
        if c not in seen:
            seen.add(c)
            unique.append(c)

    unique.sort(key=lambda c: (c.count("/"), len(c)), reverse=True)
    return unique


def detect_file_query(question: str) -> str | None:
    """
    Returns the raw path/filename token the user is asking for the exact
    content of, or None if this doesn't look like an exact-file request
    (in which case the normal semantic RAG pipeline should handle it).
    """

    if not question or not question.strip():
        return None

    lower = question.lower()

    if any(phrase in lower for phrase in _NEGATIVE_INTENT_PHRASES):
        return None

    candidates = _extract_path_candidates(question)
    if not candidates:
        return None

    has_full_phrase = any(phrase in lower for phrase in _FULL_CONTENT_PHRASES)
    has_action_verb = any(verb in lower for verb in _ACTION_VERBS)

    if not (has_full_phrase or has_action_verb):
        return None

    return candidates[0]


def _normalize(path: str) -> str:
    return path.strip().replace("\\", "/").lstrip("./").strip("/")


def resolve_file(files: list[dict], candidate: str) -> dict:
    """
    Look up `candidate` (as returned by detect_file_query) against the
    repository's file list ([{"path": ..., "content": ...}, ...]).

    Returns one of:
      {"status": "found", "file": {...}}
      {"status": "ambiguous", "candidates": [{...}, ...]}
      {"status": "not_found", "query": candidate}
    """

    files = [f for f in (files or []) if isinstance(f, dict) and f.get("path")]

    cand_norm = _normalize(candidate).lower()
    is_bare_filename = "/" not in candidate.strip("/")

    exact = [f for f in files if _normalize(f["path"]).lower() == cand_norm]
    if exact:
        return {"status": "found", "file": exact[0]}

    if not is_bare_filename:
        suffix_matches = [
            f for f in files
            if _normalize(f["path"]).lower().endswith("/" + cand_norm)
        ]
        if len(suffix_matches) == 1:
            return {"status": "found", "file": suffix_matches[0]}
        if len(suffix_matches) > 1:
            return {"status": "ambiguous", "candidates": suffix_matches}
        return {"status": "not_found", "query": candidate}

    basename_matches = [
        f for f in files
        if PurePosixPath(_normalize(f["path"])).name.lower() == cand_norm
    ]
    if len(basename_matches) == 1:
        return {"status": "found", "file": basename_matches[0]}
    if len(basename_matches) > 1:
        return {"status": "ambiguous", "candidates": basename_matches}

    return {"status": "not_found", "query": candidate}


def format_found(file: dict) -> str:
    path = file["path"]
    content = file.get("content", "")
    language = _known_language(path)

    return f"**`{path}`**\n\n```{language}\n{content}\n```"


def format_ambiguous(query: str, candidates: list[dict]) -> str:
    lines = "\n".join(f"- `{f['path']}`" for f in candidates[:20])
    return (
        f"Multiple files match **{query}**. Which one did you mean?\n\n{lines}"
    )


def format_not_found(query: str) -> str:
    return (
        f"I couldn't find a file matching **{query}** in this repository. "
        "Please check the path and try again."
    )


def exact_file_answer(question: str, files: list[dict]) -> str | None:
    """
    Full deterministic pipeline: detect -> resolve -> format.

    Returns None when `question` is not an exact-file request at all (the
    caller should fall back to the normal semantic RAG pipeline). Returns a
    ready-to-send markdown string in every other case (found, ambiguous, or
    not found) - none of those should ever go through embeddings/RAG/LLM.
    """

    candidate = detect_file_query(question)
    if candidate is None:
        return None

    result = resolve_file(files, candidate)

    if result["status"] == "found":
        return format_found(result["file"])
    if result["status"] == "ambiguous":
        return format_ambiguous(candidate, result["candidates"])
    return format_not_found(candidate)
