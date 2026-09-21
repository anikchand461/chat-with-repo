from pathlib import Path
from rag.rrf import ReciprocalRankFusion
from rag.query_classifier import QueryClassifier
from rag.multi_query import MultiQueryGenerator
from rag.loader import JSONLoader
from rag.converter import DocumentConverter
from rag.chunker import Chunker
from rag.vector_store import VectorStore
from rag.retriever import Retriever
from rag.reranker import Reranker
from rag.llm import LLM
from rag.overview import RepositoryOverview
import time
import threading
from concurrent.futures import ThreadPoolExecutor

class RAGPipeline:

    def __init__(self, json_path, collection_name=None, persist_directory="chroma_db"):
        self.json_path = json_path
        self.repo_name = Path(json_path).stem

        self.loader = JSONLoader(json_path)
        self.converter = DocumentConverter()
        self.chunker = Chunker()

        self.vector_store = VectorStore(
            self.repo_name,
            persist_directory=persist_directory,
            collection_name=collection_name,
        )
        # Share one Chroma store / embedding client with the retriever.
        self.retriever = Retriever(vector_store=self.vector_store)

        self.reranker = Reranker()
        self.llm = LLM()

        self.overview, self.display_name = self._load_overview()

        self.query_classifier = QueryClassifier()
        self.multi_query = MultiQueryGenerator()

        self.rrf = ReciprocalRankFusion()

        self._index_lock = threading.Lock()

    def is_indexed(self):
        """
        True when the vector store has documents and the repo data file exists.
        """

        try:
            has_vectors = self.vector_store.db._collection.count() > 0
        except Exception:
            has_vectors = False

        return has_vectors and Path(self.json_path).exists()

    def ensure_index(self, fetch_repository):
        """
        Rebuild the index if it is missing or empty (e.g. the chat was created
        on another machine or the local data was cleared). `fetch_repository`
        downloads the repo JSON and is only called when the file is missing.
        """

        with self._index_lock:
            if self.is_indexed():
                return

            if not Path(self.json_path).exists():
                fetch_repository()

            print(f"Index for {self.repo_name} missing or empty - rebuilding...")
            self.build_index()

    def _load_overview(self):
        """
        Build the overview sent with every question (repo name, file tree,
        README). Missing or unreadable data just means no overview.
        """

        try:
            overview = RepositoryOverview(self.loader.load())
            return overview.build(), overview.name or self.repo_name
        except Exception as e:
            print(f"Overview unavailable: {e}")
            return "", self.repo_name

    def build_index(self):

        data = self.loader.load()

        documents = self.converter.convert(data)

        chunks = self.chunker.split_documents(documents)

        self.vector_store.clear()

        try:
            self.vector_store.add_documents(chunks)
        except Exception:
            # Never leave a half-built index behind; it would look complete.
            self.vector_store.clear()
            raise

        self.overview, self.display_name = self._load_overview()

        print("Repository indexed successfully!")

    RERANK_CANDIDATES = 20

    def _retrieve_and_rerank(self, question):
        """
        Classify, retrieve (in parallel) and rerank. Returns the final docs.
        """

        total = time.perf_counter()

        query_type = self.query_classifier.classify(question)

        retrieve_k = 20 if query_type == "analysis" else 15
        rerank_k = 8 if query_type == "analysis" else 5

        t = time.perf_counter()

        if query_type == "lookup":
            ranked_lists = self.retriever.retrieve([question], k=retrieve_k)
        else:
            # Retrieve for the original question while the extra queries are
            # being generated, then retrieve the extras concurrently.
            with ThreadPoolExecutor(max_workers=2) as pool:
                original = pool.submit(self.retriever.retrieve_one, question, retrieve_k)
                generated = pool.submit(self._generate_queries, question)

                extra_queries = [q for q in generated.result() if q != question]
                ranked_lists = [original.result()]

            if extra_queries:
                ranked_lists += self.retriever.retrieve(extra_queries, k=retrieve_k)

        docs = self.rrf.fuse(ranked_lists)[: self.RERANK_CANDIDATES]

        print(f"Retrieval ({query_type}): {time.perf_counter()-t:.3f}s")

        t = time.perf_counter()

        docs = self.reranker.rerank(
            query=question,
            documents=docs,
            top_k=rerank_k,
        )

        print(f"Reranking: {time.perf_counter()-t:.3f}s")
        print(f"Pre-LLM total: {time.perf_counter()-total:.3f}s")

        return docs

    def _generate_queries(self, question):
        try:
            return self.multi_query.generate(question)
        except Exception as e:
            print(f"Multi Query Error: {e}")
            return [question]

    def ask(self, question, history=None):

        docs = self._retrieve_and_rerank(question)

        t = time.perf_counter()

        answer = self.llm.generate(
            question=question,
            history=history,
            documents=docs,
            repo_name=self.display_name,
            overview=self.overview,
        )

        print(f"LLM: {time.perf_counter()-t:.3f}s\n")

        return answer

    def ask_stream(self, question, history=None):
        """
        Same as ask(), but yields answer text as the LLM produces it.
        """

        docs = self._retrieve_and_rerank(question)

        yield from self.llm.stream(
            question=question,
            history=history,
            documents=docs,
            repo_name=self.display_name,
            overview=self.overview,
        )


_pipelines = {}
_pipelines_lock = threading.Lock()


def get_pipeline(json_path, collection_name, persist_directory):
    """
    Return a cached RAGPipeline for this collection, creating it on first use.
    """

    with _pipelines_lock:
        pipeline = _pipelines.get(collection_name)

        if pipeline is None:
            pipeline = RAGPipeline(
                json_path,
                collection_name=collection_name,
                persist_directory=persist_directory,
            )
            _pipelines[collection_name] = pipeline

        return pipeline
