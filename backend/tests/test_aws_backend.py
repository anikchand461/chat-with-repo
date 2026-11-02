"""
Tests for the AWS container backend pieces: the Postgres progress store,
the indexing job launcher (BackgroundTasks vs Fargate), the S3 repo cache,
the Fargate job entry point, and the /chat endpoints in both modes.

Runs against a throwaway SQLite database and fake AWS clients - no
network, no real database:

    .venv/bin/python -m unittest backend.tests.test_aws_backend -v
"""

import json
import os
import tempfile
import threading
import time
import unittest
from datetime import timedelta
from pathlib import Path
from unittest import mock

_TMP = tempfile.mkdtemp(prefix="cwr-test-")
os.environ["DATABASE_URL"] = f"sqlite:///{_TMP}/test.db"
os.environ["DATA_DIR"] = f"{_TMP}/data"
os.environ["JWT_SECRET"] = "test-secret"

from backend.database import db as _db  # noqa: E402  (creates the SQLite tables)

if _db.engine.url.get_backend_name() != "sqlite":
    raise RuntimeError("Refusing to run: tests must use the throwaway SQLite database")

from fastapi import BackgroundTasks  # noqa: E402
from fastapi.testclient import TestClient  # noqa: E402

from backend.app import app  # noqa: E402
from backend.auth import create_access_token  # noqa: E402
from backend.database import Chat, SessionLocal, User  # noqa: E402
from backend.database.db import IndexProgress  # noqa: E402
from backend.rag import progress, progress_db  # noqa: E402
from backend.services import index_jobs  # noqa: E402
from backend.storage import repo_cache  # noqa: E402
from backend.storage.json_writer import save_json  # noqa: E402

AWS_ENV = {
    "PROGRESS_STORE": "db",
    "INDEX_RUNNER": "ecs",
    "INDEX_CLUSTER": "test-cluster",
    "INDEX_TASK_DEFINITION": "arn:aws:ecs:ap-south-1:1:task-definition/chatwithrepo-index:1",
    "INDEX_SUBNETS": "subnet-a,subnet-b",
    "INDEX_SECURITY_GROUP": "sg-1",
}

_next_id = iter(range(1000, 100000))


class FakeEcs:
    def __init__(self, fail=False):
        self.calls = []
        self.fail = fail

    def run_task(self, **kwargs):
        self.calls.append(kwargs)
        if self.fail:
            return {"tasks": [], "failures": [{"reason": "RESOURCE:CPU"}]}
        return {"tasks": [{"taskArn": "arn:task/1"}], "failures": []}


class FakeS3:
    def __init__(self):
        self.objects = {}

    def upload_file(self, filename, bucket, key):
        self.objects[(bucket, key)] = Path(filename).read_bytes()

    def download_file(self, bucket, key, filename):
        if (bucket, key) not in self.objects:
            error = Exception("Not Found")
            error.response = {"Error": {"Code": "404"}}
            raise error
        Path(filename).write_bytes(self.objects[(bucket, key)])


def _make_user(github_token="ghp_user_token", plan="PRO"):
    with SessionLocal() as session:
        user = User(
            email=f"user{next(_next_id)}@test.dev",
            hashed_password="x",
            github_token=github_token,
            plan=plan,
            monthly_repo_limit=100,
        )
        session.add(user)
        session.commit()
        return user.id


