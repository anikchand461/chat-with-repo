<div align="center">

<img src="frontend/assets/logo-chatwithrepo.png" alt="Chat With Repo logo" width="180"/>

# Chat With Repo

### Point it at any GitHub repository. Ask it anything. Get answers grounded in the actual code.

[![Python](https://img.shields.io/badge/python-3.11+-3776AB?style=for-the-badge&logo=python&logoColor=white)](https://www.python.org/)
[![FastAPI](https://img.shields.io/badge/FastAPI-backend-009688?style=for-the-badge&logo=fastapi&logoColor=white)](https://fastapi.tiangolo.com/)
[![LangChain](https://img.shields.io/badge/LangChain-RAG-1C3C3C?style=for-the-badge&logo=langchain&logoColor=white)](https://www.langchain.com/)
[![Status](https://img.shields.io/badge/status-active-17b57f?style=for-the-badge)](#)

**[Live Demo](https://chatwithrepo-nine.vercel.app/) · [Problem](#-the-problem) · [RAG Pipeline](#-rag-pipeline) · [Architecture](#-full-architecture) · [Install](#-installation)**

</div>

---

## ◈ The Problem

```
clone repo  →  open 40 tabs  →  grep for the entry point  →  still lost
```

READMEs go stale, comments lie, and keyword search finds the word, not the concept. **Chat With Repo** indexes any public GitHub repository into a real retrieval pipeline, so you can ask questions and get answers grounded in the actual files — not a guess.

**Try it now → [chatwithrepo-nine.vercel.app](https://chatwithrepo-nine.vercel.app/)**

---

## ◈ RAG Pipeline

This is the core of the project — a multi-stage retrieval pipeline, not a single embed-and-search step:

<img src="frontend/assets/rag-pipeline.png" alt="Chat With Repo logo"/>

| Stage | Model / Method | Purpose |
|---|---|---|
| Query Classification | Groq — `llama-3.1-8b-instant` | Routes the question as `lookup` (file/function/usage) or `analysis` (architecture/design) |
| Multi-Query Generation | Groq — `llama-3.1-8b-instant` | Rephrases the question into 2 alternate search queries to widen recall |
| Embedding & Retrieval | Cohere `embed-v4.0` + Chroma | Vector search over chunked repo documents |
| Fusion | Reciprocal Rank Fusion (RRF) | Merges results from the original + generated queries into one ranked list |
| Reranking | Cohere `rerank-v3.5` | Re-scores the fused results against the original question for precision |
| Answer Generation | Gemini `3.1-flash-lite` | Synthesizes the final answer from the top reranked chunks |

Before any of this, ingestion turns a raw repo into searchable documents: **GitHub API → Loader → Converter → Chunker → Embeddings → Vector Store**, plus synthetic repository-level summary documents to improve high-level "explain this codebase" queries.

---

## ◈ Full Architecture
 
<img src="frontend/assets/architecture.png" alt="Chat With Repo logo"/>

Free vs Pro is enforced per-user (monthly repo limit, chat history depth); upgrades run through a Dodo Payments checkout + webhook flow.
```

**Free vs Pro** is enforced per-user (monthly repo limit, chat history depth); upgrades run through a Dodo Payments checkout + webhook flow.

---

## ◈ Installation

**Step 1 — Clone and install dependencies**

```bash
git clone https://github.com/anikchand461/chat-with-repo.git
cd chat-with-repo
uv sync
```

**Step 2 — Configure environment variables**

```bash
cp .env.example .env
```

```env
GROQ_API_KEY=your_groq_key        # classifier + multi-query
GOOGLE_API_KEY=your_gemini_key    # answer generation
COHERE_API_KEY=your_cohere_key    # embeddings + reranking
DODO_API_KEY=your_dodo_key        # optional, Pro plan
```

**Step 3 — Run the backend**

```bash
uv run uvicorn backend.app:app --reload
```

API comes up at `http://127.0.0.1:8000` — docs at `/docs`.

**Step 4 — Open the frontend**

```bash
cd frontend
python -m http.server 5500
```

Visit `http://127.0.0.1:5500/index.html`.

---

## ◈ Tech Stack

<div align="center">

![Python](https://img.shields.io/badge/Python-3776AB?style=flat-square&logo=python&logoColor=white)
![FastAPI](https://img.shields.io/badge/FastAPI-009688?style=flat-square&logo=fastapi&logoColor=white)
![JWT](https://img.shields.io/badge/JWT-Authentication-000000?style=flat-square&logo=jsonwebtokens&logoColor=white)
![LangChain](https://img.shields.io/badge/LangChain-1C3C3C?style=flat-square&logo=langchain&logoColor=white)

![Groq](https://img.shields.io/badge/Groq-Llama%203.1%208B%20Instant-F55036?style=flat-square)
![Cohere](https://img.shields.io/badge/Cohere-Embed%20v4%20%7C%20Rerank%20v3.5-39594D?style=flat-square)
![Gemini](https://img.shields.io/badge/Gemini-Flash--Lite-4285F4?style=flat-square&logo=googlegemini&logoColor=white)

![ChromaDB](https://img.shields.io/badge/ChromaDB-Vector%20Store-8A2BE2?style=flat-square)
![Neon](https://img.shields.io/badge/Neon-Postgres-00E699?style=flat-square&logo=neondatabase&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-4169E1?style=flat-square&logo=postgresql&logoColor=white)

![GitHub](https://img.shields.io/badge/GitHub-API-181717?style=flat-square&logo=github&logoColor=white)
![Dodo%20Payments](https://img.shields.io/badge/Dodo-Payments-9BE000?style=flat-square)

![HTML5](https://img.shields.io/badge/HTML5-E34F26?style=flat-square&logo=html5&logoColor=white)
![CSS3](https://img.shields.io/badge/CSS3-1572B6?style=flat-square&logo=css3&logoColor=white)
![JavaScript](https://img.shields.io/badge/JavaScript-F7DF1E?style=flat-square&logo=javascript&logoColor=black)

![Render](https://img.shields.io/badge/Render-46E3B7?style=flat-square&logo=render&logoColor=black)
![Vercel](https://img.shields.io/badge/Vercel-000000?style=flat-square&logo=vercel&logoColor=white)

</div>

---

<div align="center">

Built by [anikchand461](https://github.com/anikchand461)
and [shreyaghorui222004](https://github.com/shreyaghorui222004)

</div>
