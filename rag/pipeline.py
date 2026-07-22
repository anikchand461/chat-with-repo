from rag.loader import JSONLoader
from rag.converter import DocumentConverter
from rag.chunker import Chunker
from rag.vector_store import VectorStore
from rag.retriever import Retriever
from rag.llm import LLM


class RAGPipeline:

    def __init__(self, json_path):
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
        return self.llm.generate(question, docs)