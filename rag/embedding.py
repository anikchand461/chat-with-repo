import os

from dotenv import load_dotenv
from langchain_cohere import CohereEmbeddings

load_dotenv()


class EmbeddingModel:

    def __init__(
        self,
        model_name="embed-v4.0"
    ):

        self.embedding = CohereEmbeddings(
            model=model_name,
            cohere_api_key=os.getenv("COHERE_API_KEY")
        )

    def get_embedding_model(self):
        return self.embedding