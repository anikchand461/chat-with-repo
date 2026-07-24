import requests

from config import BASE_URL, HEADERS

def github_get(endpoint, params=None):

    response = requests.get(
        BASE_URL + endpoint,
        headers=HEADERS,
        params=params,
    )

    print("Status:", response.status_code)
    print("Remaining:", response.headers.get("X-RateLimit-Remaining"))

    response.raise_for_status()

    return response.json()