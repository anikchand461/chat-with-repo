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

    def rerank(
        self,
        query: str,
        documents: list,
        top_k: int = 5,
    ):
        if not documents:
            return []

        response = self.client.rerank(
            model="rerank-v3.5",
            query=query,
            documents=[
                doc.page_content
                for doc in documents
            ],
            top_n=top_k,
        )

        reranked = []

        for result in response.results:
            reranked.append(
                documents[result.index]
            )

        return reranked