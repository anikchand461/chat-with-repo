import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(PROJECT_ROOT))

from rag.pipeline import RAGPipeline


def main():
    data_dir = PROJECT_ROOT / "data"
    json_files = sorted(data_dir.glob("*.json"))

    if not json_files:
        print("❌ No JSON files found in the data folder.")
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

            print("Invalid choice.")

        except ValueError:
            print("Please enter a number.")

    selected_file = json_files[choice - 1]

    print("\n" + "=" * 60)
    print("Building Repository Index")
    print(f"Repository : {selected_file.name}")
    print("=" * 60)

    rag = RAGPipeline(
        json_path=str(selected_file)
    )

    # Build embeddings
    rag.build_index()

    print("\n✅ Repository indexed successfully!")
    print("Type 'exit' to quit.\n")

    while True:
        question = input("You > ").strip()
        if not question:
            print("Please enter a question.")
            continue

        if question.lower() in {"exit", "quit"}:
            print("\nGoodbye!")
            break

        print("\nDevLens >")
        answer = rag.ask(question)
        print(answer)
        print("-" * 60)


if __name__ == "__main__":
    main()