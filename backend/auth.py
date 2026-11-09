from __future__ import annotations

import base64
import hashlib
import hmac
import json
import os
import time

import requests
from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from google.auth import exceptions as google_exceptions
from google.auth.transport import requests as google_requests
from google.oauth2 import id_token
from sqlalchemy.orm import Session

from backend.database import User
from backend.database.session import get_db

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


def google_web_client_id() -> str:
    return os.getenv("GOOGLE_WEB_CLIENT_ID", "").strip()


def google_web_client_secret() -> str:
    # Only the Android sign-in uses it (server-side code exchange).
    return os.getenv("GOOGLE_WEB_CLIENT_SECRET", "").strip()


# Short-lived signed values for the Android sign-in: the OAuth state and the
# one-time code handed back to the app. "payload.signature" with their own
# key, so they can never pass as an access token (three parts, _SECRET) or
# the other way round.
_HANDOFF_KEY = hmac.new(_SECRET.encode(), b"google-mobile-handoff", hashlib.sha256).digest()


def sign_handoff(kind: str, data: dict, ttl_seconds: int) -> str:
    payload = _b64(
        json.dumps({**data, "kind": kind, "exp": int(time.time()) + ttl_seconds}, separators=(",", ":")).encode()
    )
    signature = _b64(hmac.new(_HANDOFF_KEY, payload.encode(), hashlib.sha256).digest())
    return f"{payload}.{signature}"


def read_handoff(kind: str, value: str) -> dict | None:
    """The data signed by sign_handoff, or None if forged, expired or of another kind."""
    try:
        payload, signature = value.split(".")
        expected = _b64(hmac.new(_HANDOFF_KEY, payload.encode(), hashlib.sha256).digest())
        if not hmac.compare_digest(signature, expected):
            return None
        data = json.loads(base64.urlsafe_b64decode(payload + "=" * (-len(payload) % 4)))
    except (ValueError, TypeError, json.JSONDecodeError):
        return None
    if not isinstance(data, dict) or data.get("kind") != kind or data.get("exp", 0) < time.time():
        return None
    return data


def exchange_google_code(code: str, redirect_uri: str) -> dict:
    """Trade an OAuth authorization code for verified Google claims (Android sign-in)."""
    global _google_session

    client_id, client_secret = google_web_client_id(), google_web_client_secret()
    if not client_id or not client_secret:
        raise HTTPException(status_code=503, detail="Google sign-in is not configured.")
    if _google_session is None:
        _google_session = requests.Session()

    try:
        response = _google_session.post(
            "https://oauth2.googleapis.com/token",
            data={
                "code": code,
                "client_id": client_id,
                "client_secret": client_secret,
                "redirect_uri": redirect_uri,
                "grant_type": "authorization_code",
            },
            timeout=15,
        )
    except requests.RequestException:
        raise HTTPException(status_code=503, detail="Could not reach Google to verify the sign-in. Try again.")
    if not response.ok or not response.json().get("id_token"):
        raise HTTPException(status_code=401, detail="Google sign-in failed or expired. Please try again.")

    return verify_google_credential(response.json()["id_token"])


# Reused across requests so fetching Google's signing certs keeps the
# connection alive.
_google_session = None


def verify_google_credential(credential: str) -> dict:
    """
    Verify a Google Identity Services ID token (signature, audience, issuer,
    expiry) and return its claims. Only verified Google emails are accepted.
    The token itself is never logged.
    """
    global _google_session

    client_id = google_web_client_id()
    if not client_id:
        raise HTTPException(status_code=503, detail="Google sign-in is not configured.")

    if _google_session is None:
        _google_session = requests.Session()

    try:
        claims = id_token.verify_oauth2_token(
            credential,
            google_requests.Request(session=_google_session),
            client_id,
            clock_skew_in_seconds=10,
        )
    except google_exceptions.TransportError:
        raise HTTPException(status_code=503, detail="Could not reach Google to verify the sign-in. Try again.")
    except (ValueError, google_exceptions.GoogleAuthError):
        # Bad signature, expired, wrong audience/issuer or malformed.
        raise HTTPException(status_code=401, detail="Google sign-in failed or expired. Please try again.")

    if not claims.get("sub") or not claims.get("email"):
        raise HTTPException(status_code=401, detail="Google sign-in failed. Please try again.")
    if claims.get("email_verified") is not True:
        raise HTTPException(status_code=401, detail="Your Google email address is not verified.")

    return claims
