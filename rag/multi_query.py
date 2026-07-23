from langchain_core.prompts import ChatPromptTemplate


class MultiQueryGenerator:

    def __init__(self, model):
        self.model = model

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

        # Extract response text safely
        content = result.content

        if isinstance(content, list):
            text = ""
            for item in content:
                if isinstance(item, dict):
                    text += item.get("text", "")
                else:
                    text += str(item)
        else:
            text = str(content)

        # Split into individual queries
        generated_queries = [
            q.strip()
            for q in text.split("\n")
            if q.strip()
        ]

        # Keep only the first 2 generated queries
        generated_queries = generated_queries[:2]

        # Final output = Original + 2 generated = 3 total queries
        queries = [original_query] + generated_queries

        # Remove duplicates while preserving order
        queries = list(dict.fromkeys(queries))

        return queries