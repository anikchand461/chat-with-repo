import os
from dotenv import load_dotenv

load_dotenv()

# GitHub
GITHUB_TOKEN = os.getenv("GITHUB_TOKEN")

# Cohere
COHERE_API_KEY = os.getenv("COHERE_API_KEY")

# Gemini (if using)
GEMINI_API_KEY = os.getenv("GEMINI_API_KEY")

BASE_URL = "https://api.github.com"

HEADERS = {
    "Accept": "application/vnd.github+json",
    "X-GitHub-Api-Version": "2022-11-28",
}

if GITHUB_TOKEN:
    HEADERS["Authorization"] = f"Bearer {GITHUB_TOKEN}"

# print("GitHub Token:", GITHUB_TOKEN)