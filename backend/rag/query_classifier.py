import re

from rag import smalltalk


class QueryClassifier:
    """
    Cheap heuristic classifier (no LLM round trip).

    Returns "smalltalk" for greetings/thanks/farewells/"who are you"
    questions (see rag.smalltalk - exact match only), "analysis" for
    broad / architectural questions, and "lookup" for short, targeted
    ones.
    """

    ANALYSIS_KEYWORDS = (
        "architecture",
        "codebase",
        "overall",
        "overview",
        "summarize",
        "summary",
        "design",
        "workflow",
        "flow",
        "dependenc",
        "relationship",
        "interact",
        "implement",
        "why",
        "explain",
        "how does",
        "how do",
        "how is",
        "structure",
        "contribute",
        "compare",
        "difference",
        "end to end",
        "end-to-end",
    )

    LOOKUP_KEYWORDS = (
        "where is",
        "where are",
        "which file",
        "what file",
        "show me",
        "what command",
        "how to run",
        "how to install",
        "usage of",
        "defined",
    )

    LONG_QUESTION_WORDS = 18

    def classify(self, question):
        if smalltalk.detect(question) is not None:
            return "smalltalk"

        text = question.lower().strip()

        if any(keyword in text for keyword in self.LOOKUP_KEYWORDS):
            return "lookup"

        if any(keyword in text for keyword in self.ANALYSIS_KEYWORDS):
            return "analysis"

        if len(re.findall(r"\w+", text)) >= self.LONG_QUESTION_WORDS:
            return "analysis"

        return "lookup"
