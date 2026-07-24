from sqlalchemy import create_engine, Column, Integer, String, ForeignKey, DateTime
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker, relationship
from datetime import datetime

DATABASE_URL = "sqlite:///./devlens.db"
engine = create_engine(DATABASE_URL)
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
Base = declarative_base()

class User(Base):
    __tablename__ = "users"
    id = Column(Integer, primary_key=True, index=True)
    email = Column(String, unique=True, index=True)
    hashed_password = Column(String)
    github_token = Column(String, nullable=True)
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

    chat = relationship("Chat")

Base.metadata.create_all(bind=engine)