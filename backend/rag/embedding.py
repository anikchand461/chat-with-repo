from rag.model_factory import ModelFactory


class EmbeddingModel:

    def __init__(self):
        self.embedding = ModelFactory.embedding()

    def get_embedding_model(self):
        return self.embedding