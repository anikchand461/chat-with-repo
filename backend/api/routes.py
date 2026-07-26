import os
from typing import Optional

from fastapi import APIRouter

from github import (
    get_repo,
    get_branches,
    get_tree,
    get_all_files,
)

from backend.storage.json_writer import save_json

router = APIRouter()

# -------------------------------
# Development limits
# -------------------------------
MAX_REPO_SIZE_MB = 500      # Increase from 20 MB
MAX_FILE_COUNT = 5000       # Increase from 300 files


@router.get("/repo/{owner}/{repo}")
def repository(
    owner: str,
    repo: str,
    github_token: Optional[str] = None,
):
    return get_repo(
        owner,
        repo,
        github_token=github_token,
    )


@router.get("/repo/{owner}/{repo}/branches")
def branches(
    owner: str,
    repo: str,
    github_token: Optional[str] = None,
):
    return get_branches(
        owner,
        repo,
        github_token=github_token,
    )


@router.get("/analyze/{owner}/{repo}")
def analyze_default(
    owner: str,
    repo: str,
    github_token: Optional[str] = None,
):
    repo_info = get_repo(
        owner,
        repo,
        github_token=github_token,
    )

    branch = repo_info["default_branch"]

    result = {
        "repository": repo_info,
        "branch": branch,
        "tree": get_tree(
            owner,
            repo,
            branch,
            github_token=github_token,
        ),
        "files": get_all_files(
            owner,
            repo,
            branch,
            github_token=github_token,
        ),
    }

    filename = os.path.join(
        "data",
        f"{owner}_{repo}.json",
    )

    save_json(result, filename)

    return result


@router.get("/analyze/{owner}/{repo}/{branch}")
def analyze_branch(
    owner: str,
    repo: str,
    branch: str,
    github_token: Optional[str] = None,
):
    repo_info = get_repo(
        owner,
        repo,
        github_token=github_token,
    )

    # Private repository check
    if repo_info.get("private"):
        raise Exception(
            "Private repositories require a GitHub token or Pro plan."
        )

    # -------------------------------
    # Repository size check
    # -------------------------------
    repo_size_mb = repo_info.get("size", 0) / 1024

    if repo_size_mb > MAX_REPO_SIZE_MB:
        raise Exception(
            f"Repository size is {repo_size_mb:.2f} MB. "
            f"Maximum allowed is {MAX_REPO_SIZE_MB} MB."
        )

    result = {
        "repository": repo_info,
        "branch": branch,
        "tree": get_tree(
            owner,
            repo,
            branch,
            github_token=github_token,
        ),
        "files": get_all_files(
            owner,
            repo,
            branch,
            github_token=github_token,
        ),
    }

    file_count = len(result["files"])

    # -------------------------------
    # File count check
    # -------------------------------
    if file_count > MAX_FILE_COUNT:
        raise Exception(
            f"Repository contains {file_count} files. "
            f"Maximum allowed is {MAX_FILE_COUNT} files."
        )

    filename = os.path.join(
        "data",
        f"{owner}_{repo}_{branch}.json",
    )

    save_json(result, filename)

    return result