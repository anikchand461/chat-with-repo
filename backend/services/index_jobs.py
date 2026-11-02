"""
Start a repository indexing job (backend.api.chat._index_new_chat).

By default it runs as a FastAPI BackgroundTask in this process, as it
always has (local development, Render). With INDEX_RUNNER=ecs (AWS) it
runs as a one-off Fargate task from the same container image instead
(backend/jobs/index_chat.py): a Lambda instance is frozen once its response
is sent, and indexing a large repository can outlast Lambda's 15 minutes.

The caller must already have claimed the chat (rag.progress.claim). The
Fargate task gets no secrets on its command line - it looks up the user's
GitHub token in the database and reads everything else from its task
definition (template.yaml).
"""

import json
import os

_ecs = None


def runs_remotely():
    """True when jobs run outside this process (so they can't share its memory)."""

    return os.getenv("INDEX_RUNNER") == "ecs"


def start(background_tasks, chat_id, owner, repo, branch, github_token, collection_name, user_id, is_free):
    if not runs_remotely():
        from backend.api.chat import _index_new_chat

        background_tasks.add_task(
            _index_new_chat,
            chat_id,
            owner,
            repo,
            branch,
            github_token,
            collection_name,
            user_id,
            is_free,
        )
        return

    job = {
        "chat_id": chat_id,
        "owner": owner,
        "repo": repo,
        "branch": branch,
        "collection_name": collection_name,
        "user_id": user_id,
        "is_free": is_free,
    }

    try:
        _run_task(job)
    except Exception as e:
        from backend.rag import progress

        print(f"Could not start indexing task for chat {chat_id}: {type(e).__name__}: {e}")
        progress.set_error(chat_id, "Could not start indexing. Please try again.")


def _run_task(job):
    global _ecs
    if _ecs is None:
        import boto3

        _ecs = boto3.client("ecs")

    response = _ecs.run_task(
        cluster=os.environ["INDEX_CLUSTER"],
        taskDefinition=os.environ["INDEX_TASK_DEFINITION"],
        launchType="FARGATE",
        count=1,
        startedBy=f"chat-{job['chat_id']}",
        networkConfiguration={
            "awsvpcConfiguration": {
                "subnets": os.environ["INDEX_SUBNETS"].split(","),
                "securityGroups": [os.environ["INDEX_SECURITY_GROUP"]],
                # Public subnets without a NAT gateway: the task needs a public
                # IP to reach GitHub, Cohere, Qdrant, Neon and S3.
                "assignPublicIp": "ENABLED",
            }
        },
        overrides={
            "containerOverrides": [
                {
                    "name": "indexer",
                    "command": ["python", "-m", "backend.jobs.index_chat", json.dumps(job)],
                }
            ]
        },
    )

    if response.get("failures") or not response.get("tasks"):
        raise RuntimeError(f"run_task failed: {response.get('failures')}")
