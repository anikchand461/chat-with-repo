import os
from pathlib import Path
from dotenv import load_dotenv

PROJECT_ROOT = Path(__file__).resolve().parent.parent

# Load the project's .env regardless of the directory the server was started from.
load_dotenv(PROJECT_ROOT / ".env")

# Where repository JSON and vector indexes are stored. Absolute paths anchored
# to the project root so they don't depend on the directory the server was
# started from. Override with DATA_DIR / CHROMA_DIR (e.g. a persistent disk).
DATA_DIR = Path(os.getenv("DATA_DIR") or PROJECT_ROOT / "data")
CHROMA_DIR = Path(os.getenv("CHROMA_DIR") or PROJECT_ROOT / "chroma_db")

# Server-wide GitHub token, used when a user has not saved their own.
# Raises the GitHub API limit from 60 to 5000 requests/hour. Private
# repositories are still rejected for everyone (see analyze_branch).
GITHUB_TOKEN = os.getenv("GITHUB_TOKEN")

# Cohere
COHERE_API_KEY = os.getenv("COHERE_API_KEY")

# Gemini
GEMINI_API_KEY = os.getenv("GEMINI_API_KEY")

BASE_URL = "https://api.github.com"

# Default GitHub headers
# Authorization will be added dynamically if the user has saved a token.
HEADERS = {
    "Accept": "application/vnd.github+json",
    "X-GitHub-Api-Version": "2022-11-28",
}