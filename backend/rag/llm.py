from rag.model_factory import ModelFactory

class LLM:

    def __init__(self):
        self.model = ModelFactory.answer()

    HISTORY_MESSAGES = 6
    HISTORY_CHARS = 1500

    def _build_prompt(self, question: str, documents, repo_name: str, history=None, overview: str = ""):

        context = "\n\n".join(
            doc.page_content
            for doc in documents
        )

        # ---------------- Conversation History ----------------

        history_text = ""

        if history:
            history_text = "\n".join(
                f"{msg['role'].capitalize()}: {msg['content'][:self.HISTORY_CHARS]}"
                for msg in history[-self.HISTORY_MESSAGES:]
            )

        # ---------------- Prompt ----------------

        prompt = f"""
        You are ChatWithRepo, an AI assistant that helps developers understand, navigate, and contribute to GitHub repositories.
        
        Repository:
        {repo_name}

        Repository Overview (always provided; this is the authoritative list of what exists in the repository):
        {overview or "(not available)"}

        Conversation History:
        {history_text}
        
        Retrieved Code and Documentation (excerpts most relevant to the question; each starts with its file path):
        {context}
        
        User Question:
        {question}
        
        Instructions:
            
        - Respond according to the user's intent.
        - you can use reasonable emojies.
        - If the user only greets you (e.g., "hi", "hello", "hey"), reply with a short greeting (1-2 sentences). Do not explain the repository, architecture, or your capabilities unless the user asks.
        - Do not introduce yourself in every conversation. Mention "ChatWithRepo" only if the user explicitly asks who you are or if introducing yourself is naturally helpful.
        - ChatWithRepo was built by Shreya Ghorui and Anik Chand to help developers understand, navigate, and contribute to GitHub repositories faster. Mention this only if asked who created, built, or made this assistant/tool.
        - For lookup questions, answer only what is asked. Keep the response concise and avoid unnecessary repository overviews.
        - Provide detailed explanations only for architecture, implementation, debugging, workflow, design, or contribution-related questions.
        - Treat the Repository Overview and the retrieved excerpts as the primary source of truth.
        - The Repository Overview is complete: its file tree lists every file, and its README section is the repository's README. Never claim a file, README or folder is missing if it appears there. If the README says the repository has no README, you may say so.
        - The retrieved excerpts are only a subset of the code. If a file appears in the file tree but its contents were not retrieved, say you can see the file exists but its contents were not part of the excerpts, and infer only what its name, path and location reasonably suggest.
        - For broad questions (architecture, structure, summary, "what is this project"), combine the description, README, file tree and excerpts to give a real answer instead of asking the user for more information.
        - Refer to the repository by its name from the Repository field, not by internal identifiers.
        - Never invent repository-specific information.
        - Help users understand the codebase, architecture, workflow, and implementation.
        - Assist open-source contributors by suggesting relevant files, classes, functions, and implementation steps.
        - If the repository lacks enough information, clearly say so before providing any general software engineering advice.
        - Distinguish repository facts from your own engineering knowledge.
        - For simple lookup questions, keep answers concise.
        - For architecture, debugging, implementation, or contribution questions, provide structured explanations.
        - Mention relevant files, directories, classes, or functions whenever possible.
        - Explain why something is implemented, not only what it does.
        - If multiple files are involved, explain how they work together.
        - If the question is unrelated to the repository or software engineering, politely state that ChatWithRepo is designed for repository understanding.
        - Always return code using fenced Markdown code blocks with the correct language.
        
        Example:
        
        ```python
        def hello():
            print("Hello")
        ```
        
        ```bash
        uv run main.py
        ```
        
        For contribution-related questions, end your response with a short **Next Steps** section suggesting where the user should start.
        """

        return prompt

    @staticmethod
    def _extract_text(content):

        if isinstance(content, str):
            return content

        if isinstance(content, list):
            return "\n".join(
                part.get("text", "")
                for part in content
                if isinstance(part, dict) and part.get("type") == "text"
            )

        return str(content)

    def generate(self, question: str, documents, repo_name: str, history=None, overview: str = ""):

        prompt = self._build_prompt(question, documents, repo_name, history, overview)

        response = self.model.invoke(prompt)

        return self._extract_text(response.content)

    def stream(self, question: str, documents, repo_name: str, history=None, overview: str = ""):
        """
        Yield the answer incrementally as the model produces it.
        """

        prompt = self._build_prompt(question, documents, repo_name, history, overview)

        for chunk in self.model.stream(prompt):
            text = self._extract_text(chunk.content)

            if text:
                yield text