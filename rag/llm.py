import os

from dotenv import load_dotenv
from langchain_google_genai import ChatGoogleGenerativeAI

load_dotenv()


class LLM:

    def __init__(self):
        self.llm = ChatGoogleGenerativeAI(
            model="gemini-3.1-flash-lite",
            google_api_key=os.getenv("GOOGLE_API_KEY"),
            temperature=0.2,
        )

    def generate(self, question: str, documents, repo_name: str):

        context = "\n\n".join(
            doc.page_content
            for doc in documents
        )

        prompt = f"""
You are DevLens, an AI assistant specialized in understanding GitHub repositories.

Current Repository:
{repo_name}

Repository Context:
{context}

User Question:
{question}

Instructions:
1. You are currently answering questions ONLY about the repository "{repo_name}".
2. Never mention or assume information from another repository.
3. If the user greets you (e.g. "hello", "hi"), greet them politely and mention the current repository.
4. If the user asks a question unrelated to this repository, politely explain that you answer repository-related questions only.
5. If the answer is not present in the repository context, reply exactly:
"I couldn't find this information in the repository."
6. Keep answers concise, accurate, and based only on the provided repository context.
"""

        response = self.llm.invoke(prompt)

        content = response.content

        if isinstance(content, str):
            return content

        if isinstance(content, list):
            return "\n".join(
                part.get("text", "")
                for part in content
                if isinstance(part, dict) and part.get("type") == "text"
            )

        return str(content)