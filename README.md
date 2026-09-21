<div align="center">

<img src="frontend/assets/logo-chatwithrepo.png" alt="Chat With Repo logo" width="180"/>

# Chat With Repo

### Point it at any GitHub repository. Ask it anything. Get answers grounded in the actual code.

[![Python](https://img.shields.io/badge/python-3.11+-3776AB?style=for-the-badge&logo=python&logoColor=white)](https://www.python.org/)
[![FastAPI](https://img.shields.io/badge/FastAPI-backend-009688?style=for-the-badge&logo=fastapi&logoColor=white)](https://fastapi.tiangolo.com/)
[![LangChain](https://img.shields.io/badge/LangChain-RAG-1C3C3C?style=for-the-badge&logo=langchain&logoColor=white)](https://www.langchain.com/)
[![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Gio](https://img.shields.io/badge/Gio-Android-00ADD8?style=for-the-badge)](https://gioui.org/)
[![Status](https://img.shields.io/badge/status-active-17b57f?style=for-the-badge)](#)

**[Live Demo](https://chatwithrepo-nine.vercel.app/) · [Android App](https://github.com/shreyaghorui222004/chat-with-repo/releases/tag/v1.2.0) · [Problem](#-the-problem) · [RAG Pipeline](#-rag-pipeline) · [Architecture](#-full-architecture) · [Installation](#-installation)**

</div>

---

## ◈ The Problem

```text
clone repo  →  open 40 tabs  →  grep for the entry point  →  still lost
```

READMEs go stale, comments lie, and keyword search finds the word, not the concept. **Chat With Repo** indexes any public GitHub repository into a real retrieval pipeline, so you can ask questions and get answers grounded in the actual files — not a guess.

**Try it now → [chatwithrepo-nine.vercel.app](https://chatwithrepo-nine.vercel.app/)**

---

## ◈ RAG Pipeline

This is the core of the project — a multi-stage retrieval pipeline, not a single embed-and-search step:

<img src="frontend/assets/rag-pipeline.png" alt="Chat With Repo RAG pipeline"/>

| Stage | Model / Method | Purpose |
|---|---|---|
| Query Classification | Keyword / length heuristic (no LLM call) | Routes the question as `lookup` (file/function/usage) or `analysis` (architecture/design) |
| Multi-Query Generation | Groq — `openai/gpt-oss-20b` | Rephrases the question into alternate search queries to widen recall (skipped for lookups) |
| Embedding & Retrieval | Cohere `embed-v4.0` + Chroma | Vector search over chunked repo documents |
| Fusion | Reciprocal Rank Fusion (RRF) | Merges results from the original + generated queries into one ranked list |
| Reranking | Cohere `rerank-v3.5` | Re-scores the fused results against the original question for precision |
| Repository Overview | Built from the repo data | Name, description, topics, full file tree and README are sent with **every** question, so broad questions never depend on what retrieval happened to return |
| Answer Generation | Gemini `3.1-flash-lite` | Streams the final answer from the overview plus the top reranked chunks |

Before any of this, ingestion turns a raw repo into searchable documents:

**GitHub API → Loader → Converter → Chunker → Embeddings → Vector Store**

- Lockfiles and generated files (`package-lock.json`, `yarn.lock`, `go.sum`, `*.min.js`, …) are skipped — language-agnostic, so any stack works.
- Every chunk is prefixed with its file path so it stays meaningful on its own.
- Synthetic repository-level summary documents are also generated to improve high-level "explain this codebase" queries.
- Embedding runs in paced batches and waits out provider rate limits instead of failing.
- If a chat's index is missing or empty (new machine, wiped disk), it is rebuilt automatically on the first question.

### Streaming & response time

Answers stream in token by token (server-sent events on `/chat/{id}/ask/stream`), on the web and in the Android app. Under every answer you get the time it took — *Responded in 4.2s · first word 1.8s*. The server measures and stores it with the message, so the web and mobile apps always show the same value for the same answer.

---

## ◈ Full Architecture

<img src="frontend/assets/architecture.png" alt="Chat With Repo architecture"/>

Free vs Pro is enforced per-user (monthly repo limit, chat history depth); upgrades run through a Dodo Payments checkout + webhook flow.

---

## ◈ Mobile App

Chat With Repo also has a native Android application built with **Go + Gio**.

The Android app acts as a mobile client for the existing Chat With Repo backend. The mobile UI communicates with the deployed backend through API endpoints, keeping the mobile frontend and backend separate.

### Mobile Features

- Login and registration
- Repository chat with live, token-by-token streaming answers
- Response time under every answer, synced with the web
- Repository management
- Bearer-token authentication
- Native Android UI
- Connects to the existing FastAPI backend

### Mobile Tech Stack

- **Go**
- **Gio UI**
- **Android**
- **FastAPI REST API**

### Android Release

**[Download ChatWithRepo Android v1.2.0](https://github.com/shreyaghorui222004/chat-with-repo/releases/tag/v1.2.0)**

The release contains `ChatWithRepo.apk`.

To install it, download the APK from the release page and install it on your Android device.

> Android may ask you to allow installation from unknown sources.

---

## ◈ Installation

### Web / Backend

**Step 1 — Clone and install dependencies**

```bash
git clone https://github.com/anikchand461/chat-with-repo.git
cd chat-with-repo
uv sync                        # or: pip install -r requirements.txt
```

**Step 2 — Configure environment variables**

```bash
cp .env.example .env
```

```env
GOOGLE_API_KEY=google_api_key
COHERE_API_KEY=cohere_api_key
GROQ_API_KEY=groq_api_key
DODO_API_KEY=dodopayments_api_key
DODO_PRODUCT_ID=dodo_product_id
APP_URL=http://127.0.0.1:5500/
DODO_BASE_URL=https://test.dodopayments.com
DATABASE_URL=postgresql+psycopg://neondb_owner:YOUR_PASSWORD@ep-cool-heart-azo2wz85-pooler.c-3.ap-southeast-1.aws.neon.tech/neondb?sslmode=require

# Optional
GITHUB_TOKEN=github_token      # server-wide fallback; raises the GitHub API limit from 60 to 5000/hour
DATA_DIR=/path/to/data         # where downloaded repos are stored (default: <project>/data)
CHROMA_DIR=/path/to/chroma_db  # where vector indexes are stored (default: <project>/chroma_db)
EMBED_TOKENS_PER_MINUTE=80000  # embedding pacing; set 0 with a production Cohere key
```

> **Cohere trial keys** are limited to 40 calls and 100k embedding tokens per minute, so indexing a large repo takes a couple of minutes. A production key removes the limit.
>
> **Hosting note:** indexes live on disk. On hosts with a temporary disk (e.g. Render's free tier) they are rebuilt automatically after a redeploy; attach a persistent disk and point `DATA_DIR` / `CHROMA_DIR` at it to avoid that.

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

### Android

To develop against a local backend, change `baseURL` in `mobile/main.go` (`127.0.0.1:8000` for the desktop build, `10.0.2.2:8000` for the Android emulator) and switch it back before committing.

The Android APK is distributed through GitHub Releases rather than committed to the repository.

**[Download the latest Android release](https://github.com/shreyaghorui222004/chat-with-repo/releases/tag/v1.2.0)**

1. Download `ChatWithRepo.apk`.
2. Transfer it to your Android device if necessary.
3. Open the APK and install it.
4. If Android blocks the installation, allow installation from unknown sources.

---

## ◈ Tech Stack

<div align="center">

![Python](https://img.shields.io/badge/Python-3776AB?style=flat-square&logo=python&logoColor=white)
![FastAPI](https://img.shields.io/badge/FastAPI-009688?style=flat-square&logo=fastapi&logoColor=white)
![JWT](https://img.shields.io/badge/JWT-Authentication-000000?style=flat-square&logo=jsonwebtokens&logoColor=white)
![LangChain](https://img.shields.io/badge/LangChain-1C3C3C?style=flat-square&logo=langchain&logoColor=white)

![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)
![Gio](https://img.shields.io/badge/Gio-Android-00ADD8?style=flat-square)

![Groq](https://img.shields.io/badge/Groq-GPT--OSS%2020B-F55036?style=flat-square)
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

## ◈ Android Release

| Version | Platform | Download |
|---|---|---|
| v1.2.0 | Android | **[ChatWithRepo.apk](https://github.com/shreyaghorui222004/chat-with-repo/releases/tag/v1.2.0)** |

---

<div align="center">

Built by [anikchand461](https://github.com/anikchand461)
and [shreyaghorui222004](https://github.com/shreyaghorui222004)

</div>
