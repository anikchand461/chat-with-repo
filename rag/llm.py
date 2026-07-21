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
You are an expert software engineer.

Answer ONLY using the repository context below.

Repository Context:

{context}

Question:

{question}

If the answer is not present in the repository, say:

"I couldn't find this information in the repository."
"""

        response = self.llm.invoke(prompt)

        return response.content