import base64
import hashlib
import hmac
import html
import json
import re
from urllib.parse import urlencode

from pydantic import BaseModel, Field
from fastapi import APIRouter, Depends, HTTPException, Request
from fastapi.responses import HTMLResponse, RedirectResponse
from sqlalchemy import func
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from auth import (
    create_access_token,
    exchange_google_code,
    get_current_user,
    google_web_client_id,
    google_web_client_secret,
    hash_password,
    read_handoff,
    sign_handoff,
    verify_google_credential,
)
from database import User
from database.session import get_db

router = APIRouter(prefix="/auth", tags=["auth"])


class Credentials(BaseModel):
    email: str = Field(min_length=3, max_length=255)
    password: str = Field(min_length=8, max_length=128)
    
class GitHubToken(BaseModel):
    github_access_token: str = Field(min_length=1, max_length=500)


class GoogleCredential(BaseModel):
    # The ID token from Google Identity Services (never logged).
    credential: str = Field(min_length=1, max_length=4096)


class GoogleMobileExchange(BaseModel):
    code: str = Field(min_length=1, max_length=1024)
    verifier: str = Field(min_length=43, max_length=128)


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
    if user is not None and user.hashed_password is None:
        raise HTTPException(status_code=401, detail="This account uses Google sign-in. Continue with Google instead.")
    if user is None or user.hashed_password != hash_password(credentials.password):
        raise HTTPException(status_code=401, detail="Invalid email or password")
    return {"access_token": create_access_token(user.id), "token_type": "bearer"}


@router.get("/me")
def me(user: User = Depends(get_current_user)):
    return {
        "id": user.id,
        "email": user.email,
        "has_github_token": bool(user.github_token),
        "google_linked": bool(user.google_sub),
    }


@router.get("/google/config")
def google_config():
    """Public Web Client ID for the sign-in button, so it always matches the audience checked below."""
    client_id = google_web_client_id()
    if not client_id:
        raise HTTPException(status_code=503, detail="Google sign-in is not configured.")
    return {"client_id": client_id}


def _save_google_profile(user: User, claims: dict):
    user.google_sub = claims["sub"]
    user.google_name = claims.get("name")
    user.google_picture = claims.get("picture")


@router.post("/google")
def google_login(request: GoogleCredential, db: Session = Depends(get_db)):
    """
    Sign in (or sign up) with a Google ID token. Safe to repeat: the same
    token always resolves to the same user, so AWS -> Render failover can
    resend it.
    """
    user = _google_account(verify_google_credential(request.credential), db)
    return {"access_token": create_access_token(user.id), "token_type": "bearer"}


def _google_account(claims: dict, db: Session) -> User:
    """The user for verified Google claims: linked account, else a new Google-only one."""
    user = db.query(User).filter(User.google_sub == claims["sub"]).first()
    if user is None:
        # Never merge into an existing account automatically - its owner has
        # to sign in the usual way and link Google themselves.
        email = claims["email"].strip()
        if db.query(User).filter(func.lower(User.email) == email.lower()).first():
            raise HTTPException(
                status_code=409,
                detail=(
                    "An account with this email already exists. Log in with your "
                    "email and password, then link Google to that account."
                ),
            )

        user = User(email=email, hashed_password=None)
        _save_google_profile(user, claims)
        db.add(user)
        try:
            db.commit()
        except IntegrityError:
            # The same sign-in raced us (e.g. retried on the other backend).
            db.rollback()
            user = db.query(User).filter(User.google_sub == claims["sub"]).first()
            if user is None:
                raise HTTPException(status_code=409, detail="An account with this email already exists.")
        db.refresh(user)
    elif user.google_name != claims.get("name") or user.google_picture != claims.get("picture"):
        _save_google_profile(user, claims)
        db.commit()

    return user


# ---------------------------------------------------------------------------
# Android sign-in. The app can't use Google's web button, so it opens
# /auth/google/mobile/start in the system browser with a PKCE challenge
# (RFC 8252). Google returns to /callback here; we verify the account with
# the same rules as the web and send the browser back to the app
# (chatwithrepo://auth) with a 2-minute code. The app redeems that code at
# /exchange together with its secret verifier, so another app that grabs
# the chatwithrepo:// link can't use it. All values are signed, nothing is
# stored, so AWS and Render can each serve any step.
# ---------------------------------------------------------------------------

APP_RETURN_URL = "chatwithrepo://auth"
_CHALLENGE = re.compile(r"^[A-Za-z0-9_-]{43}$")


def _mobile_callback_url(request: Request) -> str:
    host = request.headers.get("host", "")
    scheme = "http" if host.split(":")[0] in ("127.0.0.1", "localhost") else "https"
    return f"{scheme}://{host}/auth/google/mobile/callback"


