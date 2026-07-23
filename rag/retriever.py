from rag.vector_store import VectorStore


class Retriever:

    def __init__(self, repository_name):
        self.vector_store = VectorStore(repository_name)

    def retrieve(self, queries, k=20):

        all_docs = []
        seen = set()

        for query in queries:

            docs = self.vector_store.db.max_marginal_relevance_search(
                query=query,
                k=k,
                fetch_k=50,
                lambda_mult=0.5,
            )

            for doc in docs:
                text = doc.page_content

                if text not in seen:
                    seen.add(text)
                    all_docs.append(doc)

        return all_docs