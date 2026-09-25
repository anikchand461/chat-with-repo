import json
import time
from pathlib import Path
from uuid import uuid4
import traceback
from datetime import date, datetime

from fastapi import APIRouter, BackgroundTasks, Depends, HTTPException
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field
from sqlalchemy.orm import Session

from backend.database import Chat, User, Message, DailyUsage
from backend.database.db import SessionLocal
from backend.database.session import get_db
from backend.auth import get_current_user
from backend.config import DATA_DIR, CHROMA_DIR

router = APIRouter(prefix="/chat", tags=["chat"])

FREE_HISTORY_LIMIT = 6
HISTORY_LIMIT = 6
FREE_CHAT_LIMIT = 2


class CreateChatRequest(BaseModel):
    owner: str = Field(min_length=1, max_length=100)
    repo: str = Field(min_length=1, max_length=200)
    branch: str = Field(default="main", min_length=1, max_length=200)


class AskRequest(BaseModel):
    question: str


def _index_new_chat(chat_id, owner, repo, branch, github_token, collection_name, user_id, is_free):
    """
    Runs in the background after /chat/create returns, so the request
    doesn't block for however long indexing takes. Progress is published
    to rag.progress for the frontend to poll via /chat/{id}/index-status.
    """

    from backend.api.routes import analyze_branch
    from backend.rag.pipeline import get_pipeline
    from backend.rag import progress

    def on_stage(stage, detail=None, done=None, total=None):
        progress.set_stage(chat_id, stage, detail, done=done, total=total)

    try:
        on_stage("fetching")

        analyze_branch(
            owner,
            repo,
            branch,
            github_token=github_token,
        )

        json_path = DATA_DIR / f"{owner}_{repo}_{branch}.json"

        rag = get_pipeline(
            str(json_path),
            collection_name,
            str(CHROMA_DIR / "chats" / collection_name.split("_", 2)[-1]),
        )

        # Same lock ensure_index() uses (the /ask fallback path) - belt and
        # suspenders alongside progress.claim() above, so build_index() can
        # never run twice concurrently for the same pipeline no matter which
        # path triggered it.
        with rag._index_lock:
            rag.build_index(on_stage=on_stage)

        progress.set_ready(chat_id)

        if is_free:
            with SessionLocal() as session:
                user = session.query(User).filter(User.id == user_id).first()
                if user:
                    user.used_repo_count += 1
                    session.commit()

    except Exception as e:
        traceback.print_exc()
        progress.set_error(chat_id, str(e))


