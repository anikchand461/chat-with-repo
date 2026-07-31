from pathlib import Path
from uuid import uuid4
import traceback
from datetime import date, datetime

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, Field
from sqlalchemy.orm import Session

from backend.database import Chat, User, Message, DailyUsage
from backend.database.session import get_db
from backend.auth import get_current_user

router = APIRouter(prefix="/chat", tags=["chat"])

FREE_HISTORY_LIMIT = 20
FREE_CHAT_LIMIT = 2


class CreateChatRequest(BaseModel):
    owner: str = Field(min_length=1, max_length=100)
    repo: str = Field(min_length=1, max_length=200)
    branch: str = Field(default="main", min_length=1, max_length=200)


class AskRequest(BaseModel):
    question: str


@router.post("/create", status_code=201)
def create_chat(
    req: CreateChatRequest,
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
        if current_user.plan == "FREE":
            return {
                "chat_id": existing_chat.id,
                "title": existing_chat.title,
                "owner": existing_chat.owner,
                "repo": existing_chat.repo,
                "branch": existing_chat.branch,
                "already_indexed": True,
            }

        return {
            "chat_id": existing_chat.id,
            "title": existing_chat.title,
            "owner": existing_chat.owner,
            "repo": existing_chat.repo,
            "branch": existing_chat.branch,
            "already_indexed": True,
            "can_reindex": True,
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

    try:
        from backend.api.routes import analyze_branch
        from backend.rag.pipeline import RAGPipeline

        analyze_branch(
            req.owner,
            req.repo,
            branch,
            github_token=current_user.github_token,
        )

        json_path = Path("data") / f"{req.owner}_{req.repo}_{branch}.json"

        rag = RAGPipeline(
            str(json_path),
            collection_name=collection_name,
            persist_directory=str(Path("chroma_db") / "chats" / chat_key),
        )
        rag.build_index()

        if current_user.plan == "FREE":
            current_user.used_repo_count += 1
            db.commit()

    except Exception as e:
        traceback.print_exc()
        db.delete(chat)
        db.commit()
        raise HTTPException(status_code=500, detail=str(e))

    return {
        "chat_id": chat.id,
        "title": chat.title,
        "owner": chat.owner,
        "repo": chat.repo,
        "branch": chat.branch,
        "collection_name": collection_name,
    }


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


@router.post("/{chat_id}/ask")
def ask(
    chat_id: int,
    req: AskRequest,
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

    from backend.rag.pipeline import RAGPipeline

    json_path = f"data/{chat.owner}_{chat.repo}_{chat.branch}.json"

    rag = RAGPipeline(
        json_path,
        collection_name=chat.collection_name,
        persist_directory=str(
            Path("chroma_db") / "chats" / chat.collection_name.split("_", 2)[-1]
        ),
    )

    messages = (
        db.query(Message)
        .filter(Message.chat_id == chat.id)
        .order_by(Message.id)
        .all()
    )

    history = [{"role": m.role, "content": m.content} for m in messages]

    if current_user.plan == "FREE":
        history = history[-FREE_HISTORY_LIMIT:]

    answer = rag.ask(question=req.question, history=history)

    db.add(Message(chat_id=chat.id, role="user", content=req.question))
    db.add(Message(chat_id=chat.id, role="assistant", content=answer))

    if current_user.plan == "FREE":
        usage.questions_used += 1

    db.commit()

    return {"answer": answer}


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

    return [{"role": m.role, "content": m.content} for m in messages]