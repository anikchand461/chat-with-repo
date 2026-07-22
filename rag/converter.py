from pathlib import Path
from typing import Any
from .repository_summary import RepositorySummary
from langchain_core.documents import Document
from bs4 import BeautifulSoup


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

        summary = RepositorySummary()
        
        documents.extend(summary.generate(data))

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
            
                # Convert HTML to plain text
                if extension == ".html":
                    soup = BeautifulSoup(content, "html.parser")
            
                    # Remove CSS and JavaScript
                    for tag in soup(["style", "script"]):
                        tag.decompose()
            
                    # Keep only visible text
                    content = soup.get_text(separator="\n", strip=True)
            
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