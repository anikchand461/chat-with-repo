from langchain_chroma import Chroma

from rag.embedding import EmbeddingModel


class VectorStore:

    def __init__(
        self,
        repository_name: str,
        persist_directory: str = "chroma_db",
    ):

        self.embedding = EmbeddingModel().get_embedding_model()

        self.collection_name = (
            repository_name.lower()
            .replace("/", "_")
            .replace("-", "_")
            .replace(" ", "_")
        )

        self.db = Chroma(
            collection_name=self.collection_name,
            persist_directory=persist_directory,
            embedding_function=self.embedding,
        )

    def clear(self):
        """
        Remove every document from this repository collection.
        """

        try:
            ids = self.db.get()["ids"]

            if ids:
                self.db.delete(ids=ids)

        except Exception:
            pass

    def add_documents(self, documents):
        self.db.add_documents(documents)

    def similarity_search(
        self,
        query: str,
        k: int = 5,
    ):
        return self.db.similarity_search(
            query=query,
            k=k,
        )

    def get_vector_store(self):
        return self.db