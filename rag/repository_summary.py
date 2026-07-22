from pathlib import Path
from collections import Counter
from langchain_core.documents import Document


class RepositorySummary:
    """
    Generates synthetic repository-level documents.

    These documents are indexed alongside the repository files
    to improve repository-level retrieval.
    """

    def generate(self, repo_data: dict) -> list[Document]:
        files = repo_data.get("files", [])

        repository_name = self._extract_repository_name(repo_data)

        documents = [
            self._repository_overview(repository_name, files),
            self._repository_statistics(repository_name, files),
            self._repository_inventory(repository_name, files),
            self._folder_summary(repository_name, files),
        ]

        return documents


    # # ==========================================================
    # # Execution Flow
    # # ==========================================================
    
    # def _execution_flow(self, repository: str, files: list) -> Document:
    
    #     main_files = []
    
    #     for file in files:
    
    #         path = file.get("path", "")
    #         name = Path(path).name.lower()
    
    #         if (
    #             name.startswith("main.")
    #             or "/cmd/" in path.lower()
    #         ):
    #             main_files.append(path)
    
    #     content = f"""
    # Execution Flow
    
    # Repository:
    # {repository}
    
    # Application entry points:
    
    # {chr(10).join(sorted(main_files)) if main_files else "No entry point detected"}
    
    # General execution flow:
    
    # User Input
    # ↓
    
    # Entry Point
    
    # ↓
    
    # Application Logic
    
    # ↓
    
    # Internal Modules
    
    # ↓
    
    # Storage / Output
    # """
    
    #     return Document(
    #         page_content=content,
    #         metadata={
    #             "repository": repository,
    #             "document_type": "execution_flow",
    #         },
    #     )

    # ==========================================================
    # Helpers
    # ==========================================================

    def _extract_repository_name(self, repo_data: dict) -> str:
        """
        Support multiple JSON formats.
        """

        repo = repo_data.get("repository")

        if isinstance(repo, dict):
            return repo.get("name", "Unknown Repository")

        if isinstance(repo, str):
            return repo

        return repo_data.get("repository_name", "Unknown Repository")

    # ==========================================================
    # Repository Overview
    # ==========================================================

    def _repository_overview(self, repository: str, files: list) -> Document:

        readme = ""

        for file in files:
            filename = Path(file.get("path", "")).name.lower()

            if filename.startswith("readme"):
                readme = file.get("content", "")[:4000]
                break

        content = f"""
Repository Overview

Repository:
{repository}

This document provides a high-level overview of the repository.

README Summary:

{readme}
"""

        return Document(
            page_content=content,
            metadata={
                "repository": repository,
                "document_type": "repository_overview",
            },
        )

    # ==========================================================
    # Repository Statistics
    # ==========================================================

    def _repository_statistics(self, repository: str, files: list) -> Document:
    
        extensions = Counter()
        directories = Counter()
    
        total_size = 0
    
        for file in files:
    
            path = file.get("path", "")
    
            ext = Path(path).suffix.lower()
    
            if not ext:
                ext = "[no extension]"
    
            extensions[ext] += 1
    
            directory = str(Path(path).parent)
    
            directories[directory] += 1
    
            total_size += len(file.get("content", ""))
    
        extension_summary = "\n".join(
            f"{ext}: {count}"
            for ext, count in sorted(
                extensions.items(),
                key=lambda x: (-x[1], x[0])
            )
        )
    
        largest_directories = "\n".join(
            f"{directory}: {count} files"
            for directory, count in sorted(
                directories.items(),
                key=lambda x: (-x[1], x[0])
            )[:10]
        )
    
        content = f"""
    Repository Statistics
    
    Repository:
    {repository}
    
    Total Files:
    {len(files)}
    
    Total Directories:
    {len(directories)}
    
    Unique File Extensions:
    {len(extensions)}
    
    Approximate Repository Size:
    {total_size:,} characters
    
    File Extension Distribution:
    
    {extension_summary}
    
    Largest Directories:
    
    {largest_directories if largest_directories else "None"}
    """
    
        return Document(
            page_content=content,
            metadata={
                "repository": repository,
                "document_type": "repository_statistics",
            },
        )


    # ==========================================================
    # Repository Inventory
    # ==========================================================
    
    def _repository_inventory(self, repository: str, files: list) -> Document:
    
        directories = Counter()
        extensions = Counter()
    
        for file in files:
    
            path = file.get("path", "")
    
            directory = str(Path(path).parent)
            directories[directory] += 1
    
            ext = Path(path).suffix.lower()
    
            if not ext:
                ext = "[no extension]"
    
            extensions[ext] += 1
    
        directory_summary = "\n".join(
            f"{directory}: {count} files"
            for directory, count in sorted(
                directories.items(),
                key=lambda x: (-x[1], x[0])
            )
        )
    
        extension_summary = "\n".join(
            f"{ext}: {count}"
            for ext, count in sorted(
                extensions.items(),
                key=lambda x: (-x[1], x[0])
            )
        )
    
        content = f"""
    Repository Inventory
    
    Repository:
    {repository}
    
    Directories
    
    {directory_summary}
    
    File Extensions
    
    {extension_summary}
    """
    
        return Document(
            page_content=content,
            metadata={
                "repository": repository,
                "document_type": "repository_inventory",
            },
        )
    
    # ==========================================================
    # Folder Summary
    # ==========================================================
    
    def _folder_summary(self, repository: str, files: list) -> Document:
    
        folders = Counter()
    
        folder_files = {}
    
        for file in files:
    
            path = file.get("path", "")
    
            folder = str(Path(path).parent)
    
            folders[folder] += 1
    
            folder_files.setdefault(folder, [])
            folder_files[folder].append(Path(path).name)
    
        lines = []
    
        for folder, count in sorted(
            folders.items(),
            key=lambda x: (-x[1], x[0])
        ):
    
            lines.append(f"{folder} ({count} files)")
    
            for filename in sorted(folder_files[folder])[:10]:
                lines.append(f"  - {filename}")
    
            if len(folder_files[folder]) > 10:
                lines.append(
                    f"  ... {len(folder_files[folder]) - 10} more"
                )
    
            lines.append("")
    
        content = f"""
    Folder Summary
    
    Repository:
    {repository}
    
    Directory Structure
    
    {chr(10).join(lines)}
    """
    
        return Document(
            page_content=content,
            metadata={
                "repository": repository,
                "document_type": "folder_summary",
            },
        )