@router.post("/create", status_code=201)
def create_chat(
    req: CreateChatRequest,
    background_tasks: BackgroundTasks,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db),
):
    branch = req.branch or "main"

    current_month = datetime.now().strftime("%Y-%m")

    if current_user.repo_reset_month != current_month:
        current_user.used_repo_count = 0
        current_user.repo_reset_month = current_month
        db.commit()
        db.refresh(current_user)

    print("=" * 50)
    print("Plan:", repr(current_user.plan))
    print("Used:", current_user.used_repo_count)
    print("Limit:", current_user.monthly_repo_limit)
    print("TOKEN ON USER:", bool(current_user.github_token))
    print("=" * 50)

    from backend.rag import progress

    # Same owner/repo/branch → reopen existing chat (does not use a new slot)
    existing_chat = (
        db.query(Chat)
        .filter(
            Chat.user_id == current_user.id,
            Chat.owner == req.owner,
            Chat.repo == req.repo,
            Chat.branch == branch,
        )
        .first()
    )

    if existing_chat:
        # If a background index for this chat is already running (e.g. the
        # user re-clicked "create" after a dropped connection, while the
        # first attempt is still indexing in the background), don't decide
        # readiness from a live vector count - vector_store.count() > 0
        # goes true after the very first batch, long before indexing is
        # actually done, which would wrongly fast-path into an incomplete
        # chat. Defer to the tracked job's real status instead, and don't
        # start a second, redundant indexing run on top of it.
        if progress.is_tracked(existing_chat.id):
            tracked = progress.get(existing_chat.id)

            if tracked["status"] == "ready":
                base = {
                    "chat_id": existing_chat.id,
                    "title": existing_chat.title,
                    "owner": existing_chat.owner,
                    "repo": existing_chat.repo,
                    "branch": existing_chat.branch,
                    "already_indexed": True,
                }
                if current_user.plan != "FREE":
                    base["can_reindex"] = True
                return base

            if tracked["status"] == "error":
                # A previous attempt finished (failed) - e.g. it hit
                # GitHub's rate limit before a token was configured. This is
                # the user coming back to retry, most likely with a token
                # saved now, so actually retry rather than replaying the
                # same stale error forever. claim() is still atomic, so if
                # something else claims it in the same instant, this backs
                # off instead of scheduling a duplicate job.
                if progress.claim(existing_chat.id):
                    background_tasks.add_task(
                        _index_new_chat,
                        existing_chat.id,
                        existing_chat.owner,
                        existing_chat.repo,
                        existing_chat.branch,
                        current_user.github_token,
                        existing_chat.collection_name,
                        current_user.id,
                        False,
                    )
                return {
                    "chat_id": existing_chat.id,
                    "title": existing_chat.title,
                    "owner": existing_chat.owner,
                    "repo": existing_chat.repo,
                    "branch": existing_chat.branch,
                    "indexing": True,
                }

            # Still actively working - report status without scheduling a
            # second job on top of it.
            return {
                "chat_id": existing_chat.id,
                "title": existing_chat.title,
                "owner": existing_chat.owner,
                "repo": existing_chat.repo,
                "branch": existing_chat.branch,
                "indexing": True,
            }

        from backend.rag.pipeline import get_pipeline

        rag = get_pipeline(
            str(DATA_DIR / f"{existing_chat.owner}_{existing_chat.repo}_{existing_chat.branch}.json"),
            existing_chat.collection_name,
            str(CHROMA_DIR / "chats" / existing_chat.collection_name.split("_", 2)[-1]),
        )

        if rag.is_indexed():
            # Nothing tracked this chat in this process, and it genuinely
            # has vectors - treat as ready. (Still imperfect for a chat that
            # was left half-indexed by a crash in an *earlier* process, since
            # there's no durable "fully indexed" marker - see is_indexed().)
            base = {
                "chat_id": existing_chat.id,
                "title": existing_chat.title,
                "owner": existing_chat.owner,
                "repo": existing_chat.repo,
                "branch": existing_chat.branch,
                "already_indexed": True,
            }
            if current_user.plan != "FREE":
                base["can_reindex"] = True
            return base

        # The chat exists but its index doesn't (e.g. the vector store was
        # cleared, or a previous indexing attempt failed) - (re)index it the
        # same way as a brand new chat, so the frontend shows the same
        # progress panel and only opens the chat once it's actually ready.
        #
        # claim() is atomic - if a concurrent request (e.g. this same form's
        # own automatic retry, racing a first attempt that's still in
        # flight) already claimed this chat between the is_tracked() check
        # above and here, this returns False and we must NOT schedule a
        # second job. Two background jobs rebuilding the same Qdrant
        # collection at once is exactly what caused the write timeouts /
        # crash seen indexing chat-with-repo (2752 chunks, two upsert
        # streams hammering the same free-tier cluster).
        if progress.claim(existing_chat.id):
            background_tasks.add_task(
                _index_new_chat,
                existing_chat.id,
                existing_chat.owner,
                existing_chat.repo,
                existing_chat.branch,
                current_user.github_token,
                existing_chat.collection_name,
                current_user.id,
                False,  # reopening an existing chat never uses a new FREE slot
            )

        return {
            "chat_id": existing_chat.id,
            "title": existing_chat.title,
            "owner": existing_chat.owner,
            "repo": existing_chat.repo,
            "branch": existing_chat.branch,
            "indexing": True,
        }

    # FREE: hard max of 2 chats total
    if current_user.plan == "FREE":
        total_chats = (
            db.query(Chat)
            .filter(Chat.user_id == current_user.id)
            .count()
        )
        if total_chats >= FREE_CHAT_LIMIT:
            return {
                "upgrade_required": True,
                "reason": "repo_limit",
                "message": "You've reached the free limit of 2 repository chats. Upgrade to Pro for unlimited chats.",
            }

    chat_key = uuid4().hex
    collection_name = f"chat_{current_user.id}_{chat_key}"

    chat = Chat(
        user_id=current_user.id,
        title=f"{req.owner}/{req.repo}",
        owner=req.owner,
        repo=req.repo,
        branch=branch,
        collection_name=collection_name,
    )

    db.add(chat)
    db.commit()
    db.refresh(chat)

    welcome_message = f"""
👋 Welcome to **ChatWithRepo**!

I'm here to help you understand, navigate, and contribute to the **{chat.owner}/{chat.repo}** repository.

You can ask me things like:

• Explain the project architecture.
• Where is this feature implemented?
• How does this workflow work?
• Help me contribute to this repository.
• Summarize the project.
"""

    db.add(
        Message(
            chat_id=chat.id,
            role="assistant",
            content=welcome_message,
        )
    )
    db.commit()

    # chat.id is a brand new row nobody else knows about yet, so claim()
    # will always succeed here - used for consistency with the other two
    # scheduling sites, which do rely on it being atomic.
    if progress.claim(chat.id):
        background_tasks.add_task(
            _index_new_chat,
            chat.id,
            req.owner,
            req.repo,
            branch,
            current_user.github_token,
            collection_name,
            current_user.id,
            current_user.plan == "FREE",
        )

    return {
        "chat_id": chat.id,
        "title": chat.title,
        "owner": chat.owner,
        "repo": chat.repo,
        "branch": chat.branch,
        "collection_name": collection_name,
        "indexing": True,
    }


