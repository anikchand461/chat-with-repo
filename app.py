from fastapi import FastAPI

from api.routes import router

app = FastAPI(
    title="DevLens API",
    version="0.1.0"
)

app.include_router(router)


@app.get("/")
def home():
    return {
        "message": "DevLens API"
    }