from rag.loader import JSONLoader
from rag.converter import DocumentConverter
from rag.chunker import Chunker
from rag.vector_store import VectorStore
from rag.retriever import Retriever
from rag.llm import LLM


def main():

    # Load
    loader = JSONLoader("data/")
    data = loader.load()

    # Convert
    documents = DocumentConverter().convert(data)

    # Chunk
    chunks = Chunker().split_documents(documents)

    # Store
    db = VectorStore()
    db.add_documents(chunks)

    print("✅ Index Created")

    # Retrieve
    retriever = Retriever()

    question = "What is this repository about?"

    docs = retriever.retrieve(question)

    print(f"Retrieved {len(docs)} chunks")

    # LLM
    llm = LLM()

    answer = llm.generate(question, docs)

    print("\nAnswer:\n")
    print(answer)


if __name__ == "__main__":
    main()
    