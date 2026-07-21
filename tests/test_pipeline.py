from rag.pipeline import RAGPipeline

rag = RAGPipeline(
    json_path="data/shreyaghorui222004_MedIBot.json"
)

rag.build_index()

answer = rag.ask(
    "What is this repository about?"
)

print(answer)