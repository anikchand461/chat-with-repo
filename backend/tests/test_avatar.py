"""
Tests for the profile picture (GET/POST/DELETE /auth/avatar and the
"avatar" field of /auth/me). Runs against a throwaway SQLite database:

    .venv/bin/python -m unittest backend.tests.test_avatar -v
"""

import base64
import os
import tempfile
import unittest
import uuid

_TMP = tempfile.mkdtemp(prefix="cwr-avatar-test-")
os.environ.setdefault("DATABASE_URL", f"sqlite:///{_TMP}/test.db")
os.environ.setdefault("DATA_DIR", f"{_TMP}/data")
os.environ.setdefault("JWT_SECRET", "test-secret")

from backend.database import db as _db  # noqa: E402  (creates the SQLite tables)

if _db.engine.url.get_backend_name() != "sqlite":
    raise RuntimeError("Refusing to run: tests must use the throwaway SQLite database")

from fastapi.testclient import TestClient  # noqa: E402

from backend.app import app  # noqa: E402
from backend.auth import create_access_token  # noqa: E402
from backend.database import SessionLocal, User  # noqa: E402

JPEG = b"\xff\xd8\xff\xe0" + b"\x00" * 64
PNG = b"\x89PNG\r\n\x1a\n" + b"\x00" * 64


def b64(data: bytes) -> str:
    return base64.b64encode(data).decode()


class AvatarTests(unittest.TestCase):
    def setUp(self):
        self.client = TestClient(app)

    def make_user(self, google_picture=None) -> dict:
        with SessionLocal() as db:
            user = User(email=f"{uuid.uuid4().hex}@example.com", hashed_password="x", google_picture=google_picture)
            db.add(user)
            db.commit()
            return {"Authorization": f"Bearer {create_access_token(user.id)}"}

    def me(self, headers):
        return self.client.get("/auth/me", headers=headers).json()

    def test_password_account_starts_with_default(self):
        headers = self.make_user()
        me = self.me(headers)
        self.assertEqual(me["avatar"], "default")
        self.assertIsNone(me["google_picture"])
        self.assertEqual(self.client.get("/auth/avatar", headers=headers).status_code, 404)

    def test_google_account_starts_with_google_picture(self):
        headers = self.make_user(google_picture="https://lh3.googleusercontent.com/a/x")
        me = self.me(headers)
        self.assertEqual(me["avatar"], "google")
        self.assertEqual(me["google_picture"], "https://lh3.googleusercontent.com/a/x")

    def test_upload_replaces_and_delete_restores(self):
        headers = self.make_user(google_picture="https://lh3.googleusercontent.com/a/x")

        r = self.client.post("/auth/avatar", headers=headers, json={"image": b64(JPEG)})
        self.assertEqual(r.status_code, 200)
        self.assertEqual(r.json()["avatar"], "custom")
        self.assertEqual(self.me(headers)["avatar"], "custom")

        got = self.client.get("/auth/avatar", headers=headers).json()
        self.assertEqual(got["content_type"], "image/jpeg")
        self.assertEqual(base64.b64decode(got["data"]), JPEG)

        # Uploading again (e.g. replayed on the other backend) just replaces it.
        r = self.client.post("/auth/avatar", headers=headers, json={"image": b64(PNG)})
        self.assertEqual(r.status_code, 200)
        self.assertEqual(self.client.get("/auth/avatar", headers=headers).json()["content_type"], "image/png")

        r = self.client.delete("/auth/avatar", headers=headers)
        self.assertEqual(r.status_code, 200)
        self.assertEqual(r.json()["avatar"], "google")
        self.assertEqual(self.client.get("/auth/avatar", headers=headers).status_code, 404)

    def test_rejects_non_images(self):
        headers = self.make_user()
        for image in (b64(b"GIF89a" + b"\x00" * 10), "not base64!!", b64(b"<svg/>")):
            r = self.client.post("/auth/avatar", headers=headers, json={"image": image})
            self.assertEqual(r.status_code, 400, image)
        self.assertEqual(self.me(headers)["avatar"], "default")

    def test_rejects_large_images(self):
        headers = self.make_user()
        r = self.client.post("/auth/avatar", headers=headers, json={"image": b64(JPEG + b"\x00" * 512 * 1024)})
        self.assertIn(r.status_code, (413, 422))
        self.assertEqual(self.me(headers)["avatar"], "default")

    def test_requires_login(self):
        self.assertEqual(self.client.get("/auth/avatar").status_code, 401)
        self.assertEqual(self.client.post("/auth/avatar", json={"image": b64(JPEG)}).status_code, 401)
        self.assertEqual(self.client.delete("/auth/avatar").status_code, 401)


if __name__ == "__main__":
    unittest.main()
