from langchain_text_splitters import RecursiveCharacterTextSplitter
from langchain_core.documents import Document


class Chunker:
    """
    Splits LangChain Documents into smaller chunks.
    """

    def __init__(
        self,
        chunk_size: int = 1000,
        chunk_overlap: int = 200,
    ):
        self.text_splitter = RecursiveCharacterTextSplitter(
            chunk_size=chunk_size,
            chunk_overlap=chunk_overlap,
            separators=[
                "\n\n",
                "\n",
                " ",
                ""
            ]
        )

    def split_documents(
        self,
        documents: list[Document]
    ) -> list[Document]:
        """
        Split multiple documents.
        """
        chunks = []

        for document in documents:
            chunks.extend(self.split_document(document))

        return chunks

    def split_document(
        self,
        document: Document
    ) -> list[Document]:
        """
        Split a single document. Every chunk is prefixed with the file path
        so it stays meaningful (and searchable) on its own.
        """
        chunks = self.text_splitter.split_documents([document])

        path = document.metadata.get("path")

        if not path:
            return chunks

        total = len(chunks)

        for index, chunk in enumerate(chunks, start=1):
            chunk.metadata["chunk_index"] = index
            chunk.metadata["chunk_total"] = total

            # The converter already puts the file path at the top of a file's
            # first chunk; only add it where it is missing.
            if f"File: {path}" in chunk.page_content[:300]:
                continue

            part = f" (part {index}/{total})" if total > 1 else ""
            chunk.page_content = f"File: {path}{part}\n\n{chunk.page_content.strip()}"

        return chunks
