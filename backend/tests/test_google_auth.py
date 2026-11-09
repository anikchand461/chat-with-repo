"""
Tests for Google sign-in (POST /auth/google, /auth/google/link,
GET /auth/google/config), the Android browser flow (/auth/google/mobile/*)
and that email/password auth still works.

ID tokens are really signed with a throwaway RSA key and checked by the
real google-auth verifier; only the download of Google's signing certs is
faked. Runs against a throwaway SQLite database - no network:

    .venv/bin/python -m unittest backend.tests.test_google_auth -v
"""

import base64
import datetime
import hashlib
import html
import os
import re
import tempfile
import time
import unittest
from unittest import mock
from urllib.parse import parse_qs, urlparse

_TMP = tempfile.mkdtemp(prefix="cwr-google-test-")
os.environ["DATABASE_URL"] = f"sqlite:///{_TMP}/test.db"
os.environ["DATA_DIR"] = f"{_TMP}/data"
os.environ["JWT_SECRET"] = "test-secret"

from backend.database import db as _db  # noqa: E402  (creates the SQLite tables)

if _db.engine.url.get_backend_name() != "sqlite":
    raise RuntimeError("Refusing to run: tests must use the throwaway SQLite database")

from cryptography import x509  # noqa: E402
from cryptography.hazmat.primitives import hashes, serialization  # noqa: E402
from cryptography.hazmat.primitives.asymmetric import rsa  # noqa: E402
from cryptography.x509.oid import NameOID  # noqa: E402
from fastapi import HTTPException  # noqa: E402
from fastapi.testclient import TestClient  # noqa: E402
from google.auth import crypt, exceptions as google_exceptions, jwt as google_jwt  # noqa: E402
from sqlalchemy import inspect, text  # noqa: E402

from backend.app import app  # noqa: E402
from backend.auth import hash_password  # noqa: E402
from backend.database import Chat, SessionLocal, User  # noqa: E402

CLIENT_ID = "test-client.apps.googleusercontent.com"
KEY_ID = "test-key"


def _make_key():
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "test")])
    now = datetime.datetime.now(datetime.timezone.utc)
    cert = (
        x509.CertificateBuilder()
        .subject_name(name)
        .issuer_name(name)
        .public_key(key.public_key())
        .serial_number(1)
        .not_valid_before(now - datetime.timedelta(days=1))
        .not_valid_after(now + datetime.timedelta(days=1))
        .sign(key, hashes.SHA256())
    )
    private_pem = key.private_bytes(
        serialization.Encoding.PEM,
        serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption(),
    )
    return crypt.RSASigner.from_string(private_pem, KEY_ID), cert.public_bytes(serialization.Encoding.PEM)


SIGNER, CERT_PEM = _make_key()
OTHER_SIGNER, _ = _make_key()  # not in Google's certs -> bad signature

_next = iter(range(1, 100000))


def google_token(signer=SIGNER, **overrides):
    n = next(_next)
    now = int(time.time())
    claims = {
        "iss": "https://accounts.google.com",
        "aud": CLIENT_ID,
        "sub": f"google-sub-{n}",
        "email": f"google{n}@example.com",
        "email_verified": True,
        "name": f"Google User {n}",
        "picture": f"https://example.com/{n}.png",
        "iat": now,
        "exp": now + 3600,
    }
    claims.update(overrides)
    claims = {k: v for k, v in claims.items() if v is not None}
    return google_jwt.encode(signer, claims).decode(), claims


