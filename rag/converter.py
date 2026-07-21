from pathlib import Path
from typing import Any

from langchain_core.documents import Document


class DocumentConverter:
    """
    Convert repository JSON into LangChain Documents.
    """

    def convert(self, data: Any) -> list[Document]:
        documents = []

        # If JSON is:
        # {
        #   "files": [...]
        # }
        if isinstance(data, dict):
            files = data.get("files", [])

        # If JSON is:
        # [
        #   {...},
        #   {...}
        # ]
        elif isinstance(data, list):
            files = data

        else:
            raise ValueError("Unsupported JSON format.")

        for file in files:

            content = file.get("content", "")

            if not content.strip():
                continue

            path = file.get("path", "unknown")

            document = Document(
                page_content=content,
                metadata={
                    "path": path,
                    "filename": Path(path).name,
                    "extension": Path(path).suffix,
                },
            )

            documents.append(document)

        return documents