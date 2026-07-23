from pathlib import Path

from rag.loader import JSONLoader
from rag.converter import DocumentConverter
from rag.chunker import Chunker
from rag.vector_store import VectorStore
from rag.retriever import Retriever
from rag.llm import LLM


def main():
    data_dir = Path("data")
    json_files = sorted(data_dir.glob("*.json"))

    if not json_files:
        print("No repository JSON files found.")
        return

    print("=" * 60)
    print("Available Repositories")
    print("=" * 60)

    for i, file in enumerate(json_files, start=1):
        print(f"{i}. {file.name}")

    while True:
        try:
            choice = int(input("\nSelect repository: "))

            if 1 <= choice <= len(json_files):
                break

            print("Invalid selection.")

        except ValueError:
            print("Please enter a valid number.")

    json_path = json_files[choice - 1]
    repo_name = json_path.stem

    print(f"\nSelected: {repo_name}")

    # Load repository
    loader = JSONLoader(str(json_path))
    data = loader.load()

    # Convert
    converter = DocumentConverter()
    documents = converter.convert(data)

    # Chunk
    chunker = Chunker()
    chunks = chunker.split_documents(documents)

    # Build index
    vector_store = VectorStore(repo_name)
    vector_store.clear()
    vector_store.add_documents(chunks)

    # Retriever
    retriever = Retriever(repo_name)

    model = LLM()

    while True:
        question = input("\nYou > ").strip()

        if question.lower() == "exit":
            break

        if not question:
            continue

        docs = retriever.retrieve(question)

        answer = model.generate(
            question=question,
            documents=docs,
            repo_name=repo_name,
        )

        print(f"\nDevLens >\n{answer}")


if __name__ == "__main__":
    main()