class GoogleAuthTests(unittest.TestCase):
    def setUp(self):
        self.client = TestClient(app)
        patches = [
            mock.patch.dict(os.environ, {"GOOGLE_WEB_CLIENT_ID": CLIENT_ID}),
            mock.patch("google.oauth2.id_token._fetch_certs", return_value={KEY_ID: CERT_PEM}),
        ]
        for p in patches:
            p.start()
            self.addCleanup(p.stop)

    def google(self, token):
        return self.client.post("/auth/google", json={"credential": token})

    def me(self, access_token):
        return self.client.get("/auth/me", headers={"Authorization": f"Bearer {access_token}"})

    def password_user(self, email, password="password123"):
        db = SessionLocal()
        try:
            user = User(email=email, hashed_password=hash_password(password), plan="PRO")
            db.add(user)
            db.commit()
            db.add(Chat(user_id=user.id, title="o/r", owner="o", repo="r", collection_name=f"c-{user.id}"))
            db.commit()
            return user.id
        finally:
            db.close()

    # --- token verification -------------------------------------------

    def test_missing_credential(self):
        self.assertEqual(self.client.post("/auth/google", json={}).status_code, 422)
        self.assertEqual(self.client.post("/auth/google", json={"credential": ""}).status_code, 422)

    def test_invalid_token(self):
        r = self.google("not-a-jwt")
        self.assertEqual(r.status_code, 401)
        self.assertNotIn("not-a-jwt", r.text)

    def test_bad_signature(self):
        token, _ = google_token(signer=OTHER_SIGNER)
        self.assertEqual(self.google(token).status_code, 401)

    def test_expired_token(self):
        past = int(time.time()) - 7200
        token, _ = google_token(iat=past, exp=past + 3600)
        self.assertEqual(self.google(token).status_code, 401)

    def test_wrong_audience(self):
        token, _ = google_token(aud="someone-else.apps.googleusercontent.com")
        self.assertEqual(self.google(token).status_code, 401)

    def test_wrong_issuer(self):
        token, _ = google_token(iss="https://evil.example.com")
        self.assertEqual(self.google(token).status_code, 401)

    def test_unverified_email(self):
        token, claims = google_token(email_verified=False)
        r = self.google(token)
        self.assertEqual(r.status_code, 401)
        self.assertIn("not verified", r.json()["detail"])
        self._assert_no_user(claims["sub"])

    def test_missing_email_verified_claim(self):
        token, claims = google_token(email_verified=None)
        self.assertEqual(self.google(token).status_code, 401)
        self._assert_no_user(claims["sub"])

    def test_not_configured(self):
        token, _ = google_token()
        with mock.patch.dict(os.environ, {"GOOGLE_WEB_CLIENT_ID": ""}):
            self.assertEqual(self.google(token).status_code, 503)
            self.assertEqual(self.client.get("/auth/google/config").status_code, 503)

    def test_google_unreachable(self):
        token, _ = google_token()
        with mock.patch(
            "google.oauth2.id_token._fetch_certs",
            side_effect=google_exceptions.TransportError("down"),
        ):
            self.assertEqual(self.google(token).status_code, 503)

    def test_config_returns_public_client_id(self):
        r = self.client.get("/auth/google/config")
        self.assertEqual(r.status_code, 200)
        self.assertEqual(r.json(), {"client_id": CLIENT_ID})

    # --- accounts -----------------------------------------------------

    def test_new_google_account(self):
        token, claims = google_token()
        r = self.google(token)
        self.assertEqual(r.status_code, 200)
        body = r.json()
        self.assertEqual(body["token_type"], "bearer")
        self.assertEqual(set(body), {"access_token", "token_type"})

        me = self.me(body["access_token"])
        self.assertEqual(me.status_code, 200)
        self.assertEqual(me.json()["email"], claims["email"])
        self.assertTrue(me.json()["google_linked"])

        db = SessionLocal()
        try:
            user = db.query(User).filter(User.google_sub == claims["sub"]).one()
            self.assertIsNone(user.hashed_password)
            self.assertEqual(user.google_name, claims["name"])
            self.assertEqual(user.google_picture, claims["picture"])
            self.assertEqual(user.plan, "FREE")
        finally:
            db.close()

    def test_existing_linked_account_signs_in_again(self):
        token, claims = google_token()
        first = self.google(token).json()
        # Same Google account later, with an updated name.
        token2, _ = google_token(sub=claims["sub"], email=claims["email"], name="New Name")
        second = self.google(token2)
        self.assertEqual(second.status_code, 200)
        self.assertEqual(self.me(first["access_token"]).json()["id"], self.me(second.json()["access_token"]).json()["id"])
        db = SessionLocal()
        try:
            users = db.query(User).filter(User.google_sub == claims["sub"]).all()
            self.assertEqual(len(users), 1)
            self.assertEqual(users[0].google_name, "New Name")
        finally:
            db.close()

    def test_same_token_twice_is_idempotent(self):
        # What AWS -> Render failover does when it resends the request.
        token, _ = google_token()
        a, b = self.google(token), self.google(token)
        self.assertEqual(a.status_code, 200)
        self.assertEqual(b.status_code, 200)
        self.assertEqual(self.me(a.json()["access_token"]).json()["id"], self.me(b.json()["access_token"]).json()["id"])

    def test_existing_password_account_is_not_merged(self):
        email = f"pw{next(_next)}@example.com"
        user_id = self.password_user(email)
        token, claims = google_token(email=email.upper())  # case must not matter
        r = self.google(token)
        self.assertEqual(r.status_code, 409)
        self.assertIn("Log in with your email and password", r.json()["detail"])
        db = SessionLocal()
        try:
            self.assertIsNone(db.get(User, user_id).google_sub)
            self._assert_no_user(claims["sub"])
        finally:
            db.close()

    def test_link_then_sign_in_with_google(self):
        email = f"pw{next(_next)}@example.com"
        user_id = self.password_user(email)
        login = self.client.post("/auth/login", json={"email": email, "password": "password123"})
        self.assertEqual(login.status_code, 200)
        jwt = login.json()["access_token"]

        token, _ = google_token(email=email)
        self.assertEqual(self.client.post("/auth/google/link", json={"credential": token}).status_code, 401)
        r = self.client.post(
            "/auth/google/link", json={"credential": token}, headers={"Authorization": f"Bearer {jwt}"}
        )
        self.assertEqual(r.status_code, 200)

        signed_in = self.google(token)
        self.assertEqual(signed_in.status_code, 200)
        me = self.me(signed_in.json()["access_token"]).json()
        self.assertEqual(me["id"], user_id)
        self.assertTrue(me["google_linked"])

        # Chats and subscription are untouched.
        chats = self.client.get("/chat/list", headers={"Authorization": f"Bearer {signed_in.json()['access_token']}"})
        self.assertEqual(chats.status_code, 200)
        self.assertEqual([c["title"] for c in chats.json()], ["o/r"])
        db = SessionLocal()
        try:
            self.assertEqual(db.get(User, user_id).plan, "PRO")
        finally:
            db.close()

        # The password still works too.
        self.assertEqual(
            self.client.post("/auth/login", json={"email": email, "password": "password123"}).status_code, 200
        )

    def test_link_google_account_owned_by_someone_else(self):
        token, _ = google_token()
        self.assertEqual(self.google(token).status_code, 200)
        email = f"pw{next(_next)}@example.com"
        self.password_user(email)
        jwt = self.client.post("/auth/login", json={"email": email, "password": "password123"}).json()["access_token"]
        r = self.client.post(
            "/auth/google/link", json={"credential": token}, headers={"Authorization": f"Bearer {jwt}"}
        )
        self.assertEqual(r.status_code, 409)

    def test_google_only_account_cannot_password_login(self):
        token, claims = google_token()
        self.assertEqual(self.google(token).status_code, 200)
        r = self.client.post("/auth/login", json={"email": claims["email"], "password": "anything123"})
        self.assertEqual(r.status_code, 401)
        self.assertIn("Google", r.json()["detail"])

    # --- existing password auth ---------------------------------------

    def test_password_register_and_login_still_work(self):
        email = f"new{next(_next)}@example.com"
        reg = self.client.post("/auth/register", json={"email": email, "password": "password123"})
        self.assertEqual(reg.status_code, 200)
        self.assertEqual(self.me(reg.json()["access_token"]).json()["email"], email)
        self.assertFalse(self.me(reg.json()["access_token"]).json()["google_linked"])

        login = self.client.post("/auth/login", json={"email": email, "password": "password123"})
        self.assertEqual(login.status_code, 200)
        self.assertEqual(
            self.client.post("/auth/login", json={"email": email, "password": "wrongpass1"}).status_code, 401
        )
        self.assertEqual(
            self.client.post("/auth/register", json={"email": email, "password": "password123"}).status_code, 409
        )

    def _assert_no_user(self, sub):
        db = SessionLocal()
        try:
            self.assertIsNone(db.query(User).filter(User.google_sub == sub).first())
        finally:
            db.close()


