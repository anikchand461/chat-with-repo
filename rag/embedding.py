from langchain_huggingface import HuggingFaceEmbeddings


class EmbeddingModel:
    """
    Wrapper around HuggingFace embedding model.
    """

    def __init__(
        self,
        model_name: str = "BAAI/bge-small-en-v1.5"
    ):
        self.embedding = HuggingFaceEmbeddings(
            model_name=model_name,
            model_kwargs={"device": "cpu"},
            encode_kwargs={"normalize_embeddings": True},
        )

    def get_embedding_model(self):
        """
        Return the embedding model.
        """
        return self.embedding