@router.get("/{chat_id}/index-status")
def index_status(
    chat_id: int,
    background_tasks: BackgroundTasks,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db),
):
    chat = (
        db.query(Chat)
        .filter(Chat.id == chat_id, Chat.user_id == current_user.id)
        .first()
    )

    if not chat:
        raise HTTPException(404, "Chat not found")

    from backend.rag import progress

    tracked = progress.get(chat_id) if progress.is_tracked(chat_id) else None

    # IMPORTANT: this is a passive GET, polled automatically every ~1.5s by
    # the dashboard while a chat is indexing, plus once by chat.html's own
    # guard. It must never *retry* a failed job on its own - claim() would
    # immediately overwrite "error" back to "working" before the response
    # below even reads it, so the caller would never actually observe the
    # error at all. With no token configured, every auto-retry fails almost
    # instantly on the first GitHub call, so the very next poll (1.5s later,
    # sometimes sooner) would see "error" and retry again - forever, with
    # the error never surfacing. (This used to live here; it was the actual
    # cause of the indexing spinner that never stopped.) A failed chat is
    # only ever retried by a deliberate action - see chat.html's guard,
    # which calls POST /chat/create once when it sees "error" - not by
    # simply checking status.
    if tracked is None:
        # Nothing in this process has indexed (or attempted to index) this
        # chat yet - e.g. it's from an earlier server run, or its vectors
        # were cleared. Check for real, and self-heal if it's not actually
        # ready, instead of reporting a stale "ready" and only discovering
        # the problem later when a question gets asked inside the chat page.
        from backend.rag.pipeline import get_pipeline

        rag = get_pipeline(
            str(DATA_DIR / f"{chat.owner}_{chat.repo}_{chat.branch}.json"),
            chat.collection_name,
            str(CHROMA_DIR / "chats" / chat.collection_name.split("_", 2)[-1]),
        )

        if not rag.is_indexed() and progress.claim(chat_id):
            # claim() is atomic - guards against a concurrent poll (or the
            # create-modal's own retry) racing this same self-heal check
            # and scheduling a second indexing job for the same chat.
            background_tasks.add_task(
                _index_new_chat,
                chat.id,
                chat.owner,
                chat.repo,
                chat.branch,
                current_user.github_token,
                chat.collection_name,
                current_user.id,
                False,
            )

    return progress.get(chat_id)


@router.get("/list")
def list_chats(
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db),
):
    chats = (
        db.query(Chat)
        .filter(Chat.user_id == current_user.id)
        .order_by(Chat.created_at.desc())
        .all()
    )

    return [
        {
            "chat_id": chat.id,
            "title": chat.title,
            "owner": chat.owner,
            "repo": chat.repo,
            "branch": chat.branch,
        }
        for chat in chats
    ]


def _prepare_ask(chat_id: int, current_user: User, db: Session):
    """
    Shared setup for /ask and /ask/stream.

    Returns (chat, usage, rag, history) or an upgrade_required dict.
    """

    chat = (
        db.query(Chat)
        .filter(Chat.id == chat_id, Chat.user_id == current_user.id)
        .first()
    )

    if not chat:
        raise HTTPException(404, "Chat not found")

    today = date.today()

    usage = (
        db.query(DailyUsage)
        .filter(
            DailyUsage.user_id == current_user.id,
            DailyUsage.date == today,
        )
        .first()
    )

    if usage is None:
        usage = DailyUsage(
            user_id=current_user.id,
            date=today,
            questions_used=0,
        )
        db.add(usage)
        db.commit()
        db.refresh(usage)

    if current_user.plan == "FREE" and usage.questions_used >= 10:
        return {
            "upgrade_required": True,
            "reason": "daily_questions",
            "message": "You have reached today's free question limit.",
        }

    from backend.rag.pipeline import get_pipeline
    from backend.rag import progress

    rag = get_pipeline(
        str(DATA_DIR / f"{chat.owner}_{chat.repo}_{chat.branch}.json"),
        chat.collection_name,
        str(CHROMA_DIR / "chats" / chat.collection_name.split("_", 2)[-1]),
    )

    def fetch_repository():
        from backend.api.routes import analyze_branch

        analyze_branch(
            chat.owner,
            chat.repo,
            chat.branch,
            github_token=current_user.github_token,
        )

    # Same short predefined stages as chat creation (rag.progress.STAGES),
    # so the frontend can show the same "indexing X/Y" message here too -
    # this path runs whenever a chat's index needs rebuilding after the
    # chat already exists (e.g. the vector store was cleared).
    def on_stage(stage, detail=None, done=None, total=None):
        progress.set_stage(chat_id, stage, detail, done=done, total=total)

    try:
        rag.ensure_index(fetch_repository, on_stage=on_stage)
        progress.set_ready(chat_id)
    except Exception as e:
        traceback.print_exc()
        progress.set_error(chat_id, str(e))
        raise HTTPException(
            status_code=500,
            detail=f"Could not prepare this repository for questions: {e}",
        )

    messages = (
        db.query(Message)
        .filter(Message.chat_id == chat.id)
        .order_by(Message.id)
        .all()
    )

    # Skip the assistant welcome message(s) before the first user turn.
    first_user = next(
        (i for i, m in enumerate(messages) if m.role == "user"),
        len(messages),
    )

    history = [
        {"role": m.role, "content": m.content}
        for m in messages[first_user:]
    ]

    limit = FREE_HISTORY_LIMIT if current_user.plan == "FREE" else HISTORY_LIMIT
    history = history[-limit:]

    return chat, usage, rag, history