class FakeTokenResponse:
    def __init__(self, status, body):
        self.ok = status < 400
        self._body = body

    def json(self):
        return self._body


class GoogleMobileFlowTests(unittest.TestCase):
    """Android: browser -> /start -> Google -> /callback -> chatwithrepo:// -> /exchange."""

    def setUp(self):
        self.client = TestClient(app)
        self.verifier = "v" * 43 + next_suffix()
        self.challenge = pkce_challenge(self.verifier)
        self.token_calls = []
        patches = [
            mock.patch.dict(os.environ, {"GOOGLE_WEB_CLIENT_ID": CLIENT_ID, "GOOGLE_WEB_CLIENT_SECRET": "shh"}),
            mock.patch("google.oauth2.id_token._fetch_certs", return_value={KEY_ID: CERT_PEM}),
        ]
        for p in patches:
            p.start()
            self.addCleanup(p.stop)

    def start(self, challenge=None):
        return self.client.get(
            "/auth/google/mobile/start", params={"challenge": challenge or self.challenge}, follow_redirects=False
        )

    def callback(self, id_token, google_status=200, **params):
        """Run /start, then come back from Google with an authorization code."""
        query = parse_qs(urlparse(self.start().headers["location"]).query)
        session = mock.Mock()
        session.post.side_effect = lambda url, data, timeout: (
            self.token_calls.append(data) or FakeTokenResponse(google_status, {"id_token": id_token})
        )
        with mock.patch("backend.auth._google_session", session), mock.patch("auth._google_session", session):
            return self.client.get(
                "/auth/google/mobile/callback",
                params={"state": query["state"][0], "code": "google-auth-code", **params},
            )

    def app_link(self, page):
        """The chatwithrepo:// URL the callback page sends the browser to."""
        href = re.search(r'href="([^"]+)"', page.text).group(1)
        link = urlparse(html.unescape(href))
        self.assertEqual((link.scheme, link.netloc), ("chatwithrepo", "auth"))
        return {k: v[0] for k, v in parse_qs(link.query).items()}

    def exchange(self, code, verifier=None):
        return self.client.post(
            "/auth/google/mobile/exchange", json={"code": code, "verifier": verifier or self.verifier}
        )

    def test_start_redirects_to_google(self):
        r = self.start()
        self.assertEqual(r.status_code, 302)
        target = urlparse(r.headers["location"])
        query = parse_qs(target.query)
        self.assertEqual(target.netloc, "accounts.google.com")
        self.assertEqual(query["client_id"], [CLIENT_ID])
        self.assertEqual(query["response_type"], ["code"])
        self.assertEqual(query["scope"], ["openid email profile"])
        self.assertEqual(query["redirect_uri"], ["http://testserver/auth/google/mobile/callback".replace("http", "https")])
        self.assertNotIn("shh", r.headers["location"], "the client secret must never leave the server")

    def test_start_rejects_a_bad_challenge_and_missing_config(self):
        self.assertIn("could not start", self.start(challenge="short").text)
        with mock.patch.dict(os.environ, {"GOOGLE_WEB_CLIENT_SECRET": ""}):
            self.assertIn("not configured", self.start().text)

    def test_full_flow_new_account(self):
        token, claims = google_token()
        page = self.callback(token)
        self.assertEqual(page.status_code, 200)
        self.assertEqual(self.token_calls[0]["client_secret"], "shh")
        self.assertEqual(self.token_calls[0]["code"], "google-auth-code")
        code = self.app_link(page)["code"]

        r = self.exchange(code)
        self.assertEqual(r.status_code, 200)
        me = self.client.get("/auth/me", headers={"Authorization": f"Bearer {r.json()['access_token']}"}).json()
        self.assertEqual(me["email"], claims["email"])
        self.assertTrue(me["google_linked"])
        # Repeating the exchange (failover resend) gives the same account.
        again = self.exchange(code).json()["access_token"]
        self.assertEqual(self.client.get("/auth/me", headers={"Authorization": f"Bearer {again}"}).json()["id"], me["id"])

    def test_wrong_verifier_is_rejected(self):
        token, _ = google_token()
        code = self.app_link(self.callback(token))["code"]
        r = self.exchange(code, verifier="x" * 50)
        self.assertEqual(r.status_code, 400)
        self.assertIn("expired", r.json()["detail"])

    def test_expired_code_is_rejected(self):
        token, _ = google_token()
        code = self.app_link(self.callback(token))["code"]
        with mock.patch("time.time", return_value=time.time() + 600):
            self.assertEqual(self.exchange(code).status_code, 400)

    def test_tampered_or_reused_values_are_rejected(self):
        token, _ = google_token()
        code = self.app_link(self.callback(token))["code"]
        payload, sig = code.split(".")
        self.assertEqual(self.exchange(payload + "." + sig[::-1]).status_code, 400)
        # A code is not an access token, and the OAuth state is not a code.
        self.assertEqual(self.client.get("/auth/me", headers={"Authorization": f"Bearer {code}"}).status_code, 401)
        state = parse_qs(urlparse(self.start().headers["location"]).query)["state"][0]
        self.assertEqual(self.exchange(state).status_code, 400)

    def test_forged_state_is_rejected(self):
        r = self.client.get("/auth/google/mobile/callback", params={"state": "abc.def", "code": "x"})
        self.assertIn("expired", self.app_link(r)["error"])

    def test_user_cancelled_at_google(self):
        state = parse_qs(urlparse(self.start().headers["location"]).query)["state"][0]
        r = self.client.get("/auth/google/mobile/callback", params={"state": state, "error": "access_denied"})
        self.assertIn("cancelled", self.app_link(r)["error"])

    def test_google_rejects_the_code(self):
        token, _ = google_token()
        self.assertIn("failed", self.app_link(self.callback(token, google_status=400))["error"])

    def test_unverified_email_and_existing_account(self):
        token, _ = google_token(email_verified=False)
        self.assertIn("not verified", self.app_link(self.callback(token))["error"])

        email = f"pw{next(_next)}@example.com"
        db = SessionLocal()
        db.add(User(email=email, hashed_password=hash_password("password123")))
        db.commit()
        db.close()
        token, _ = google_token(email=email)
        self.assertIn("already exists", self.app_link(self.callback(token))["error"])

    def test_error_text_is_escaped(self):
        token, _ = google_token()
        with mock.patch("backend.api.auth.exchange_google_code", side_effect=HTTPException(401, "<script>x</script>")):
            page = self.callback(token)
        self.assertNotIn("<script>x</script>", page.text)
        self.assertIn("&lt;script&gt;x&lt;/script&gt;", page.text)
        self.assertEqual(self.app_link(page)["error"], "<script>x</script>")


