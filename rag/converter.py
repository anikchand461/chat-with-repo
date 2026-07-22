from pathlib import Path
from typing import Any

from langchain_core.documents import Document


class DocumentConverter:
    """
    Convert repository JSON into LangChain Documents.
    """

    def convert(self, data: Any) -> list[Document]:
        documents = []

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

        # --------------------------------------------------
        # Find README
        # --------------------------------------------------

        readme_content = ""

        for file in files:
            filename = Path(file.get("path", "")).name.lower()

            if filename.startswith("readme"):
                readme_content = file.get("content", "").strip()
                break

        # --------------------------------------------------
        # Repository Overview (NEW)
        # --------------------------------------------------

        overview = Document(
            page_content=f"""
Repository Overview

Repository:
{repository_name}

This document provides a high-level overview of the repository.

Repository README:

{readme_content[:3000]}
""",
            metadata={
                "repository": repository_name,
                "document_type": "repository_overview",
                "path": "__repository_overview__",
                "filename": "__repository_overview__",
                "extension": ".overview",
            },
        )

        documents.append(overview)

        # --------------------------------------------------
        # Convert all files
        # --------------------------------------------------

        for file in files:

            content = file.get("content", "").strip()

            if not content:
                continue

            path = file.get("path", "unknown")
            filename = Path(path).name
            extension = Path(path).suffix.lower()

            # -----------------------------------------
            # README
            # -----------------------------------------

            if filename.lower().startswith("readme"):

                page_content = f"""
Repository: {repository_name}

Document Type: Repository README

File: {path}

This README explains:

- Project Overview
- Main Purpose
- Features
- Installation
- Usage
- Technologies

README Content:

{content}
"""

                document_type = "readme"

            # -----------------------------------------
            # Documentation
            # -----------------------------------------

            elif extension in {".md", ".html"}:

                page_content = f"""
Repository: {repository_name}

Document Type: Documentation

File: {path}

Documentation Content:

{content}
"""

                document_type = "documentation"

            # -----------------------------------------
            # Source Code
            # -----------------------------------------

            else:

                page_content = f"""
Repository: {repository_name}

Document Type: Source Code

File: {path}

Source Code:

{content}
"""

                document_type = "source_code"

            document = Document(
                page_content=page_content,
                metadata={
                    "repository": repository_name,
                    "path": path,
                    "filename": filename,
                    "extension": extension,
                    "document_type": document_type,
                },
            )

            documents.append(document)

        return documents