@router.post("/{chat_id}/ask")
def ask(
    chat_id: int,
    req: AskRequest,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db),
):
    started = time.perf_counter()

    prepared = _prepare_ask(chat_id, current_user, db)

    if isinstance(prepared, dict):
        return prepared

    chat, usage, rag, history = prepared

    answer = rag.ask(question=req.question, history=history)
    total = time.perf_counter() - started

    db.add(Message(chat_id=chat.id, role="user", content=req.question))
    db.add(
        Message(
            chat_id=chat.id,
            role="assistant",
            content=answer,
            response_seconds=total,
        )
    )

    if current_user.plan == "FREE":
        usage.questions_used += 1

    db.commit()

    return {"answer": answer, "response_seconds": total}


@router.post("/{chat_id}/ask/stream")
def ask_stream(
    chat_id: int,
    req: AskRequest,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db),
):
    """
    Server-sent events version of /ask.

    Events (each `data:` line is JSON):
      {"token": "..."}  - a piece of the answer
      {"done": true, "total": s, "first": s} - the answer has been saved;
                        total/first are the server-measured response times
      {"error": "..."}  - generation failed (nothing is saved)
    If the daily limit is hit, a plain JSON upgrade_required body is returned.
    """

    started = time.perf_counter()

    prepared = _prepare_ask(chat_id, current_user, db)

    if isinstance(prepared, dict):
        return prepared

    chat, _usage, rag, history = prepared

    chat_pk = chat.id
    user_pk = current_user.id
    is_free = current_user.plan == "FREE"
    question = req.question

    def sse(payload):
        return f"data: {json.dumps(payload)}\n\n"

    def event_stream():
        parts = []
        sources = []
        first_word = None

        try:
            for event in rag.ask_stream(question=question, history=history):
                if event["type"] == "sources":
                    sources = event["files"]
                    if sources:
                        yield sse({"sources": sources})
                    continue

                text = event["text"]
                if first_word is None:
                    first_word = time.perf_counter() - started
                parts.append(text)
                yield sse({"token": text})
        except Exception as e:
            traceback.print_exc()
            yield sse({"error": str(e)})
            return

        total = time.perf_counter() - started

        # The request-scoped session may already be closed while streaming,
        # so persist the turn with a fresh one.
        with SessionLocal() as session:
            session.add(Message(chat_id=chat_pk, role="user", content=question))
            session.add(
                Message(
                    chat_id=chat_pk,
                    role="assistant",
                    content="".join(parts),
                    response_seconds=total,
                    first_word_seconds=first_word,
                    sources=sources or None,
                )
            )

            if is_free:
                row = (
                    session.query(DailyUsage)
                    .filter(
                        DailyUsage.user_id == user_pk,
                        DailyUsage.date == date.today(),
                    )
                    .first()
                )
                if row:
                    row.questions_used += 1

            session.commit()

        yield sse({"done": True, "total": total, "first": first_word})

    return StreamingResponse(
        event_stream(),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )


@router.get("/{chat_id}/messages")
def get_messages(
    chat_id: int,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db),
):
    chat = (
        db.query(Chat)
        .filter(Chat.id == chat_id, Chat.user_id == current_user.id)
        .first()
    )

    if not chat:
        raise HTTPException(404, "Chat not found")

    messages = (
        db.query(Message)
        .filter(Message.chat_id == chat.id)
        .order_by(Message.created_at)
        .all()
    )

    return [
        {
            "role": m.role,
            "content": m.content,
            "response_seconds": m.response_seconds,
            "first_word_seconds": m.first_word_seconds,
            "sources": m.sources,
        }
        for m in messages
    ]