def pkce_challenge(verifier):
    return base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).rstrip(b"=").decode()


def next_suffix():
    return str(next(_next))


class MigrationTests(unittest.TestCase):
    def test_adds_google_columns_to_an_old_users_table(self):
        """An existing database (no Google columns) is upgraded in place, data kept, and re-running is a no-op."""
        from sqlalchemy import create_engine

        engine = create_engine(f"sqlite:///{tempfile.mkdtemp(prefix='cwr-mig-')}/old.db")
        with engine.begin() as conn:
            conn.execute(text("CREATE TABLE users (id INTEGER PRIMARY KEY, email VARCHAR, hashed_password VARCHAR, plan VARCHAR)"))
            conn.execute(text("CREATE TABLE messages (id INTEGER PRIMARY KEY, chat_id INTEGER, content VARCHAR)"))
            conn.execute(text("INSERT INTO users (email, hashed_password, plan) VALUES ('old@example.com', 'h', 'PRO')"))

        with mock.patch.object(_db, "engine", engine):
            first = _db._add_missing_columns()
            second = _db._add_missing_columns()

        self.assertTrue(any("google_sub" in s for s in first))
        self.assertTrue(any("ix_users_google_sub" in s for s in first))
        self.assertEqual(second, [])

        columns = {c["name"] for c in inspect(engine).get_columns("users")}
        self.assertTrue({"google_sub", "google_name", "google_picture"} <= columns)
        unique = [i for i in inspect(engine).get_indexes("users") if i["name"] == "ix_users_google_sub"]
        self.assertTrue(unique and unique[0]["unique"])
        with engine.connect() as conn:
            row = conn.execute(text("SELECT email, hashed_password, plan, google_sub FROM users")).one()
        self.assertEqual(tuple(row), ("old@example.com", "h", "PRO", None))


if __name__ == "__main__":
    unittest.main()
