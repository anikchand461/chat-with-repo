<div align="center">

<img src="frontend/assets/logo-chatwithrepo.png" alt="Chat With Repo logo" width="180"/>

# Chat With Repo

### Point it at any GitHub repository. Ask it anything. Get answers grounded in the actual code.

[![Python](https://img.shields.io/badge/python-3.11+-3776AB?style=for-the-badge&logo=python&logoColor=white)](https://www.python.org/)
[![FastAPI](https://img.shields.io/badge/FastAPI-backend-009688?style=for-the-badge&logo=fastapi&logoColor=white)](https://fastapi.tiangolo.com/)
[![LangChain](https://img.shields.io/badge/LangChain-RAG-1C3C3C?style=for-the-badge&logo=langchain&logoColor=white)](https://www.langchain.com/)
[![AWS Lambda](https://img.shields.io/badge/AWS-Lambda-FF9900?style=for-the-badge&logo=awslambda&logoColor=white)](https://aws.amazon.com/lambda/)
[![Go](https://img.shields.io/badge/Go-Android-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://gioui.org/)
[![Status](https://img.shields.io/badge/status-active-17b57f?style=for-the-badge)](#)

**[Live Demo](https://chatwithrepo-nine.vercel.app/) · [Android App](https://github.com/shreyaghorui222004/chat-with-repo/releases/latest) · [Problem](#-the-problem) · [Solution](#-the-solution) · [RAG Pipeline](#-rag-pipeline) · [Architecture](#-full-architecture) · [Installation](#-installation) · [Deployment](#-deployment)**

</div>

---

## ◈ The Problem

```text
clone repo  →  open 40 tabs  →  grep for the entry point  →  still lost
```

New repo, no map. The usual tools don't get you unstuck:

- **`grep` finds words, not ideas.** It won't find "where retries happen" if the code calls it `backoff`.
- **READMEs go stale.** They describe the plan, not the code.
- **Pasting files into a chatbot** means guessing which files matter *before* you understand the project.
- **Reading file by file** works for ten files, not a thousand.

---

## ◈ The Solution

**Chat With Repo** reads the whole repository first, then lets you just ask.

| | |
|---|---|
| 🔎 **Understands the question** | Each question is rephrased, searched several ways and reranked, so "how does auth work" and "where's the JWT checked" land on the same code. |
| 🗺️ **Never loses the big picture** | The file tree and README go along with every question, so "explain this project" always works. |
| 📎 **Shows its sources** | Every answer lists the files it used, one click away on GitHub. |
| 📄 **Ask for a file, get the file** | Exact file contents straight from the repo, no paraphrasing. |
| 🔗 **Just paste a URL** | No cloning, uploading or picking files. |
| 📱 **Web and Android** | Both use the same backend and show the same answers. |
| ⚡ **Live progress, live answers** | Watch indexing happen in real time; answers stream in word by word. |

**Try it now → [chatwithrepo-nine.vercel.app](https://chatwithrepo-nine.vercel.app/)**

---

## ◈ RAG Pipeline

A multi-stage retrieval pipeline, not a single embed-and-search step:

<img src="frontend/assets/rag-pipeline.png" alt="Chat With Repo RAG pipeline"/>

| Stage | Model / Method | Purpose |
|---|---|---|
| Query classification | Keyword / length heuristic (no LLM call) | Routes the question as `lookup` (file / function / usage) or `analysis` (architecture / design) |
| Multi-query generation | Groq `openai/gpt-oss-20b` | Rephrases the question to widen recall (skipped for lookups) |
| Embedding & retrieval | Cohere `embed-v4.0` + Qdrant | Vector search over the chunked repository |
| Fusion | Reciprocal Rank Fusion | Merges the results of all queries into one ranking |
| Reranking | Cohere `rerank-v3.5` | Re-scores the fused results against the original question |
| Repository overview | Built from the repo data | Name, description, file tree and README sent with **every** question |
| Answer generation | Gemini `3.1-flash-lite` | Streams the answer from the overview plus the top chunks |

### Indexing

**GitHub API → Loader → Converter → Chunker → Embeddings → Qdrant**

- Lockfiles and generated files (`package-lock.json`, `go.sum`, `*.min.js`, …) are skipped, for any language.
- Every chunk carries its file path, and repository-level summary documents help with "explain this codebase" questions.
- Embedding runs in paced batches and waits out rate limits instead of failing.
- One Qdrant collection per chat, so an index survives restarts and redeploys.
- Indexing runs in the background with live progress (fetching, then a chunk-by-chunk progress bar). The chat opens only once it's ready, and a failed chat is retried the next time you open it.

### Streaming & response time

Answers stream token by token over server-sent events (`/chat/{id}/ask/stream`), on web and Android. Each answer shows its timing (*Responded in 4.2s · first word 1.8s*), measured by the server and stored with the message, so every device shows the same value.

---

## ◈ Full Architecture

<img src="frontend/assets/architecture.png" alt="Chat With Repo architecture"/>

### Where data lives

| Data | Stored in | Notes |
|---|---|---|
| Users, chats, messages, daily usage | Neon Postgres | `users`, `chats`, `messages`, `daily_usage` |
| Indexing progress | Neon Postgres (AWS) · memory (Render) | `index_progress`, so the Lambda can follow a Fargate job |
| Parsed repository JSON | Neon Postgres (`repository_json`, JSONB) | Survives restarts; restored to local disk when missing |
| Repository JSON, working copy | Local disk (`DATA_DIR`) | Fast cache while the server runs; Amazon S3 shares it between AWS instances |
| Vectors | Qdrant Cloud | One collection per chat |

**Repository JSON lifecycle.** When a repository is downloaded, its JSON is written to local disk *and* saved to Neon. Questions read the local file. After a restart wipes the disk (Render's free tier, a new Lambda instance), the file is restored from S3 (AWS) or Neon on the first question, with no GitHub download and no re-embedding. Files over `REPO_JSON_DB_MAX_MB` (default 25 MB) aren't saved to Neon and are downloaded from GitHub again after a restart.

### Plans

| | Free | Pro |
|---|---|---|
| Repository chats | 2 in total | Unlimited |
| Questions | 10 per day | Unlimited |
| Private repositories | — | ✓ |
| Re-index on demand | — | ✓ |

Upgrades go through a Dodo Payments checkout and webhook.

---

## ◈ Mobile App

A native Android client built with **Go + Gio**, talking to the same FastAPI backend.

- Login and registration with bearer-token authentication
- Repository chats with live, token-by-token answers
- Live indexing progress with automatic retry, same as the web dashboard
- Response times synced with the web
- Profile screen to check and save a personal GitHub token
- Links to the web manual and site

**[Download ChatWithRepo Android v1.3.0](https://github.com/shreyaghorui222004/chat-with-repo/releases/tag/v1.3.0)** — the release contains `ChatWithRepo.apk`.

> Android may ask you to allow installation from unknown sources.

---

## ◈ Installation

### 1 · Clone and install

```bash
git clone https://github.com/anikchand461/chat-with-repo.git
cd chat-with-repo
uv sync                        # or: pip install -r requirements.txt
```

### 2 · Configure

```bash
cp .env.example .env
```

```env
GOOGLE_API_KEY=google_api_key
COHERE_API_KEY=cohere_api_key
GROQ_API_KEY=groq_api_key
QDRANT_URL=https://your-cluster-id.region.aws.cloud.qdrant.io
QDRANT_API_KEY=qdrant_api_key
DATABASE_URL=postgresql+psycopg://USER:PASSWORD@YOUR-NEON-HOST/neondb?sslmode=require
JWT_SECRET=long_random_string    # must be identical on every backend (AWS + Render)
DODO_API_KEY=dodopayments_api_key
DODO_PRODUCT_ID=dodo_product_id
DODO_BASE_URL=https://test.dodopayments.com
APP_URL=http://127.0.0.1:5500/

# Optional
# GITHUB_TOKEN=github_token      # server-wide fallback for users without their own token
# DATA_DIR=/path/to/data         # local repo JSON cache (default: <project>/data)
# REPO_JSON_DB_MAX_MB=25         # largest repo JSON saved to Neon
# EMBED_TOKENS_PER_MINUTE=80000  # embedding pacing; 0 with a production Cohere key
```

> **GitHub token.** Without `GITHUB_TOKEN`, each user saves their own token on the Profile page, because GitHub's anonymous limit (60 requests/hour) isn't enough to index most repositories.
>
> **Cohere trial keys** allow 40 calls and 100k embedding tokens per minute, so large repositories take a couple of minutes to index.

### 3 · Run the backend

```bash
uv run uvicorn backend.app:app --reload --reload-dir backend
```

API at `http://127.0.0.1:8000`, docs at `/docs`. Keep `--reload-dir backend`: indexing writes to `data/`, and watching the whole project would restart the server mid-request.

### 4 · Open the frontend

```bash
cd frontend && python -m http.server 5500
```

Visit `http://127.0.0.1:5500/index.html`. It uses the deployed backends by default; set `USE_LOCAL_API = true` in `frontend/js/app.js` to use your local one (and set it back before committing).

### Tests

```bash
node --test frontend/tests/                              # AWS → Render failover (Node 18+)
.venv/bin/python -m unittest backend.tests.test_aws_backend   # backend: SQLite + fake AWS, no network
```

### Android development

The app uses the Render backend (`baseURL` in `mobile/main.go`). For a local backend use `127.0.0.1:8000` (desktop) or `10.0.2.2:8000` (emulator). `manualURL` / `webURL` in `mobile/ui/auth.go` point at the web frontend. Switch all of them back before releasing.

---

## ◈ Deployment

| Part | Hosted on | URL |
|---|---|---|
| Frontend | Vercel | [chatwithrepo-nine.vercel.app](https://chatwithrepo-nine.vercel.app/) |
| Backend · **primary** | AWS Lambda (container) + Function URL with response streaming | `https://qettzfl73r7xhbres3oqvx3qt40mmqrp.lambda-url.ap-south-1.on.aws` |
| Indexing | AWS ECS Fargate, one task per job | started by the Lambda |
| Repo JSON cache | Amazon S3 (AWS) · Neon `repository_json` (both) | |
| Backend · **fallback** | Render | `https://chat-with-repo-4vwy.onrender.com` |
| Database | Neon Postgres | shared by both backends |
| Vectors | Qdrant Cloud | shared by both backends |

Both backends run the **same code** against the **same database and Qdrant cluster**, so users, logins, chats and history are identical whichever one answers. The AWS-only behaviour (Fargate indexing, S3, progress in Postgres) is switched on by environment variables that only the AWS stack sets.

### Request flow

```text
Browser (Vercel)
  │  apiFetch() — frontend/js/app.js
  │
  ├─①─► AWS Lambda  (Function URL, RESPONSE_STREAM)
  │       Lambda Web Adapter → uvicorn → FastAPI
  │       new chat → ECS Fargate task → GitHub → S3 + Neon → Qdrant
  │
  │     no headers within 10 s · connection error · 502 / 503 / 504
  │
  └─②─► Render  (same FastAPI code, indexes in-process)

        both → Neon · Qdrant · GitHub · Cohere · Groq · Gemini
```

### Failover rules

1. **AWS first.** Normal answers, including 4xx errors, are used as they are. No extra delay while AWS is up.
2. **10-second failover.** No response headers within 10 s, a connection error, or 502/503/504 → the same request goes to Render. The limit applies only until the response starts, so long streams are never cut off.
3. **Remembered for 5 minutes.** After a failover, requests go straight to Render for 5 minutes (`localStorage`); each page load re-checks AWS in the background.
4. **Never run twice.** Requests that change something (register, create chat, ask, checkout) first check whether AWS is alive and wait for it rather than sending a second copy. They never fail over on a 504.
5. **Streams are never restarted.** A stream that breaks part-way shows an error instead of a duplicate answer from Render.
6. **Leaving a page isn't an outage.** Requests cancelled by navigation don't mark AWS down.

*Try it locally:* serve the frontend on port 5500 and block the `lambda-url` domain in DevTools (Network → request blocking). Afterwards, unblock it and delete `devlens_api_primary_down_until` from Local Storage.

### AWS (primary)

Defined in `template.yaml` (AWS SAM) and deployed by `deploy-aws.sh` as the stack `chatwithrepo` in `ap-south-1`.

| Resource | Role |
|---|---|
| `ApiContainerFunction` | The API: container image (`Dockerfile`) running uvicorn behind [Lambda Web Adapter](https://github.com/awslabs/aws-lambda-web-adapter); arm64, 2 GB, 15 min timeout. Its Function URL streams answers with no 30-second API Gateway limit. |
| `IndexCluster` · `IndexTaskDefinition` | Fargate task that indexes one repository (`python -m backend.jobs.index_chat`), same image as the API. Public subnets with outbound-only access, so no NAT gateway. |
| `RepoCacheBucket` | Private, encrypted S3 bucket for repo JSON (`repos/<owner>_<repo>_<branch>.json`), shared by Fargate and the Lambda. |
| `index_progress` table | Indexing progress in Postgres (`PROGRESS_STORE=db`). A job silent for 10 minutes is reported as failed. |
| `FastAPIApp` + HTTP API | Earlier zip Lambda behind API Gateway (`e77kyrzage.execute-api…`). Kept as a backup, not used by the frontend (no streaming, 30 s limit). |

**Deploy** (Docker, AWS CLI, SAM CLI, `uv`):

```bash
./deploy-aws.sh --no-execute-changeset   # build + push the image, create a changeset to review
./deploy-aws.sh                          # build + push + deploy (asks before applying)
```

The script builds the arm64 image, pushes it to ECR `chatwithrepo-api` (last 3 kept), deploys, and sends a warm-up request. AWS-only dependencies are in `requirements-aws.txt`.

**Configuration.** Non-secret settings live in `samconfig.toml` (gitignored). Secrets (`DATABASE_URL`, `JWT_SECRET`, API keys) are `NoEcho` stack parameters that keep their values across deploys; change one in the CloudFormation console (*Update → Use current template*).

**Logs.** CloudWatch `/aws/lambda/chatwithrepo-api-container` (API) and `/ecs/chatwithrepo-index` (indexing).

**Timings.** A new Lambda instance starts in ~5 s and its first question takes ~15–25 s while the RAG libraries load; later questions are fast. Indexing a small repository takes ~35 s including the Fargate start.

### Render (fallback)

| | |
|---|---|
| Build | `pip install -r requirements.txt` |
| Start | `uvicorn backend.app:app --host 0.0.0.0 --port $PORT` |

Render indexes in-process and keeps progress in memory, so run a **single** worker. The free tier sleeps after ~15 minutes idle; an uptime monitor pinging `/` every 10 minutes keeps it awake. Repo JSON is restored from Neon after a restart.

### Keeping both backends in sync

- **Same environment variables**, especially `JWT_SECRET` (or logins from one backend are rejected by the other), `DATABASE_URL` and `QDRANT_URL`.
- **Same commit on both.** Allowed frontend origins (CORS) are in `backend/app.py`.
- **Payments.** The Dodo webhook points at one backend; `APP_URL` is where users return after checkout.

### Known limitations

- **Indexing during a failover.** Progress lives in Postgres on AWS but in memory on Render, so Render may start indexing a chat AWS is already indexing.
- **Android** uses Render only, without the failover.

---

## ◈ Tech Stack

<div align="center">

![Python](https://img.shields.io/badge/Python-3776AB?style=flat-square&logo=python&logoColor=white)
![FastAPI](https://img.shields.io/badge/FastAPI-009688?style=flat-square&logo=fastapi&logoColor=white)
![JWT](https://img.shields.io/badge/JWT-Authentication-000000?style=flat-square&logo=jsonwebtokens&logoColor=white)
![LangChain](https://img.shields.io/badge/LangChain-1C3C3C?style=flat-square&logo=langchain&logoColor=white)

![Groq](https://img.shields.io/badge/Groq-GPT--OSS%2020B-F55036?style=flat-square)
![Cohere](https://img.shields.io/badge/Cohere-Embed%20v4%20%7C%20Rerank%20v3.5-39594D?style=flat-square)
![Gemini](https://img.shields.io/badge/Gemini-Flash--Lite-4285F4?style=flat-square&logo=googlegemini&logoColor=white)

![Qdrant](https://img.shields.io/badge/Qdrant-Vector%20Store-DC244C?style=flat-square)
![Neon](https://img.shields.io/badge/Neon-Postgres%20%2B%20JSONB-00E699?style=flat-square&logo=neondatabase&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-4169E1?style=flat-square&logo=postgresql&logoColor=white)

![AWS Lambda](https://img.shields.io/badge/AWS-Lambda-FF9900?style=flat-square&logo=awslambda&logoColor=white)
![AWS Fargate](https://img.shields.io/badge/AWS-Fargate-FF9900?style=flat-square&logo=amazonecs&logoColor=white)
![Amazon S3](https://img.shields.io/badge/Amazon-S3-569A31?style=flat-square&logo=amazons3&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white)
![Render](https://img.shields.io/badge/Render-46E3B7?style=flat-square&logo=render&logoColor=black)
![Vercel](https://img.shields.io/badge/Vercel-000000?style=flat-square&logo=vercel&logoColor=white)

![HTML5](https://img.shields.io/badge/HTML5-E34F26?style=flat-square&logo=html5&logoColor=white)
![CSS3](https://img.shields.io/badge/CSS3-1572B6?style=flat-square&logo=css3&logoColor=white)
![JavaScript](https://img.shields.io/badge/JavaScript-F7DF1E?style=flat-square&logo=javascript&logoColor=black)
![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)
![Gio](https://img.shields.io/badge/Gio-Android-00ADD8?style=flat-square)

![GitHub](https://img.shields.io/badge/GitHub-API-181717?style=flat-square&logo=github&logoColor=white)
![Dodo Payments](https://img.shields.io/badge/Dodo-Payments-9BE000?style=flat-square)

</div>

---

<div align="center">

Built by [anikchand461](https://github.com/anikchand461)
and [shreyaghorui222004](https://github.com/shreyaghorui222004)

</div>
