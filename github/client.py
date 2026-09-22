from datetime import datetime, timezone

import requests
from copy import deepcopy
from backend.config import BASE_URL, HEADERS, GITHUB_TOKEN

def _rate_limit_reset_text(response):
    """
    GitHub sends the exact reset time as a Unix timestamp in
    X-RateLimit-Reset - unauthenticated limits are a rolling 60/hour
    window, not a fixed clock hour, so this is the only reliable way to
    know when it actually opens back up. Falls back to generic wording if
    the header is missing for some reason.

    Only the relative delta is shown ("in about N min"), never a guessed
    clock time: this runs on the server, which has no idea what timezone
    the person reading the message is in, and datetime.astimezone() with
    no argument would silently use the *server's* local zone (typically
    UTC on a host) mislabeled as if it were the reader's own.
    """

    reset_header = response.headers.get("X-RateLimit-Reset")

    if not reset_header:
        return "(usually within an hour)"

    try:
        reset_at = datetime.fromtimestamp(int(reset_header), tz=timezone.utc)
    except (TypeError, ValueError):
        return "(usually within an hour)"

    minutes = max(0, round((reset_at - datetime.now(timezone.utc)).total_seconds() / 60))

    if minutes <= 0:
        return "(should be available now - try again)"

    return f"(in about {minutes} min)"


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
            "(or set GITHUB_TOKEN on the server) and try again later "
            f"{_rate_limit_reset_text(response)}."
        )

    response.raise_for_status()
    return response.json()