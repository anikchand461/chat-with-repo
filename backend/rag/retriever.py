from concurrent.futures import ThreadPoolExecutor

from rag.vector_store import VectorStore
from dotenv import load_dotenv

load_dotenv()


class Retriever:

    FETCH_K = 30

    def __init__(self, repository_name=None, persist_directory="chroma_db", collection_name=None, vector_store=None):
        # Reuse an existing store when given, so we don't open a second
        # Chroma client + embedding client for the same collection.
        self.vector_store = vector_store or VectorStore(
            repository_name,
            persist_directory=persist_directory,
            collection_name=collection_name,
        )

    def retrieve_one(self, query, k=15):
        db = self.vector_store.db

        embedding = self.vector_store.embedding.embed_query(query)

        return db.max_marginal_relevance_search_by_vector(
            embedding=embedding,
            k=k,
            fetch_k=max(self.FETCH_K, k),
            lambda_mult=0.5,
        )

    def retrieve(self, queries, k=15):
        """
        Retrieve for every query concurrently (each one is a network embed
        call followed by a Chroma lookup). Result order matches `queries`.
        """

        if len(queries) == 1:
            return [self.retrieve_one(queries[0], k)]

        with ThreadPoolExecutor(max_workers=len(queries)) as pool:
            return list(pool.map(lambda q: self.retrieve_one(q, k), queries))
