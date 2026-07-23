from __future__ import annotations

import base64
import hashlib
import hmac
import json
import os
import time

from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from sqlalchemy.orm import Session

from database import User
from database.session import get_db

_SECRET = os.getenv("JWT_SECRET", "devlens-change-this-secret")
_bearer = HTTPBearer(auto_error=False)


def _b64(value: bytes) -> str:
    return base64.urlsafe_b64encode(value).rstrip(b"=").decode()


def create_access_token(user_id: int) -> str:
    header = _b64(json.dumps({"alg": "HS256", "typ": "JWT"}, separators=(",", ":")).encode())
    payload = _b64(json.dumps({"sub": str(user_id), "exp": int(time.time()) + 86400}, separators=(",", ":")).encode())
    unsigned = f"{header}.{payload}".encode()
    signature = _b64(hmac.new(_SECRET.encode(), unsigned, hashlib.sha256).digest())
    return f"{header}.{payload}.{signature}"


def _decode_token(token: str) -> int:
    try:
        header, payload, signature = token.split(".")
        unsigned = f"{header}.{payload}".encode()
        expected = _b64(hmac.new(_SECRET.encode(), unsigned, hashlib.sha256).digest())
        if not hmac.compare_digest(signature, expected):
            raise ValueError
        data = json.loads(base64.urlsafe_b64decode(payload + "=" * (-len(payload) % 4)))
        if data["exp"] < time.time():
            raise ValueError
        return int(data["sub"])
    except (ValueError, KeyError, TypeError, json.JSONDecodeError):
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Invalid or expired token")


def get_current_user(
    credentials: HTTPAuthorizationCredentials | None = Depends(_bearer),
    db: Session = Depends(get_db),
) -> User:
    if credentials is None:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Login required")
    user = db.query(User).filter(User.id == _decode_token(credentials.credentials)).first()
    if user is None:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="User not found")
    return user


def hash_password(password: str) -> str:
    return hashlib.sha256(password.encode()).hexdigest()
