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

    def generate(self, question: str, documents):

        context = "\n\n".join(
            doc.page_content
            for doc in documents
        )

        prompt = f"""
You are DevLens, an AI assistant that helps developers understand GitHub repositories.

Repository Context:
{context}

User Question:
{question}

Instructions:
- Answer using ONLY the repository context.
- If the user greets you (e.g. "hello", "hi"), greet them politely.
- If the user asks a question unrelated to the repository, politely explain that you only answer questions about this repository.
- If the requested information is not found in the repository, reply:
  "I couldn't find this information in the repository."
- Keep your answers concise and accurate.
"""

        response = self.llm.invoke(prompt)

        if isinstance(response.content, list):
            return "\n".join(
                block.get("text", "")
                for block in response.content
                if isinstance(block, dict) and block.get("type") == "text"
            )

        return str(response.content)