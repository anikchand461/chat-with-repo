import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(PROJECT_ROOT))

from rag.loader import JSONLoader
from rag.converter import DocumentConverter

loader = JSONLoader("data/shreyaghorui222004_MedIBot.json")
data = loader.load()

converter = DocumentConverter()
documents = converter.convert(data)

print("Total Documents:", len(documents))

print(documents[0].metadata)
print(documents[0].page_content[:300])