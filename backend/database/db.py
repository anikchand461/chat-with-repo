from datetime import datetime, date
import os

from dotenv import load_dotenv
from sqlalchemy import (
    create_engine,
    Column,
    Integer,
    String,
    ForeignKey,
    DateTime,
    Date,
    Float,
    JSON,
    LargeBinary,
    inspect,
    text,
)
from sqlalchemy.dialects.postgresql import JSONB
from sqlalchemy.orm import declarative_base, deferred, sessionmaker, relationship

load_dotenv()

DATABASE_URL = os.getenv("DATABASE_URL")
print("dtatbase started")

engine = create_engine(
    DATABASE_URL,
    pool_pre_ping=True,
)

SessionLocal = sessionmaker(
    autocommit=False,
    autoflush=False,
    bind=engine,
)

Base = declarative_base()

class User(Base):
    __tablename__ = "users"

    id = Column(Integer, primary_key=True, index=True)
    email = Column(String, unique=True, index=True)
    hashed_password = Column(String, nullable=True)  # None for Google-only accounts
    github_token = Column(String, nullable=True)

    # Google sign-in (web). google_sub is Google's stable account id.
    google_sub = Column(String, unique=True, index=True, nullable=True)
    google_name = Column(String, nullable=True)
    google_picture = Column(String, nullable=True)

    # Profile picture the user uploaded (JPEG/PNG bytes). When empty the apps
    # show google_picture, else a default avatar. Deferred so the bytes are
    # only read by GET /auth/avatar, not on every authenticated request.
    avatar_data = deferred(Column(LargeBinary, nullable=True))
    avatar_type = Column(String, nullable=True)  # image/jpeg or image/png

    # Subscription
    plan = Column(String, default="FREE")
    dodo_customer_id = Column(String, nullable=True)
    dodo_subscription_id = Column(String, nullable=True)

    # Repository Limits
    monthly_repo_limit = Column(Integer, default=2)
    used_repo_count = Column(Integer, default=0)
    repo_reset_month = Column(String, default="")

    chats = relationship("Chat", back_populates="user")

class Chat(Base):
    __tablename__ = "chats"
    id = Column(Integer, primary_key=True, index=True)
    user_id = Column(Integer, ForeignKey("users.id"))
    title = Column(String)  # e.g. "shreyaghorui222004/DevLens"
    owner = Column(String)
    repo = Column(String)
    branch = Column(String, default="main")
    collection_name = Column(String, unique=True)
    created_at = Column(DateTime, default=datetime.utcnow)
    user = relationship("User", back_populates="chats")

class Message(Base):
    __tablename__ = "messages"

    id = Column(Integer, primary_key=True)
    chat_id = Column(Integer, ForeignKey("chats.id"))
    role = Column(String)          # user / assistant
    content = Column(String)
    created_at = Column(DateTime, default=datetime.utcnow)

    # How long the assistant took to answer (assistant messages only), so the
    # web and mobile apps show the same timing.
    response_seconds = Column(Float, nullable=True)
    first_word_seconds = Column(Float, nullable=True)

    # The real repository file paths retrieval used for this answer (assistant
    # messages only, streamed answers only - see backend/api/chat.py's
    # ask_stream). Persisted so the "which files" chips shown while an answer
    # is live also show up consistently when the chat is reopened, on the
    # same device or a different one - both read the same row.
    sources = Column(JSON, nullable=True)

    chat = relationship("Chat")

class DailyUsage(Base):
    __tablename__ = "daily_usage"

    id = Column(Integer, primary_key=True)
    user_id = Column(Integer, ForeignKey("users.id"), nullable=False)
    date = Column(Date, default=date.today, nullable=False)
    questions_used = Column(Integer, default=0)

class IndexProgress(Base):
    """
    Indexing progress per chat, shared by every backend instance. Only used
    with PROGRESS_STORE=db (AWS) - see backend/rag/progress_db.py.
    """

    __tablename__ = "index_progress"

    chat_id = Column(Integer, primary_key=True)
    status = Column(String, nullable=False)   # working / ready / error
    stage = Column(String, nullable=False)
    message = Column(String, nullable=False)
    done = Column(Integer, nullable=True)
    total = Column(Integer, nullable=True)
    updated_at = Column(DateTime, nullable=False)

class RepositoryJson(Base):
    """
    Copy of each downloaded repository JSON file (DATA_DIR), so it survives
    Render restarts - see backend/storage/repo_store.py.
    """

    __tablename__ = "repository_json"

    name = Column(String, primary_key=True)   # file name, e.g. octocat_Spoon-Knife_main.json
    data = Column(JSON().with_variant(JSONB, "postgresql"), nullable=False)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow)


Base.metadata.create_all(bind=engine)


def _add_missing_columns():
    """
    create_all() never alters existing tables, so add columns introduced
    after a database already existed here. Safe to run any number of times,
    including from several backends at once (AWS + Render), and it never
    drops or rewrites data. Returns the statements it ran.
    """

    postgres = engine.dialect.name == "postgresql"
    if_not_exists = "IF NOT EXISTS " if postgres else ""

    columns_to_add = {
        "messages": {
            "response_seconds": "DOUBLE PRECISION",
            "first_word_seconds": "DOUBLE PRECISION",
            "sources": "JSON",
        },
        "users": {
            "google_sub": "VARCHAR",
            "google_name": "VARCHAR",
            "google_picture": "VARCHAR",
            "avatar_data": "BYTEA" if postgres else "BLOB",
            "avatar_type": "VARCHAR",
        },
    }

    applied = []
    with engine.begin() as conn:
        for table, columns in columns_to_add.items():
            existing = {c["name"] for c in inspect(conn).get_columns(table)}
            for name, sql_type in columns.items():
                if name not in existing:
                    statement = f"ALTER TABLE {table} ADD COLUMN {if_not_exists}{name} {sql_type}"
                    conn.execute(text(statement))
                    applied.append(statement)

        # Unique, so one Google account can only ever sign in to one user.
        # Same name create_all() gives a fresh database; NULLs don't collide.
        indexes = {i["name"] for i in inspect(conn).get_indexes("users")}
        if "ix_users_google_sub" not in indexes:
            statement = "CREATE UNIQUE INDEX IF NOT EXISTS ix_users_google_sub ON users (google_sub)"
            conn.execute(text(statement))
            applied.append(statement)

        # Google-only users have no password. Older databases may have made
        # the column NOT NULL; relaxing it keeps every existing value.
        if postgres:
            password = next(
                c for c in inspect(conn).get_columns("users") if c["name"] == "hashed_password"
            )
            if not password["nullable"]:
                statement = "ALTER TABLE users ALTER COLUMN hashed_password DROP NOT NULL"
                conn.execute(text(statement))
                applied.append(statement)

    return applied


MIGRATIONS_APPLIED = _add_missing_columns()
