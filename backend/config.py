import os
from dotenv import load_dotenv

load_dotenv()

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