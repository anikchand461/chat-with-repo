import requests
from copy import deepcopy
from backend.config import BASE_URL, HEADERS

def github_get(endpoint, params=None, github_token=None):
    headers = deepcopy(HEADERS)

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

    response.raise_for_status()
    return response.json()