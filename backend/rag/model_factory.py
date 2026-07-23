import os
from dotenv import load_dotenv
from langchain_groq import ChatGroq
from langchain_google_genai import ChatGoogleGenerativeAI
from langchain_cohere import CohereEmbeddings

load_dotenv()

class ModelFactory:

    GROQ_MODEL = "llama-3.1-8b-instant"
    GEMINI_MODEL = "gemini-3.1-flash-lite"
    EMBEDDING_MODEL = "embed-v4.0"
    RERANK_MODEL = "rerank-v3.5"

    @classmethod
    def classifier(cls):
        return ChatGroq(
            model=cls.GROQ_MODEL,
            api_key=os.getenv("GROQ_API_KEY"), 
            temperature=0,
        )

    @classmethod
    def query_generator(cls):
        return ChatGroq(
            model=cls.GROQ_MODEL,
            api_key=os.getenv("GROQ_API_KEY"),
            temperature=0.2,
        )

    @classmethod
    def answer(cls):
        return ChatGoogleGenerativeAI(
            model=cls.GEMINI_MODEL,
            google_api_key=os.getenv("GOOGLE_API_KEY"),
            temperature=0.2,
        )

    @classmethod
    def embedding(cls):
        return CohereEmbeddings(  # pyright: ignore[reportCallIssue]
            model=cls.EMBEDDING_MODEL,
            cohere_api_key=os.getenv("COHERE_API_KEY"),
        )

    @classmethod
    def reranker_model(cls):
        return cls.RERANK_MODEL

