import requests

from config import BASE_URL, HEADERS


def github_get(endpoint, params=None):
    response = requests.get(
        BASE_URL + endpoint,
        headers=HEADERS,
        params=params,
    )

    response.raise_for_status()

    return response.json()