class ProgressDbStoreTest(unittest.TestCase):
    def setUp(self):
        self.env = mock.patch.dict(os.environ, {"PROGRESS_STORE": "db"})
        self.env.start()
        self.chat_id = next(_next_id)

    def tearDown(self):
        self.env.stop()

    def test_claim_is_exclusive_while_working(self):
        self.assertFalse(progress.is_tracked(self.chat_id))
        self.assertTrue(progress.claim(self.chat_id))
        self.assertFalse(progress.claim(self.chat_id))
        self.assertEqual(progress.get(self.chat_id)["stage"], "starting")

    def test_stages_ready_and_error(self):
        progress.claim(self.chat_id)
        progress.set_stage(self.chat_id, "indexing", "Indexing 5/10", done=5, total=10)
        self.assertEqual(
            progress.get(self.chat_id),
            {"status": "working", "stage": "indexing", "message": "Indexing 5/10", "done": 5, "total": 10},
        )

        progress.set_error(self.chat_id, "boom")
        self.assertEqual(progress.get(self.chat_id)["status"], "error")
        self.assertTrue(progress.claim(self.chat_id), "a failed chat can be retried")

        progress.set_ready(self.chat_id)
        self.assertEqual(progress.get(self.chat_id)["status"], "ready")
        self.assertNotIn("done", progress.get(self.chat_id))

    def test_untracked_chat_reads_as_ready(self):
        self.assertEqual(progress.get(self.chat_id)["status"], "ready")

    def test_stale_job_reports_error_and_can_be_reclaimed(self):
        progress.claim(self.chat_id)
        self._age(progress_db.STALE_AFTER + timedelta(minutes=1))

        self.assertEqual(progress.get(self.chat_id)["message"], progress_db.STALE_MESSAGE)
        self.assertTrue(progress.claim(self.chat_id))
        self.assertEqual(progress.get(self.chat_id)["status"], "working")

    def test_touch_keeps_a_running_job_fresh(self):
        progress.claim(self.chat_id)
        self._age(progress_db.STALE_AFTER + timedelta(minutes=1))
        progress.touch(self.chat_id)
        self.assertEqual(progress.get(self.chat_id)["status"], "working")
        self.assertFalse(progress.claim(self.chat_id))

    def test_concurrent_claims_only_one_wins(self):
        results = []
        barrier = threading.Barrier(8)

        def worker():
            barrier.wait()
            results.append(progress.claim(self.chat_id))

        threads = [threading.Thread(target=worker) for _ in range(8)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()

        self.assertEqual(results.count(True), 1, results)

    def test_clear(self):
        progress.claim(self.chat_id)
        progress.clear(self.chat_id)
        self.assertFalse(progress.is_tracked(self.chat_id))

    def _age(self, delta):
        with SessionLocal() as session:
            row = session.get(IndexProgress, self.chat_id)
            row.updated_at = row.updated_at - delta
            session.commit()


class InMemoryStoreUnchangedTest(unittest.TestCase):
    """Without PROGRESS_STORE (Render, local) nothing goes to the database."""

    def test_memory_store(self):
        chat_id = next(_next_id)
        with mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop("PROGRESS_STORE", None)
            self.assertTrue(progress.claim(chat_id))
            self.assertFalse(progress.claim(chat_id))
            progress.set_ready(chat_id)
            self.assertEqual(progress.get(chat_id)["status"], "ready")

        with SessionLocal() as session:
            self.assertIsNone(session.get(IndexProgress, chat_id))


class IndexJobsTest(unittest.TestCase):
    ARGS = (7, "octo", "hello", "main", "ghp_secret", "chat_1_abc", 3, True)

    def test_background_task_by_default(self):
        tasks = BackgroundTasks()
        with mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop("INDEX_RUNNER", None)
            index_jobs.start(tasks, *self.ARGS)

        from backend.api.chat import _index_new_chat

        self.assertEqual(len(tasks.tasks), 1)
        self.assertIs(tasks.tasks[0].func, _index_new_chat)
        self.assertEqual(tasks.tasks[0].args, self.ARGS)

    def test_fargate_task_on_aws(self):
        ecs = FakeEcs()
        with mock.patch.dict(os.environ, AWS_ENV), mock.patch.object(index_jobs, "_ecs", ecs):
            index_jobs.start(None, *self.ARGS)

        (call,) = ecs.calls
        self.assertEqual(call["launchType"], "FARGATE")
        self.assertEqual(call["cluster"], "test-cluster")
        network = call["networkConfiguration"]["awsvpcConfiguration"]
        self.assertEqual(network["subnets"], ["subnet-a", "subnet-b"])
        self.assertEqual(network["assignPublicIp"], "ENABLED")

        command = call["overrides"]["containerOverrides"][0]["command"]
        self.assertEqual(command[:3], ["python", "-m", "backend.jobs.index_chat"])
        job = json.loads(command[3])
        self.assertEqual(job["chat_id"], 7)
        self.assertNotIn("ghp_secret", json.dumps(call), "secrets must not be in the task request")

    def test_fargate_failure_is_recorded(self):
        chat_id = next(_next_id)
        with mock.patch.dict(os.environ, AWS_ENV), mock.patch.object(index_jobs, "_ecs", FakeEcs(fail=True)):
            progress.claim(chat_id)
            index_jobs.start(None, chat_id, *self.ARGS[1:])
            state = progress.get(chat_id)

        self.assertEqual(state["status"], "error")


class RepoCacheTest(unittest.TestCase):
    def test_save_uploads_and_missing_file_is_restored(self):
        s3 = FakeS3()
        path = Path(os.environ["DATA_DIR"]) / "octo_hello_main.json"

        with mock.patch.object(repo_cache, "BUCKET", "bucket"), mock.patch.object(repo_cache, "_client", s3):
            save_json({"name": "hello", "files": []}, str(path))
            self.assertIn(("bucket", "repos/octo_hello_main.json"), s3.objects)

            path.unlink()
            self.assertTrue(repo_cache.download(path))
            self.assertEqual(json.loads(path.read_text())["name"], "hello")

            self.assertFalse(repo_cache.download(path), "already on disk")
            self.assertFalse(repo_cache.download(path.with_name("missing.json")))

    def test_disabled_without_bucket(self):
        s3 = FakeS3()
        path = Path(os.environ["DATA_DIR"]) / "plain.json"

        with mock.patch.object(repo_cache, "BUCKET", None), mock.patch.object(repo_cache, "_client", s3):
            save_json({"name": "plain"}, str(path))

        self.assertEqual(s3.objects, {})


class FargateJobEntryPointTest(unittest.TestCase):
    def test_runs_index_with_token_from_db_and_heartbeat(self):
        from backend.jobs import index_chat

        user_id = _make_user(github_token="ghp_from_db")
        chat_id = next(_next_id)
        seen = {}

        def fake_index(chat_id_, owner, repo, branch, token, collection, user, is_free):
            seen.update(token=token, owner=owner, is_free=is_free)
            progress.set_stage(chat_id_, "indexing")
            with SessionLocal() as session:  # make the row look old...
                row = session.get(IndexProgress, chat_id_)
                row.updated_at -= timedelta(hours=1)
                session.commit()
            time.sleep(0.3)  # ...and let the heartbeat refresh it
            seen["fresh"] = progress.get(chat_id_)["status"] == "working"
            progress.set_ready(chat_id_)

        job = {
            "chat_id": chat_id, "owner": "octo", "repo": "hello", "branch": "main",
            "collection_name": "c", "user_id": user_id, "is_free": False,
        }

        with mock.patch.dict(os.environ, {"PROGRESS_STORE": "db"}), \
                mock.patch("backend.api.chat._index_new_chat", fake_index), \
                mock.patch.object(progress_db, "HEARTBEAT_SECONDS", 0.05):
            progress.claim(chat_id)
            code = index_chat.main(["index_chat", json.dumps(job)])

        self.assertEqual(code, 0)
        self.assertEqual(seen["token"], "ghp_from_db")
        self.assertTrue(seen["fresh"], "heartbeat should keep the job from looking stale")

    def test_usage(self):
        from backend.jobs import index_chat

        self.assertEqual(index_chat.main(["index_chat"]), 2)

    def test_fresh_process_can_import_the_indexing_code(self):
        # Regression: on Fargate the job is the only thing in the process -
        # nothing imports backend/app.py, which is what puts backend/ on
        # sys.path for the `rag.x` imports inside backend/rag/.
        import subprocess
        import sys

        repo_root = Path(__file__).resolve().parents[2]
        env = {k: v for k, v in os.environ.items() if k != "PYTHONPATH"}

        def run(code):
            return subprocess.run(
                [sys.executable, "-c", code],
                cwd=repo_root, env=env, capture_output=True, text=True, timeout=300,
            )

        # Without the job module the import really does fail, so the check
        # below is meaningful.
        without = run("import backend.rag.pipeline")
        self.assertIn("No module named 'rag'", without.stderr)

        result = run("import backend.jobs.index_chat; import backend.rag.pipeline")
        self.assertEqual(result.returncode, 0, result.stderr[-2000:])

    def test_failure_before_indexing_is_reported_immediately(self):
        from backend.jobs import index_chat

        chat_id = next(_next_id)
        job = {
            "chat_id": chat_id, "owner": "octo", "repo": "hello", "branch": "main",
            "collection_name": "c", "user_id": 1, "is_free": False,
        }

        with mock.patch.dict(os.environ, {"PROGRESS_STORE": "db"}), \
                mock.patch("backend.database.SessionLocal", side_effect=RuntimeError("db down")):
            progress.claim(chat_id)
            with self.assertRaises(RuntimeError):
                index_chat.main(["index_chat", json.dumps(job)])
            state = progress.get(chat_id)

        self.assertEqual(state["status"], "error")


class FakeRag:
    def __init__(self, indexed):
        self.indexed = indexed
        self.ensure_calls = 0

    def is_indexed(self):
        return self.indexed

    def ensure_index(self, fetch_repository, on_stage=None):
        self.ensure_calls += 1


class ChatApiTest(unittest.TestCase):
    def setUp(self):
        self.client = TestClient(app)
        self.user_id = _make_user()
        self.headers = {"Authorization": f"Bearer {create_access_token(self.user_id)}"}

    def _create(self, repo):
        return self.client.post("/chat/create", json={"owner": "octo", "repo": repo, "branch": "main"}, headers=self.headers)

    def test_aws_mode_create_poll_and_ask(self):
        ecs = FakeEcs()
        with mock.patch.dict(os.environ, AWS_ENV), mock.patch.object(index_jobs, "_ecs", ecs):
            first = self._create("aws-repo").json()
            self.assertTrue(first["indexing"])
            chat_id = first["chat_id"]
            self.assertEqual(len(ecs.calls), 1)

            # Re-clicking create while the job runs must not start another.
            again = self._create("aws-repo").json()
            self.assertEqual(again["chat_id"], chat_id)
            self.assertEqual(len(ecs.calls), 1)

            status = self.client.get(f"/chat/{chat_id}/index-status", headers=self.headers).json()
            self.assertEqual(status["stage"], "starting")

            # Asking while indexing: 409, never 502/503 (which would fail over to Render).
            rag = FakeRag(indexed=True)
            with mock.patch("backend.rag.pipeline.get_pipeline", return_value=rag):
                busy = self.client.post(f"/chat/{chat_id}/ask", json={"question": "hi"}, headers=self.headers)
            self.assertEqual(busy.status_code, 409)
            self.assertEqual(rag.ensure_calls, 0)

            # The Fargate job finishes (in another process).
            progress.set_ready(chat_id)
            status = self.client.get(f"/chat/{chat_id}/index-status", headers=self.headers).json()
            self.assertEqual(status["status"], "ready")

            done = self._create("aws-repo").json()
            self.assertTrue(done["already_indexed"])

            # Vectors gone later: ask starts one job instead of rebuilding inline.
            rag = FakeRag(indexed=False)
            with mock.patch("backend.rag.pipeline.get_pipeline", return_value=rag):
                rebuild = self.client.post(f"/chat/{chat_id}/ask", json={"question": "hi"}, headers=self.headers)
            self.assertEqual(rebuild.status_code, 409)
            self.assertEqual(rag.ensure_calls, 0)
            self.assertEqual(len(ecs.calls), 2)

    def test_render_mode_unchanged(self):
        calls = []

        def fake_index(*args):
            calls.append(args)
            progress.set_ready(args[0])

        with mock.patch.dict(os.environ, {}, clear=False), \
                mock.patch("backend.api.chat._index_new_chat", fake_index):
            for key in AWS_ENV:
                os.environ.pop(key, None)

            body = self._create("render-repo").json()  # TestClient runs background tasks before returning
            self.assertTrue(body["indexing"])
            self.assertEqual(len(calls), 1)
            self.assertEqual(calls[0][4], "ghp_user_token")

            status = self.client.get(f"/chat/{body['chat_id']}/index-status", headers=self.headers).json()
            self.assertEqual(status["status"], "ready")

        with SessionLocal() as session:
            self.assertIsNone(session.get(IndexProgress, body["chat_id"]), "Render mode must not use the DB store")


if __name__ == "__main__":
    unittest.main()
