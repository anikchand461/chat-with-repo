from rag.vector_store import VectorStore


class Retriever:
    """
    Retrieves candidate documents from the vector store.
    """

    def __init__(self):
        self.vector_store = VectorStore()

    def retrieve(
        self,
        query: str,
        k: int = 20,
    ):
        """
        Retrieve candidate documents.
        These will later be reranked.
        """

        return self.vector_store.similarity_search(
            query=query,
            k=k,
        )