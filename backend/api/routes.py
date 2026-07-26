import os

from fastapi import APIRouter

from github import (
    get_repo,
    get_branches,
    get_tree,
    get_all_files,
)

from backend.storage.json_writer import save_json

router = APIRouter()


@router.get("/repo/{owner}/{repo}")
def repository(owner: str, repo: str):
    return get_repo(owner, repo)


@router.get("/repo/{owner}/{repo}/branches")
def branches(owner: str, repo: str):
    return get_branches(owner, repo)


@router.get("/analyze/{owner}/{repo}")
def analyze_default(
    owner: str,
    repo: str,
):
    repo_info = get_repo(owner, repo)

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
    github_token=None,
):
    repo_info = get_repo(
        owner,
        repo,
        github_token=github_token,
    )

    # Public repository check
    if repo_info.get("private"):
        raise Exception("Private repositories require a Pro plan.")
    
    # Repository size check (GitHub API reports size in KB)
    repo_size_mb = repo_info.get("size", 0) / 1024
    
    if repo_size_mb > 20:
        raise Exception(
            f"Repository size is {repo_size_mb:.2f} MB. "
            "Free plan supports repositories up to 20 MB."
        )

    result = {
        "repository": repo_info,
        "branch": branch,
        "tree": get_tree(owner, repo, branch),
        "files": get_all_files(owner, repo, branch),
    }

    filename = os.path.join(
        "data",
        f"{owner}_{repo}_{branch}.json",
    )

    file_count = len(result["files"])
    
    if file_count > 300:
        raise Exception(
            f"Repository contains {file_count} files. "
            "Free plan supports up to 300 files."
        )

    save_json(result, filename)

    return result