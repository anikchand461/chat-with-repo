import random

# Exact-match phrase sets (after normalization) for questions that have
# nothing to do with the repository - greeting the assistant, thanking it,
# saying bye, or asking what it is. Kept intentionally narrow: only
# unambiguous daily-life pleasantries and "about the chatbot" questions.
# Anything that could plausibly be about the repo (even mixed in with a
# greeting) must fall through to normal RAG - see detect().

GREETINGS = {
    "hi", "hii", "hiii", "hiiii", "hello", "helloo", "hey", "heyy",
    "heya", "hey there", "hi there", "hello there",
    "yo", "sup", "what's up", "whats up", "howdy", "hola",
    "good morning", "good afternoon", "good evening", "greetings",
}

THANKS = {
    "thanks", "thank you", "thanks a lot", "thank you so much",
    "thx", "ty", "appreciate it", "much appreciated", "thanks so much",
}

FAREWELLS = {
    "bye", "goodbye", "good bye", "see you", "see ya", "cya",
    "later", "take care", "bye bye",
}

IDENTITY = {
    "who are you", "what are you", "what is your name", "whats your name",
    "what's your name", "who created you", "who made you", "who built you",
    "who developed you", "who is your creator", "who are your creators",
    "who is your developer", "what is chatwithrepo", "what's chatwithrepo",
    "tell me about yourself", "introduce yourself", "what do you do",
    "what can you do", "what can you help with", "who owns this project",
}

GREETING_RESPONSES = [
    "Hello! 👋 How can I help you explore this repository today?",
    "Hi there! What would you like to know about this project?",
    "Hey! Ready whenever you are - ask me anything about this codebase.",
    "Hello! What can I help you with in this repository?",
    "Hi! What's on your mind about this project?",
]

THANKS_RESPONSES = [
    "You're welcome! Let me know if you need anything else. 🙂",
    "Anytime! Happy to help further if you have more questions.",
    "Glad I could help!",
    "No problem at all - ask away if anything else comes up.",
]

FAREWELL_RESPONSES = [
    "Goodbye! Come back anytime you have questions about this repo. 👋",
    "See you later! Happy coding.",
    "Bye for now! I'll be here whenever you need to dig into the code.",
]

IDENTITY_RESPONSES = [
    "I'm ChatWithRepo, an AI assistant built by Shreya Ghorui and Anik Chand to help you understand, navigate, and contribute to GitHub repositories.",
    "I'm ChatWithRepo, created by Shreya Ghorui and Anik Chand - I help developers explore and understand codebases through conversation.",
    "My name's ChatWithRepo! Built by Shreya Ghorui and Anik Chand to make understanding any GitHub repository faster and easier.",
]

_RESPONSES = {
    "greeting": GREETING_RESPONSES,
    "thanks": THANKS_RESPONSES,
    "farewell": FAREWELL_RESPONSES,
    "identity": IDENTITY_RESPONSES,
}


def _normalize(text: str) -> str:
    text = text.strip().lower()
    text = text.rstrip("?!.,; ")
    return " ".join(text.split())


def detect(question: str) -> str | None:
    """
    Returns "greeting" | "thanks" | "farewell" | "identity" when `question`
    is *exactly* (after trimming case/punctuation/whitespace) one of the
    known small-talk phrases, else None. Deliberately exact-match only -
    "hi, how does auth work?" must never match, since it's a real question.
    """

    if not question:
        return None

    normalized = _normalize(question)

    if normalized in GREETINGS:
        return "greeting"
    if normalized in THANKS:
        return "thanks"
    if normalized in FAREWELLS:
        return "farewell"
    if normalized in IDENTITY:
        return "identity"

    return None


def respond(category: str) -> str:
    return random.choice(_RESPONSES[category])
