import requests
from copy import deepcopy

from backend.config import BASE_URL, HEADERS


def github_get(endpoint, params=None, github_token=None):
    headers = deepcopy(HEADERS)

    if github_token:
        headers["Authorization"] = f"Bearer {github_token}"

    response = requests.get(
        BASE_URL + endpoint,
        headers=headers,
        params=params,
    )

    print("=" * 60)
    print("URL:", response.url)
    print("Status:", response.status_code)
    print("Remaining:", response.headers.get("X-RateLimit-Remaining"))

    if response.status_code != 200:
        print("Response:")
        print(response.text)

    response.raise_for_status()

    return response.json()