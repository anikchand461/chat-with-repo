import requests
from copy import deepcopy
from backend.config import BASE_URL, HEADERS, GITHUB_TOKEN

def github_get(endpoint, params=None, github_token=None):
    headers = deepcopy(HEADERS)

    github_token = github_token or GITHUB_TOKEN

    if github_token:
        headers["Authorization"] = f"Bearer {github_token}"
        print("AUTH: using token")
    else:
        print("AUTH: NO TOKEN")

    response = requests.get(
        BASE_URL + endpoint,
        headers=headers,
        params=params,
    )

    print("URL:", BASE_URL + endpoint)
    print("Status:", response.status_code)
    print("Remaining:", response.headers.get("X-RateLimit-Remaining"))

    if not response.ok:
        print("Response:", response.text)

    if (
        response.status_code in (403, 429)
        and response.headers.get("X-RateLimit-Remaining") == "0"
    ):
        raise Exception(
            "GitHub API rate limit reached. Add a GitHub token in your profile "
            "(or set GITHUB_TOKEN on the server) and try again later."
        )

    response.raise_for_status()
    return response.json()