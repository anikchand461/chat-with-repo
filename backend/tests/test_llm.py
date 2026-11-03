"""
Answer generation: Groq first, Gemini when Groq can't answer.

Fake models, no network:

    .venv/bin/python -m unittest backend.tests.test_llm -v
"""

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))  # rag.* imports

from rag.llm import LLM  # noqa: E402


class Chunk:
    def __init__(self, content):
        self.content = content


class FakeModel:
    def __init__(self, words=(), fail_after=None, error=None):
        self.words = list(words)
        self.fail_after = fail_after  # raise after this many words
        self.error = error or RuntimeError("429 rate_limit_exceeded")
        self.calls = 0

    def stream(self, prompt):
        self.calls += 1
        for i, w in enumerate(self.words):
            if self.fail_after is not None and i == self.fail_after:
                raise self.error
            yield Chunk(w)
        if self.fail_after is not None and self.fail_after >= len(self.words):
            raise self.error


def make_llm(groq, gemini):
    llm = LLM.__new__(LLM)  # skip creating real clients
    llm.model, llm.fallback = groq, gemini
    return llm


def answer(llm):
    return "".join(llm.stream("q", [], "octo/repo"))


class AnswerModelTest(unittest.TestCase):
    def test_groq_answers(self):
        groq, gemini = FakeModel(["Hel", "lo"]), FakeModel(["no"])
        self.assertEqual(answer(make_llm(groq, gemini)), "Hello")
        self.assertEqual(gemini.calls, 0)

    def test_gemini_answers_when_groq_is_refused(self):
        groq, gemini = FakeModel(["x"], fail_after=0), FakeModel(["From ", "Gemini"])
        self.assertEqual(answer(make_llm(groq, gemini)), "From Gemini")

    def test_gemini_answers_when_groq_returns_nothing(self):
        groq, gemini = FakeModel([""]), FakeModel(["Gemini"])
        self.assertEqual(answer(make_llm(groq, gemini)), "Gemini")

    def test_a_started_answer_is_never_restarted(self):
        groq, gemini = FakeModel(["Hel", "lo"], fail_after=1), FakeModel(["Gemini"])
        got = []
        with self.assertRaises(RuntimeError):
            for word in make_llm(groq, gemini).stream("q", [], "octo/repo"):
                got.append(word)
        self.assertEqual(got, ["Hel"])
        self.assertEqual(gemini.calls, 0)

    def test_both_failing_reports_the_error(self):
        groq = FakeModel(["x"], fail_after=0)
        gemini = FakeModel(["x"], fail_after=0, error=RuntimeError("503 UNAVAILABLE"))
        with self.assertRaisesRegex(RuntimeError, "503"):
            answer(make_llm(groq, gemini))

    def test_generate_falls_back_too(self):
        class Refusing:
            def invoke(self, prompt):
                raise RuntimeError("429 rate_limit_exceeded")

        class Answering:
            def invoke(self, prompt):
                return Chunk("Gemini answer")

        llm = make_llm(Refusing(), Answering())
        self.assertEqual(llm.generate("q", [], "octo/repo"), "Gemini answer")


if __name__ == "__main__":
    unittest.main()
