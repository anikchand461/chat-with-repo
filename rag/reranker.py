import os

from dotenv import load_dotenv
from cohere import ClientV2

load_dotenv()


class Reranker:
    """
    Reranks retrieved documents using Cohere Rerank.
    """

    def __init__(self):
        self.client = ClientV2(
            api_key=os.getenv("COHERE_API_KEY")
        )

    def _format_document(self, doc):
        """
        Create a richer representation of a document for reranking.
        """

        repository = doc.metadata.get("repository", "Unknown")
        path = doc.metadata.get("path", "Unknown")
        document_type = doc.metadata.get("document_type", "Unknown")

        return f"""
Repository: {repository}

File: {path}

Document Type: {document_type}

Content:

{doc.page_content}
""".strip()

    def rerank(
        self,
        query: str,
        documents: list,
        top_k: int = 5,
    ):
        if not documents:
            return []

        formatted_documents = [
            self._format_document(doc)
            for doc in documents
        ]

        response = self.client.rerank(
            model="rerank-v3.5",
            query=query,
            documents=formatted_documents,
            top_n=min(top_k, len(formatted_documents)),
        )

        reranked_documents = [
            documents[result.index]
            for result in response.results
        ]

        return reranked_documents