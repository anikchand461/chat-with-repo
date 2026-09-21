import os
import time

from langchain_chroma import Chroma

from rag.embedding import EmbeddingModel


class VectorStore:

    def __init__(
        self,
        repository_name: str,
        persist_directory: str = "chroma_db",
        collection_name: str | None = None,
    ):

        self.embedding = EmbeddingModel().get_embedding_model()

        self.collection_name = collection_name or (
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

    BATCH_SIZE = 64
    BATCH_PAUSE = 1.5      # minimum seconds between batches
    # Cohere trial keys also cap embedding tokens (100k/minute). Pace batches to
    # stay under this; set EMBED_TOKENS_PER_MINUTE=0 to disable (production keys).
    TOKENS_PER_MINUTE = int(os.getenv("EMBED_TOKENS_PER_MINUTE", "80000"))
    CHARS_PER_TOKEN = 3.5
    RATE_LIMIT_WAIT = 20   # seconds to wait after a 429 before retrying
    MAX_ATTEMPTS = 6

    def add_documents(self, documents):
        """
        Embed and store documents in small batches. Embedding APIs rate-limit
        (Cohere trial keys allow 40 calls/minute), so pace the calls and wait
        out 429s instead of failing the whole indexing run.
        """

        total = len(documents)

        for start in range(0, total, self.BATCH_SIZE):
            batch = documents[start : start + self.BATCH_SIZE]

            for attempt in range(1, self.MAX_ATTEMPTS + 1):
                try:
                    self.db.add_documents(batch)
                    break
                except Exception as e:
                    rate_limited = "429" in str(e) or "TooManyRequests" in type(e).__name__

                    if not rate_limited or attempt == self.MAX_ATTEMPTS:
                        raise

                    print(
                        f"Embedding rate limited, waiting {self.RATE_LIMIT_WAIT}s "
                        f"(attempt {attempt}/{self.MAX_ATTEMPTS})..."
                    )
                    time.sleep(self.RATE_LIMIT_WAIT)

            print(f"Indexed {min(start + self.BATCH_SIZE, total)}/{total} chunks")

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