import requests

from config import BASE_URL, HEADERS
from copy import deepcopy

def github_get(endpoint, params=None, github_token=None):

    headers = deepcopy(HEADERS)

    if github_token:
        headers["Authorization"] = f"Bearer {github_token}"

    response = requests.get(
        BASE_URL + endpoint,
        headers=headers,
        params=params,
    )

    print("Status:", response.status_code)
    print("Remaining:", response.headers.get("X-RateLimit-Remaining"))

    response.raise_for_status()

    return response.json()