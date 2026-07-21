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
        return self.text_splitter.split_documents(documents)

    def split_document(
        self,
        document: Document
    ) -> list[Document]:
        """
        Split a single document.
        """
        return self.text_splitter.split_documents([document])