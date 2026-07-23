from rag.vector_store import VectorStore


class Retriever:

    def __init__(self, repository_name):
        self.vector_store = VectorStore(repository_name)

    def retrieve(self, queries, k=20):

        ranked_lists = []

        for query in queries:

            docs = self.vector_store.db.max_marginal_relevance_search(
                query=query,
                k=k,
                fetch_k=50,
                lambda_mult=0.5,
            )

            ranked_lists.append(docs)

        return ranked_lists