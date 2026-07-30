<div align="center">

<img src="assets/logo.png" alt="Chat With Repo logo" width="180"/>

# Chat With Repo

### Point it at any GitHub repository. Ask it anything. Get answers grounded in the actual code.

[![Live Demo](https://img.shields.io/badge/live%20demo-chatwithrepo.vercel.app-17b57f?style=for-the-badge&logo=vercel&logoColor=white)](https://chatwithrepo.vercel.app)
[![Python](https://img.shields.io/badge/python-3.11+-3776AB?style=for-the-badge&logo=python&logoColor=white)](https://www.python.org/)
[![FastAPI](https://img.shields.io/badge/FastAPI-backend-009688?style=for-the-badge&logo=fastapi&logoColor=white)](https://fastapi.tiangolo.com/)
[![LangChain](https://img.shields.io/badge/LangChain-RAG-1C3C3C?style=for-the-badge&logo=langchain&logoColor=white)](https://www.langchain.com/)
[![Status](https://img.shields.io/badge/status-active-17b57f?style=for-the-badge)](#)

**[Live Demo](https://chatwithrepo.vercel.app) · [Problem](#-the-problem) · [RAG Pipeline](#-rag-pipeline) · [Architecture](#-full-architecture) · [Install](#-installation)**

</div>

---

## ◈ The Problem

```
clone repo  →  open 40 tabs  →  grep for the entry point  →  still lost
```

READMEs go stale, comments lie, and keyword search finds the word, not the concept. **Chat With Repo** indexes any public GitHub repository into a real retrieval pipeline, so you can ask questions and get answers grounded in the actual files — not a guess.

**Try it now → [chatwithrepo.vercel.app](https://chatwithrepo.vercel.app)**

---

## ◈ RAG Pipeline

This is the core of the project — a multi-stage retrieval pipeline, not a single embed-and-search step:

```mermaid
flowchart LR
    Q["User Question"] --> QC["Query Classifier<br/>Groq · llama-3.1-8b-instant<br/>lookup vs analysis"]
    QC --> MQ["Multi-Query Generator<br/>Groq · llama-3.1-8b-instant<br/>+2 rephrasings"]
    MQ --> R["Retriever<br/>Cohere embed-v4.0 + Chroma"]
    R --> RRF["Reciprocal Rank Fusion<br/>merges ranked result sets"]
    RRF --> RR["Reranker<br/>Cohere rerank-v3.5"]
    RR --> LLM["Answer Generation<br/>Gemini 3.1 Flash-Lite"]
    LLM --> A["Grounded Answer"]
```

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

```mermaid
flowchart TB
    subgraph Frontend["Frontend — HTML / CSS / JS"]
        A1["Login / Register"]
        A2["Dashboard"]
        A3["Chat UI"]
        A4["Profile / Billing"]
    end

    subgraph API["FastAPI Backend"]
        B1["Auth Router — JWT"]
        B2["Repository Router"]
        B3["Chat Router"]
        B4["Payment Router — Dodo"]
    end

    subgraph Ingest["Repository Ingestion"]
        C1["GitHub Client — tree + files"]
        C2["JSON Writer — data/*.json"]
    end

    subgraph RAG["RAG Pipeline"]
        D1["Loader → Converter → Chunker"]
        D2["Vector Store — Cohere embeddings"]
        D3["Query Classifier"]
        D4["Multi-Query Generator"]
        D5["Retriever → RRF → Reranker"]
        D6["LLM — Gemini"]
    end

    subgraph Data["Persistence"]
        E1[("SQL — users, chats, messages")]
        E2[("Vector DB — Chroma")]
    end

    A1 --> B1
    A2 --> B2
    A3 --> B3
    A4 --> B4

    B2 --> C1 --> C2 --> D1 --> D2 --> E2
    B3 --> D3 --> D4 --> D5 --> D6 --> B3

    B1 --> E1
    B3 --> E1
    B4 --> E1
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
JWT_SECRET=your_jwt_secret
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

![FastAPI](https://img.shields.io/badge/FastAPI-009688?style=flat-square&logo=fastapi&logoColor=white)
![LangChain](https://img.shields.io/badge/LangChain-1C3C3C?style=flat-square&logo=langchain&logoColor=white)
![Cohere](https://img.shields.io/badge/Cohere-embed%20%2B%20rerank-39594D?style=flat-square)
![Gemini](https://img.shields.io/badge/Gemini-answering-4285F4?style=flat-square&logo=googlegemini&logoColor=white)
![Groq](https://img.shields.io/badge/Groq-classifier%20%2B%20multiquery-F55036?style=flat-square)
![Chroma](https://img.shields.io/badge/Chroma-vector%20store-purple?style=flat-square)
![Vercel](https://img.shields.io/badge/Vercel-deployed-000000?style=flat-square&logo=vercel&logoColor=white)

</div>

---

<div align="center">

Built by [anikchand461](https://github.com/anikchand461)
and [shreyaghorui222004](https://github.com/shreyaghorui222004)

</div>