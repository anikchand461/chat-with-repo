from pathlib import Path
from typing import Any

from langchain_core.documents import Document


class DocumentConverter:
    """
    Convert repository JSON into LangChain Documents.
    """

    def convert(self, data: Any) -> list[Document]:
        documents = []

        # Repository name (if available)
        repository_name = "Unknown Repository"

        if isinstance(data, dict):
            files = data.get("files", [])

            repository = data.get("repository", {})
            repository_name = (
                repository.get("name")
                or data.get("repository_name")
                or "Unknown Repository"
            )

        elif isinstance(data, list):
            files = data

        else:
            raise ValueError("Unsupported JSON format.")

        for file in files:

            content = file.get("content", "").strip()

            if not content:
                continue

            path = file.get("path", "unknown")
            filename = Path(path).name
            extension = Path(path).suffix.lower()

            # -------------------------------
            # README files
            # -------------------------------
            if filename.lower().startswith("readme"):

                page_content = f"""
Repository: {repository_name}

Document Type: Repository README

File: {path}

This document describes the repository, its purpose, features,
installation, usage, and other important information.

Content:

{content}
"""

            # -------------------------------
            # Documentation
            # -------------------------------
            elif extension in {".md", ".html"}:

                page_content = f"""
Repository: {repository_name}

Document Type: Documentation

File: {path}

Documentation Content:

{content}
"""

            # -------------------------------
            # Source Code
            # -------------------------------
            else:

                page_content = f"""
Repository: {repository_name}

Document Type: Source Code

File: {path}

Code:

{content}
"""

            document = Document(
                page_content=page_content,
                metadata={
                    "repository": repository_name,
                    "path": path,
                    "filename": filename,
                    "extension": extension,
                    "document_type": (
                        "readme"
                        if filename.lower().startswith("readme")
                        else (
                            "documentation"
                            if extension in {".md", ".html"}
                            else "source_code"
                        )
                    ),
                },
            )

            documents.append(document)

        return documents