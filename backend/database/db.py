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
    inspect,
    text,
)
from sqlalchemy.orm import declarative_base, sessionmaker, relationship

load_dotenv()

DATABASE_URL = os.getenv("DATABASE_URL")
print("DATABASE_URL:", DATABASE_URL)

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
    hashed_password = Column(String)
    github_token = Column(String, nullable=True)

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

    chat = relationship("Chat")

class DailyUsage(Base):
    __tablename__ = "daily_usage"

    id = Column(Integer, primary_key=True)
    user_id = Column(Integer, ForeignKey("users.id"), nullable=False)
    date = Column(Date, default=date.today, nullable=False)
    questions_used = Column(Integer, default=0)

Base.metadata.create_all(bind=engine)


def _add_missing_columns():
    """
    create_all() never alters existing tables, so add the timing columns to
    databases created before they existed.
    """

    existing = {c["name"] for c in inspect(engine).get_columns("messages")}

    with engine.begin() as conn:
        for name in ("response_seconds", "first_word_seconds"):
            if name not in existing:
                conn.execute(text(f"ALTER TABLE messages ADD COLUMN {name} DOUBLE PRECISION"))


_add_missing_columns()
