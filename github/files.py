import base64
from urllib.parse import quote

from .client import github_get
from .tree import get_tree


SOURCE_EXTENSIONS = (
    ".py",
    ".js",
    ".ts",
    ".tsx",
    ".java",
    ".cpp",
    ".c",
    ".go",
    ".rs",
    ".php",
    ".html",
    ".css",
    ".json",
    ".md",
    ".yml",
    ".yaml",
)


def get_file(owner, repo, path, branch=None):
    path = quote(path, safe="/")

    params = {}

    if branch:
        params["ref"] = branch

    data = github_get(
        f"/repos/{owner}/{repo}/contents/{path}",
        params,
    )

    if data.get("type") != "file":
        return None

    content = base64.b64decode(
        data["content"]
    ).decode(
        "utf-8",
        errors="ignore",
    )

    return {
        "path": path,
        "content": content,
    }


def get_all_files(owner, repo, branch=None):
    tree = get_tree(owner, repo, branch)

    files = []

    for item in tree:
        if item["type"] != "blob":
            continue

        path = item["path"]

        if path.endswith(SOURCE_EXTENSIONS):
            file = get_file(
                owner,
                repo,
                path,
                branch,
            )

            if file:
                files.append(file)

    return files