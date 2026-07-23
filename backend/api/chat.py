from pathlib import Path

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, Field
from database import Chat, User
from database.session import get_db
from auth import get_current_user
from sqlalchemy.orm import Session
from uuid import uuid4

router = APIRouter(prefix="/chat", tags=["chat"])

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
    chat_key = uuid4().hex
    collection_name = f"chat_{current_user.id}_{chat_key}"
    chat = Chat(
        user_id=current_user.id,
        title=f"{req.owner}/{req.repo}",
        owner=req.owner,
        repo=req.repo,
        branch=branch,
        collection_name=collection_name
    )
    db.add(chat)
    db.commit()
    db.refresh(chat)

    try:
        from rag.pipeline import RAGPipeline
        from api.routes import analyze_branch
        analyze_branch(req.owner, req.repo, branch)
        json_path = Path("data") / f"{req.owner}_{req.repo}_{branch}.json"
        rag = RAGPipeline(
            str(json_path),
            collection_name=collection_name,
            persist_directory=str(Path("chroma_db") / "chats" / chat_key),
        )
        rag.build_index()
    except Exception as exc:
        db.delete(chat)
        db.commit()
        raise HTTPException(status_code=502, detail=f"Repository indexing failed: {exc}")

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
    chats = db.query(Chat).filter(Chat.user_id == current_user.id).order_by(Chat.created_at.desc()).all()
    return [{"chat_id": chat.id, "title": chat.title, "owner": chat.owner, "repo": chat.repo, "branch": chat.branch} for chat in chats]

@router.post("/{chat_id}/ask")
def ask(chat_id: int, req: AskRequest, current_user: User = Depends(get_current_user), db: Session = Depends(get_db)):
    chat = db.query(Chat).filter(Chat.id == chat_id, Chat.user_id == current_user.id).first()
    if not chat:
        raise HTTPException(404, "Chat not found")

    json_path = f"data/{chat.owner}_{chat.repo}_{chat.branch}.json"
    from rag.pipeline import RAGPipeline
    rag = RAGPipeline(
        json_path,
        collection_name=chat.collection_name,
        persist_directory=str(Path("chroma_db") / "chats" / chat.collection_name),
    )
    answer = rag.ask(req.question)
    return {"answer": answer}