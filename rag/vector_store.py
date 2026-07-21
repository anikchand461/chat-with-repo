from langchain_chroma import Chroma

from rag.embedding import EmbeddingModel


class VectorStore:

    def __init__(
        self,
        persist_directory: str = "chroma_db"
    ):

        self.embedding = EmbeddingModel().get_embedding_model()

        self.db = Chroma(
            persist_directory=persist_directory,
            embedding_function=self.embedding
        )

    def add_documents(self, documents):
        """
        Add chunked documents to ChromaDB.
        """
        self.db.add_documents(documents)

    def similarity_search(
        self,
        query: str,
        k: int = 5
    ):
        """
        Search similar documents.
        """
        return self.db.similarity_search(
            query=query,
            k=k
        )

    def get_vector_store(self):
        return self.db