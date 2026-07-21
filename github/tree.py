from urllib.parse import quote

from .client import github_get
from .repository import get_default_branch


def get_tree(owner, repo, branch=None):
    if branch is None:
        branch = get_default_branch(owner, repo)

    branch = quote(branch, safe="")

    return github_get(
        f"/repos/{owner}/{repo}/git/trees/{branch}",
        {
            "recursive": 1
        }
    )["tree"]