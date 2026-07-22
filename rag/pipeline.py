from rag.loader import JSONLoader
from rag.converter import DocumentConverter
from rag.chunker import Chunker
from rag.vector_store import VectorStore
from rag.retriever import Retriever
from rag.llm import LLM
from pathlib import Path


class RAGPipeline:

    def __init__(self, json_path):
        self.json_path = json_path
        self.repo_name = Path(json_path).stem
        self.loader = JSONLoader(json_path)
        self.converter = DocumentConverter()
        self.chunker = Chunker()
        self.vector_store = VectorStore()
        self.retriever = Retriever()
        self.llm = LLM()

    def build_index(self):
        data = self.loader.load()
        documents = self.converter.convert(data)
        chunks = self.chunker.split_documents(documents)
        self.vector_store.add_documents(chunks)
        print("Index built successfully!")

    def ask(self, question):
        docs = self.retriever.retrieve(question)
        # print("\n========== RETRIEVED DOCUMENTS ==========\n")
        
        # for i, doc in enumerate(docs, 1):
        #     print(f"Document {i}")
        #     print(doc.metadata)
        #     print(doc.page_content[:500])
        #     print("-" * 80)
        
        return self.llm.generate(
            question,
            docs,
            self.repo_name
        )