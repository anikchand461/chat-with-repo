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

class RAGPipeline:

    def __init__(self, json_path):
        self.json_path = json_path
        self.repo_name = Path(json_path).stem

        self.loader = JSONLoader(json_path)
        self.converter = DocumentConverter()
        self.chunker = Chunker()

        self.vector_store = VectorStore(self.repo_name)
        self.retriever = Retriever(self.repo_name)

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

    def ask(self, question):

        # ---------------- Query Classification ----------------

        try:
            query_type = self.query_classifier.classify(question)
        except Exception as e:
            print(f"Query Classification Error: {e}")
            query_type = "lookup"

        retrieve_k = 30 if query_type == "analysis" else 20
        rerank_k = 8 if query_type == "analysis" else 5

        # print(f"\nQuery Type: {query_type}")

        # ---------------- Multi Query Generation ----------------

        try:
            queries = self.multi_query.generate(question)

            # print("\nGenerated Queries:")
            # for i, query in enumerate(queries, start=1):
            #     print(f"{i}. {query}")

        except Exception as e:

            print(f"\nMulti Query Error: {e}")

            queries = [question]

        # ---------------- Retrieval ----------------
        
        ranked_lists = self.retriever.retrieve(
            queries=queries,
            k=retrieve_k,
        )
        
        docs = self.rrf.fuse(ranked_lists)

        # print(f"\nRetrieved {len(docs)} unique documents")

        # ---------------- Reranking ----------------

        docs = docs[:30]
        
        docs = self.reranker.rerank(
            query=question,
            documents=docs,
            top_k=rerank_k,
        )

        # print(f"Top {len(docs)} documents after reranking")

        # ---------------- Answer Generation ----------------

        return self.llm.generate(
            question=question,
            documents=docs,
            repo_name=self.repo_name,
        )