from langchain_core.prompts import ChatPromptTemplate
from rag.model_factory import ModelFactory

class MultiQueryGenerator:

    def __init__(self):
        self.model = ModelFactory.query_generator()

        self.prompt = ChatPromptTemplate.from_template(
            """
Generate exactly 2 alternative search queries for the following repository question.

Original Query:
{original_query}

Instructions:
- Preserve the original meaning.
- Rephrase the query from different perspectives.
- Optimize the queries for retrieving relevant code, documentation, and implementation details.
- Return exactly 2 queries.
- Do not number the queries.
- Do not include explanations.
- Output only the queries, one per line.
"""
        )

    def generate(self, original_query: str):
    
        chain = self.prompt | self.model
    
        result = chain.invoke(
            {
                "original_query": original_query
            }
        )
    
        content = result.content
    
        if isinstance(content, list):
            text = "".join(
                item.get("text", "")
                for item in content
                if isinstance(item, dict)
            )
        else:
            text = str(content)
    
        generated_queries = [
            query.strip()
            for query in text.splitlines()
            if query.strip()
        ][:2]
    
        queries = [original_query] + generated_queries
    
        return list(dict.fromkeys(queries))