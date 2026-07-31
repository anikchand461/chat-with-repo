import sys
from pathlib import Path

BACKEND_ROOT = Path(__file__).resolve().parent
if str(BACKEND_ROOT) not in sys.path:
    sys.path.insert(0, str(BACKEND_ROOT))

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from backend.api.routes import router
from backend.api.auth import router as auth_router
from backend.api.chat import router as chat_router
from backend.api.payment import router as payment_router

app = FastAPI(
    title="DevLens API",
    version="0.1.0",
    description="GitHub Repo RAG Chat",
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=[
        "http://localhost:5500",
        "http://127.0.0.1:5500",
        "http://localhost:5501",
        "http://127.0.0.1:5501",
        "http://localhost:8000",
        "http://127.0.0.1:8000",
        "https://chatwithrepo-nine.vercel.app",  # current live frontend
        "https://chatwithrepo.vercel.app",
    ],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(router)
app.include_router(auth_router)
app.include_router(chat_router)
app.include_router(payment_router)


@app.get("/")
def home():
    return {
        "message": "✅ DevLens API is running!",
        "docs": "/docs",
    }


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="127.0.0.1", port=8000, reload=True)