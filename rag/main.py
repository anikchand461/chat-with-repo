from rag.loader import JSONLoader
from rag.converter import DocumentConverter
from rag.chunker import Chunker
from rag.vector_store import VectorStore
from rag.retriever import Retriever
from rag.llm import LLM


def main():
    # Load repository JSON
    loader = JSONLoader("data/shreyaghorui222004_MedIBot.json")
    data = loader.load()

    # Convert JSON to Documents
    converter = DocumentConverter()
    documents = converter.convert(data)

    print(f"Loaded {len(documents)} documents")

    # Split documents into chunks
    chunker = Chunker()
    chunks = chunker.split_documents(documents)

    print(f"Created {len(chunks)} chunks")

    # Store chunks in ChromaDB
    vector_store = VectorStore()
    vector_store.add_documents(chunks)

    print("Documents stored in ChromaDB")

    # Retrieve relevant chunks
    retriever = Retriever()

    while True:
        question = input("\nAsk a question (type 'exit' to quit): ")

        if question.lower() == "exit":
            break

        docs = retriever.retrieve(question)

        llm = LLM()
        answer = llm.generate(question, docs)

        print("\nAnswer:\n")
        print(answer)


if __name__ == "__main__":
    main()