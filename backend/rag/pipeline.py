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

        self.query_classifier = QueryClassifier()
        self.multi_query = MultiQueryGenerator()

        self.rrf = ReciprocalRankFusion()

    def build_index(self):

        data = self.loader.load()

        documents = self.converter.convert(data)

        chunks = self.chunker.split_documents(documents)

        self.vector_store.clear()
        self.vector_store.add_documents(chunks)

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
            repo_name=self.repo_name,
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
            repo_name=self.repo_name,
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
