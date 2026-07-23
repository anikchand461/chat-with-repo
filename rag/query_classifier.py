from langchain_core.prompts import ChatPromptTemplate
from rag.model_factory import ModelFactory

class QueryClassifier:

    def __init__(self):
        self.model = ModelFactory.classifier()

        self.prompt = ChatPromptTemplate.from_messages([
            (
                "system",
                """
Classify the user's repository question.

Return ONLY one word:

lookup
analysis

lookup:
- asks for a function
- asks for usage
- asks for a file
- asks where something is
- asks about commands

analysis:
- architecture
- codebase
- module interaction
- dependencies
- design
- workflow
- reasoning
- implementation
- relationships
- overall explanation
"""
            ),
            ("human", "{question}")
        ])

    def classify(self, question):

        chain = self.prompt | self.model

        result = chain.invoke({
            "question": question
        })

        # Safely extract text from the model response
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

        classification = text.lower().strip()

        # print(f"\nQuery Classification Result: {classification}")

        if "analysis" in classification:
            return "analysis"

        return "lookup"