from .client import github_get


def get_repo(owner, repo, github_token=None):
    return github_get(
        f"/repos/{owner}/{repo}",
        github_token=github_token,
    )


def get_branches(owner, repo):
    return github_get(
        f"/repos/{owner}/{repo}/branches"
    )


def get_default_branch(owner, repo):
    repo_data = get_repo(owner, repo)

    return repo_data["default_branch"]