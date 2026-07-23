from pydantic import BaseModel, Field
from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session

from auth import create_access_token, get_current_user, hash_password
from database import User
from database.session import get_db

router = APIRouter(prefix="/auth", tags=["auth"])


class Credentials(BaseModel):
    email: str = Field(min_length=3, max_length=255)
    password: str = Field(min_length=8, max_length=128)
    
class GitHubToken(BaseModel):
    github_access_token: str = Field(min_length=1, max_length=500)


@router.post("/register")
def register(credentials: Credentials, db: Session = Depends(get_db)):
    if db.query(User).filter(User.email == credentials.email).first():
        raise HTTPException(status_code=409, detail="Email already registered")
    user = User(email=credentials.email, hashed_password=hash_password(credentials.password))
    db.add(user)
    db.commit()
    db.refresh(user)
    return {"access_token": create_access_token(user.id), "token_type": "bearer"}


@router.post("/login")
def login(credentials: Credentials, db: Session = Depends(get_db)):
    user = db.query(User).filter(User.email == credentials.email).first()
    if user is None or user.hashed_password != hash_password(credentials.password):
        raise HTTPException(status_code=401, detail="Invalid email or password")
    return {"access_token": create_access_token(user.id), "token_type": "bearer"}


@router.get("/me")
def me(user: User = Depends(get_current_user)):
    return {"id": user.id, "email": user.email}

@router.post("/github-token")
def save_github_token(
    request: GitHubToken,
    user: User = Depends(get_current_user),
    db: Session = Depends(get_db),
):
    user.github_token = request.github_access_token
    db.commit()
    return {"saved": True}
