# github/config.py
import os
from dotenv import load_dotenv
from copy import deepcopy

load_dotenv()

GITHUB_TOKEN = os.getenv("GITHUB_TOKEN")
BASE_URL = "https://api.github.com"

HEADERS = {
    "Accept": "application/vnd.github+json",
    "X-GitHub-Api-Version": "2022-11-28",
}

__all__ = ["GITHUB_TOKEN", "BASE_URL", "HEADERS"]