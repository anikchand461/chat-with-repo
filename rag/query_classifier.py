from langchain_core.prompts import ChatPromptTemplate

class QueryClassifier:

    def __init__(self, model):
        self.model = model

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

        classification = result.content.lower()
        
        if "analysis" in classification:
            return "analysis"
        
        return "lookup"