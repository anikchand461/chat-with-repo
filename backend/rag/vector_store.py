import os
import time

from qdrant_client import QdrantClient
from qdrant_client.http import models as qmodels
from langchain_qdrant import QdrantVectorStore

from rag.embedding import EmbeddingModel


class VectorStore:

    def __init__(
        self,
        repository_name: str,
        persist_directory: str = "chroma_db",  # unused now; kept so existing callers don't need to change
        collection_name: str | None = None,
    ):

        self.embedding = EmbeddingModel().get_embedding_model()

        self.collection_name = collection_name or (
            repository_name.lower()
            .replace("/", "_")
            .replace("-", "_")
            .replace(" ", "_")
        )

        self.client = QdrantClient(
            url=os.getenv("QDRANT_URL"),
            api_key=os.getenv("QDRANT_API_KEY"),
        )

        if not self.client.collection_exists(self.collection_name):
            self._vector_size = len(self.embedding.embed_query("dimension probe"))
            self._create_collection()
        else:
            info = self.client.get_collection(self.collection_name)
            self._vector_size = info.config.params.vectors.size

        self.db = QdrantVectorStore(
            client=self.client,
            collection_name=self.collection_name,
            embedding=self.embedding,
        )

    def _create_collection(self):
        self.client.create_collection(
            collection_name=self.collection_name,
            vectors_config=qmodels.VectorParams(
                size=self._vector_size,
                distance=qmodels.Distance.COSINE,
            ),
        )

    def count(self):
        """
        Number of vectors currently stored for this repository's collection.
        """

        try:
            return self.client.count(
                collection_name=self.collection_name,
                exact=True,
            ).count
        except Exception:
            return 0

    def clear(self):
        """
        Remove every document from this repository collection.
        """

        try:
            self.client.delete_collection(self.collection_name)
            self._create_collection()
        except Exception:
            pass

    BATCH_SIZE = 64
    BATCH_PAUSE = 1.5      # minimum seconds between batches
    # Cohere trial keys also cap embedding tokens (100k/minute). Pace batches to
    # stay under this; set EMBED_TOKENS_PER_MINUTE=0 to disable (production keys).
    TOKENS_PER_MINUTE = int(os.getenv("EMBED_TOKENS_PER_MINUTE", "80000"))
    CHARS_PER_TOKEN = 3.5
    RATE_LIMIT_WAIT = 20   # seconds to wait after a 429 before retrying
    MAX_ATTEMPTS = 6

    def add_documents(self, documents, on_progress=None):
        """
        Embed and store documents in small batches. Embedding APIs rate-limit
        (Cohere trial keys allow 40 calls/minute), so pace the calls and wait
        out 429s instead of failing the whole indexing run.

        `on_progress(done, total)`, if given, is called after each batch -
        used to surface a short "indexing X/Y" status to the frontend.
        """

        total = len(documents)

        for start in range(0, total, self.BATCH_SIZE):
            batch = documents[start : start + self.BATCH_SIZE]

            for attempt in range(1, self.MAX_ATTEMPTS + 1):
                try:
                    self.db.add_documents(batch)
                    break
                except Exception as e:
                    # Retry both Cohere rate limits (429) and transient
                    # network/timeout errors talking to Qdrant (e.g.
                    # httpx.WriteTimeout wrapped in
                    # qdrant_client.http.exceptions.ResponseHandlingException,
                    # seen upserting to the free-tier cluster under load) -
                    # a single hiccup shouldn't fail the whole indexing run.
                    msg = str(e).lower()
                    err_type = type(e).__name__
                    transient = (
                        "429" in str(e)
                        or "TooManyRequests" in err_type
                        or "timed out" in msg
                        or "timeout" in err_type.lower()
                        or "ResponseHandlingException" in err_type
                    )

                    if not transient or attempt == self.MAX_ATTEMPTS:
                        raise

                    print(
                        f"Batch upsert failed ({err_type}), waiting {self.RATE_LIMIT_WAIT}s "
                        f"(attempt {attempt}/{self.MAX_ATTEMPTS})..."
                    )
                    time.sleep(self.RATE_LIMIT_WAIT)

            done = min(start + self.BATCH_SIZE, total)
            print(f"Indexed {done}/{total} chunks")

            if on_progress:
                on_progress(done, total)

            if start + self.BATCH_SIZE < total:
                pause = self.BATCH_PAUSE

                if self.TOKENS_PER_MINUTE > 0:
                    tokens = sum(len(d.page_content) for d in batch) / self.CHARS_PER_TOKEN
                    pause = max(pause, tokens / self.TOKENS_PER_MINUTE * 60)

                time.sleep(pause)

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
