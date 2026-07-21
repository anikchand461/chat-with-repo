from rag.vector_store import VectorStore


class Retriever:
    """
    Retrieves the most relevant chunks from ChromaDB.
    """

    def __init__(self):
        self.vector_store = VectorStore()

    def retrieve(
        self,
        query: str,
        k: int = 5
    ):
        """
        Retrieve the top-k most relevant documents.
        """

        return self.vector_store.similarity_search(
            query=query,
            k=k
        )