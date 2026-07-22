from pathlib import Path
from rag.query_classifier import QueryClassifier
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

        # Add this line
        self.query_classifier = QueryClassifier(self.llm.model)

    def build_index(self):
        data = self.loader.load()
    
        documents = self.converter.convert(data)
    
        chunks = self.chunker.split_documents(documents)
    
        # Clear old vectors for this repository
        self.vector_store.clear()
    
        # Add new vectors
        self.vector_store.add_documents(chunks)
    
        print("Index built successfully!")

    def ask(self, question):
    
        # -------------------------------------------------
        # Classify Query
        # -------------------------------------------------
        try:
            query_type = self.query_classifier.classify(question)
        except Exception:
            query_type = "lookup"
            
        if query_type == "analysis":
            retrieve_k = 30
            rerank_k = 8
        else:
            retrieve_k = 20
            rerank_k = 5
    
        # print(f"\nQuery Type : {query_type}")
        # print(f"Retrieve K : {retrieve_k}")
        # print(f"Rerank K   : {rerank_k}")
    
        # -------------------------------------------------
        # Retrieve
        # -------------------------------------------------
    
        docs = self.retriever.retrieve(
            query=question,
            k=retrieve_k,
        )
    
        # print("\n" + "=" * 80)
        # print("RETRIEVED DOCUMENTS")
        # print("=" * 80)
    
        # for i, doc in enumerate(docs, 1):
        #     print(f"\n[{i}]")
        #     print(doc.metadata)
        #     print(doc.page_content[:300])
        #     print("-" * 80)
    
        # -------------------------------------------------
        # Rerank
        # -------------------------------------------------
    
        docs = self.reranker.rerank(
            query=question,
            documents=docs,
            top_k=rerank_k,
        )
    
        # print("\n" + "=" * 80)
        # print("RERANKED DOCUMENTS")
        # print("=" * 80)
    
        # for i, doc in enumerate(docs, 1):
        #     print(f"\n[{i}]")
        #     print(doc.metadata)
        #     print(doc.page_content[:300])
        #     print("-" * 80)
    
        # -------------------------------------------------
        # Generate
        # -------------------------------------------------
    
        return self.llm.generate(
            question=question,
            documents=docs,
            repo_name=self.repo_name,
        )