def _return_to_app(**params) -> HTMLResponse:
    """A page that hands the result to the app (and a button, in case the browser needs a tap)."""
    target = f"{APP_RETURN_URL}?{urlencode(params)}"
    message = "Signed in with Google." if "code" in params else params.get("error", "")
    return HTMLResponse(
        f"""<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>ChatWithRepo</title>
<style>
  body {{ margin: 0; min-height: 100vh; display: grid; place-items: center; padding: 24px;
         font-family: system-ui, sans-serif; background: #07130f; color: #eaf7f1; text-align: center; }}
  p {{ color: #9fc2b3; line-height: 1.5; max-width: 320px; }}
  a {{ display: inline-block; margin-top: 8px; padding: 14px 24px; border-radius: 12px; font-weight: 600;
       color: #04241a; text-decoration: none; background: linear-gradient(135deg, #55d6a4, #0e9f6e); }}
</style></head>
<body><main>
  <h1>ChatWithRepo</h1>
  <p>{html.escape(message)}</p>
  <a href="{html.escape(target)}">Return to the app</a>
</main>
<script>location.replace({json.dumps(target)});</script>
</body></html>""",
        headers={"Cache-Control": "no-store", "Referrer-Policy": "no-referrer"},
    )


@router.get("/google/mobile/start")
def google_mobile_start(challenge: str, request: Request):
    if not _CHALLENGE.match(challenge):
        return _return_to_app(error="Google sign-in could not start. Please try again.")
    if not google_web_client_id() or not google_web_client_secret():
        return _return_to_app(error="Google sign-in is not configured.")

    redirect_uri = _mobile_callback_url(request)
    state = sign_handoff("state", {"c": challenge, "r": redirect_uri}, ttl_seconds=600)
    query = urlencode({
        "client_id": google_web_client_id(),
        "redirect_uri": redirect_uri,
        "response_type": "code",
        "scope": "openid email profile",
        "state": state,
        "prompt": "select_account",
    })
    return RedirectResponse(f"https://accounts.google.com/o/oauth2/v2/auth?{query}", status_code=302)


@router.get("/google/mobile/callback")
def google_mobile_callback(
    state: str = "",
    code: str = "",
    error: str = "",
    db: Session = Depends(get_db),
):
    flow = read_handoff("state", state)
    if flow is None:
        return _return_to_app(error="Google sign-in expired. Please try again.")
    if error or not code:
        return _return_to_app(error="Google sign-in was cancelled.")

    try:
        user = _google_account(exchange_google_code(code, flow["r"]), db)
    except HTTPException as e:
        return _return_to_app(error=str(e.detail))

    return _return_to_app(code=sign_handoff("code", {"u": user.id, "c": flow["c"]}, ttl_seconds=120))


@router.post("/google/mobile/exchange")
def google_mobile_exchange(request: GoogleMobileExchange, db: Session = Depends(get_db)):
    """Swap the app's code + PKCE verifier for the usual JWT. Safe to repeat (failover)."""
    expired = HTTPException(status_code=400, detail="Google sign-in expired. Please try again.")
    data = read_handoff("code", request.code)
    if data is None:
        raise expired

    challenge = base64.urlsafe_b64encode(hashlib.sha256(request.verifier.encode()).digest()).rstrip(b"=").decode()
    if not hmac.compare_digest(challenge, data["c"]):
        raise expired

    user = db.query(User).filter(User.id == data["u"]).first()
    if user is None:
        raise expired
    return {"access_token": create_access_token(user.id), "token_type": "bearer"}


@router.post("/google/link")
def link_google(
    request: GoogleCredential,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db),
):
    """Link a Google account to the signed-in user, so they can use either way to log in."""
    claims = verify_google_credential(request.credential)

    user = db.query(User).filter(User.id == current_user.id).first()
    if user is None:
        raise HTTPException(status_code=401, detail="User not found")

    owner = db.query(User).filter(User.google_sub == claims["sub"]).first()
    if owner is not None and owner.id != user.id:
        raise HTTPException(status_code=409, detail="This Google account is already linked to another account.")
    if user.google_sub and user.google_sub != claims["sub"]:
        raise HTTPException(status_code=409, detail="A different Google account is already linked.")

    _save_google_profile(user, claims)
    try:
        db.commit()
    except IntegrityError:
        db.rollback()
        raise HTTPException(status_code=409, detail="This Google account is already linked to another account.")

    return {"linked": True}

@router.post("/github-token")
def save_github_token(
    request: GitHubToken,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db),
):
    token = request.github_access_token.strip()
    if not token:
        raise HTTPException(status_code=400, detail="GitHub token cannot be empty.")

    # Load user in THIS session so the commit actually persists
    user = db.query(User).filter(User.id == current_user.id).first()
    if user is None:
        raise HTTPException(status_code=401, detail="User not found")

    user.github_token = token
    db.commit()
    db.refresh(user)

    return {
        "saved": True,
        "has_github_token": bool(user.github_token),
    }