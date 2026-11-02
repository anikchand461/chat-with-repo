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

**[Live Demo](https://chatwithrepo-nine.vercel.app/) · [Android App](https://github.com/shreyaghorui222004/chat-with-repo/releases/latest) · [Problem](#-the-problem) · [Solution](#-the-solution) · [RAG Pipeline](#-rag-pipeline) · [Architecture](#-full-architecture) · [Installation](#-installation) · [Deployment](#-deployment)**

</div>

---

## ◈ The Problem

```text
clone repo  →  open 40 tabs  →  grep for the entry point  →  still lost
```

We've all been there. New repo, no map. So you reach for the usual tools — and none of them actually get you unstuck:

- `grep` finds words, not ideas. It won't find "where retries happen" if the code just calls it `backoff`.
- READMEs go stale the moment nobody updates them. They tell you the plan, not the codebase.
- Pasting files into a chatbot just moves the problem — now you have to guess which files matter *before* you understand the project.
- Reading file by file works for ten files. It falls apart at a thousand.

---

## ◈ The Solution

**Chat With Repo** skips the guesswork. Give it a repo, and it reads the whole thing first — then lets you just ask.

- **It doesn't just search, it understands the question.** Every question gets rephrased a few different ways, searched, and reranked — so "how does auth work" and "where's the JWT checked" both land on the same right answer.
- **It never forgets the big picture.** The file tree and README ride along with every question, so "explain this project" doesn't depend on luck.
- **It shows its work.** Every answer comes with the files it actually read, one click from the answer to the real code on GitHub.
- **Ask for a file, get the file.** No paraphrasing, no hallucinating — just the real contents, straight from the repo.
- **Just paste a URL.** No cloning, no uploading, no picking files yourself.
- **One app, two screens.** Web and Android run on the same backend, so you get the same answers either way.
- **Nothing happens in silence.** You watch it index in real time, and if something fails, it just quietly tries again.

**Try it now → [chatwithrepo-nine.vercel.app](https://chatwithrepo-nine.vercel.app/)**

---

## ◈ RAG Pipeline

This is the core of the project — a multi-stage retrieval pipeline, not a single embed-and-search step:

<img src="frontend/assets/rag-pipeline.png" alt="Chat With Repo RAG pipeline"/>

| Stage | Model / Method | Purpose |
|---|---|---|
| Query Classification | Keyword / length heuristic (no LLM call) | Routes the question as `lookup` (file/function/usage) or `analysis` (architecture/design) |
| Multi-Query Generation | Groq — `openai/gpt-oss-20b` | Rephrases the question into alternate search queries to widen recall (skipped for lookups) |
| Embedding & Retrieval | Cohere `embed-v4.0` + Qdrant | Vector search over chunked repo documents |
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
- Vectors live in Qdrant Cloud, one collection per chat, so an index survives backend restarts and redeploys — it isn't tied to local disk.
- Indexing runs in the background (on AWS as a separate Fargate task, so it isn't bound by Lambda's 15-minute limit): creating a chat (or reopening one whose index is missing) shows live progress on the dashboard — fetch stage, then a chunk-by-chunk progress bar — and only opens the chat once it's actually ready, instead of dropping you into an empty chat. A chat that failed to index (e.g. GitHub's rate limit) is retried automatically the next time you open it.

### Streaming & response time

Answers stream in token by token (server-sent events on `/chat/{id}/ask/stream`), on the web and in the Android app. Under every answer you get the time it took — *Responded in 4.2s · first word 1.8s*. The server measures and stores it with the message, so the web and mobile apps always show the same value for the same answer.

---

## ◈ Full Architecture

<img src="frontend/assets/architecture.png" alt="Chat With Repo architecture"/>

Free vs Pro is enforced per-user: the free plan is capped at 2 repository chats total (not monthly) and 10 questions/day, with a shorter conversation-history window sent to the LLM; Pro removes both caps and adds private repos and on-demand re-indexing. Upgrades run through a Dodo Payments checkout + webhook flow.

---

## ◈ Mobile App

Chat With Repo also has a native Android application built with **Go + Gio**.

The Android app acts as a mobile client for the existing Chat With Repo backend. The mobile UI communicates with the deployed backend through API endpoints, keeping the mobile frontend and backend separate.

### Mobile Features

- Login and registration
- Repository chat with live, token-by-token streaming answers
- Live "setting up your chat" progress while a repo is indexed — same fetch/index stages and progress bar as the web dashboard, with automatic retry for a chat that failed to index
- Response time under every answer, synced with the web
- Profile screen — check and save your personal GitHub access token
- Manual and Web links, opening the web app's user guide and site in the system browser
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

**[Download ChatWithRepo Android v1.3.0](https://github.com/shreyaghorui222004/chat-with-repo/releases/tag/v1.3.0)**

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
JWT_SECRET=long_random_string    # signs login tokens - must be identical on every backend (AWS + Render)

# Optional
# GITHUB_TOKEN=github_token     # server-wide fallback, applied to every user who hasn't saved their
                                 # own token. Intentionally left unset by default: without it, indexing
                                 # a repo requires each user to save their own token on the Profile page
                                 # first (see POST /auth/github-token) - GitHub's public API allows only
                                 # 60 requests/hour, not enough for most repos. Only set this if you want
                                 # every user to get a working default without configuring anything.
DATA_DIR=/path/to/data         # where downloaded repos (raw JSON) are stored (default: <project>/data)
QDRANT_URL=https://your-cluster-id.region.aws.cloud.qdrant.io  # Qdrant Cloud cluster endpoint
QDRANT_API_KEY=qdrant_api_key  # Qdrant Cloud API key
EMBED_TOKENS_PER_MINUTE=80000  # embedding pacing; set 0 with a production Cohere key
```

> **Cohere trial keys** are limited to 40 calls and 100k embedding tokens per minute, so indexing a large repo takes a couple of minutes. A production key removes the limit.
>
> **Vector storage:** vector indexes live in Qdrant Cloud (`QDRANT_URL` / `QDRANT_API_KEY`), one collection per chat, so they survive backend restarts and redeploys.
>
> **Hosting note:** the raw repo JSON (`DATA_DIR`) lives on local disk. On hosts with a temporary disk (e.g. Render's free tier), that file is lost on every spin-down, which triggers a GitHub re-fetch (but *not* a re-embed — the existing Qdrant vectors are reused) on the next question. Point `DATA_DIR` at a persistent disk to avoid even that. On AWS the file is also kept in S3 (`REPO_CACHE_BUCKET`), so it survives Lambda and Fargate restarts.

**Step 3 — Run the backend**

```bash
uv run uvicorn backend.app:app --reload --reload-dir backend
```

`--reload-dir backend` matters: without it, `--reload` watches the whole
project root by default, including `data/` and `chroma_db/` - both of which
the app itself writes to while indexing a repo. That triggers uvicorn to
restart mid-request (dropping whatever was in flight, e.g. a "network error"
on `/chat/create` a few seconds in) every time indexing writes a file.
Scoping the watch to `backend/` (the only place source code lives) fixes it.

API comes up at `http://127.0.0.1:8000` — docs at `/docs`.

**Step 4 — Open the frontend**

```bash
cd frontend
python -m http.server 5500
```

Visit `http://127.0.0.1:5500/index.html`.

The frontend talks to the deployed backends by default. To use your local backend instead, set `USE_LOCAL_API = true` at the top of `frontend/js/app.js` (it then calls `http://127.0.0.1:8000` only, with no fallback) - and set it back to `false` before committing.

**Tests** - the frontend's AWS → Render failover logic has a small test suite (Node 18+, no dependencies):

```bash
node --test frontend/tests/
```

### Android

The Android app talks to the Render backend only (`baseURL` in `mobile/main.go`) - it does not use the AWS primary or the failover described in [Deployment](#-deployment) yet.

To develop against a local backend, change `baseURL` in `mobile/main.go` (`127.0.0.1:8000` for the desktop build, `10.0.2.2:8000` for the Android emulator) and switch it back before committing.

Same applies to `manualURL` and `webURL` in `mobile/ui/auth.go` (the Dashboard drawer's Manual/Web links) — they point at the web frontend and are commonly left on a local dev URL while testing; switch them to the deployed frontend domain before releasing a build.

The Android APK is distributed through GitHub Releases rather than committed to the repository.

**[Download the latest Android release](https://github.com/shreyaghorui222004/chat-with-repo/releases/tag/v1.3.0)**

1. Download `ChatWithRepo.apk`.
2. Transfer it to your Android device if necessary.
3. Open the APK and install it.
4. If Android blocks the installation, allow installation from unknown sources.

---

## ◈ Deployment

| Part | Hosted on | URL |
|---|---|---|
| Frontend | Vercel | [chatwithrepo-nine.vercel.app](https://chatwithrepo-nine.vercel.app/) |
| Backend - **primary** | AWS Lambda (container image) behind a Function URL with response streaming | `https://qettzfl73r7xhbres3oqvx3qt40mmqrp.lambda-url.ap-south-1.on.aws` |
| Repository indexing | AWS ECS Fargate - one task per indexing job | started by the Lambda |
| Repo cache | Amazon S3 | raw repo JSON, shared by the Lambda and Fargate |
| Backend - **fallback** | Render | `https://chat-with-repo-4vwy.onrender.com` |
| Database | Neon PostgreSQL | shared by both backends |
| Vector store | Qdrant Cloud | shared by both backends |

Both backends run the **same FastAPI code** with the **same environment variables**, and both read and write the same Neon database and Qdrant cluster. So it doesn't matter which one answers a request: a user, their login token, their chats and their message history are identical on both. The AWS-only behaviour (Fargate indexing, S3 cache, progress in Postgres) is switched on by environment variables that only the AWS stack sets, so Render runs exactly as before.

### How a request reaches the backend

```text
Browser (Vercel frontend)
  │   apiFetch() in frontend/js/app.js
  │
  ├─①─► AWS Lambda   *.lambda-url.ap-south-1.on.aws  (Function URL, RESPONSE_STREAM)
  │       Lambda Web Adapter ──► uvicorn 127.0.0.1:8000 ──► FastAPI
  │       new chat ──► ECS Fargate task (indexing) ──► S3 repo cache
  │
  │     no response headers within 10s · connection/TLS error · 502 / 503 / 504
  │
  └─②─► Render       chat-with-repo-4vwy.onrender.com ──► FastAPI (same code)

           both ──► Neon Postgres · Qdrant Cloud · GitHub API · Cohere · Groq · Gemini
```

Every API call in the web app - including the streamed answers on `/chat/{id}/ask/stream` - goes through one function, `apiFetch()` in `frontend/js/app.js`:

1. **AWS first.** The request goes to AWS. If AWS answers normally (including errors like 400, 401, 403, 404 or 409), that answer is used. When AWS is up there is no extra delay.
2. **10-second failover.** If AWS sends no response headers within 10 seconds, can't be reached, or returns 502/503/504, the request is cancelled and sent to Render with the same method, body and `Authorization` header. The limit only applies until the response starts, so long streamed answers are never cut off. 10 seconds leaves room for a Lambda cold start (~5 s).
3. **Remember for 5 minutes.** After a failover, the browser remembers (in `localStorage`) that AWS is down and sends everything straight to Render for 5 minutes, so only the first request pays the wait. Each page load also checks AWS once in the background, so traffic moves back to AWS as soon as it is up again.
4. **Never run a request twice.** Requests that change something - registering, creating a chat, asking a question, starting a checkout - check whether the AWS app itself is alive. If it is, it may already be working on the request, so the frontend waits for AWS instead of sending a second copy to Render. These requests also never move to Render on a 504 (the app may still be processing them).
5. **Streams are never restarted.** Once AWS starts sending an answer, it stays on AWS. If the stream breaks part-way, the user sees the error instead of a second, duplicate answer from Render.
6. **Leaving a page is not an outage.** Requests cancelled because the user navigated away (e.g. clicked Logout while the dashboard was still loading) don't mark AWS down and aren't replayed on Render.

To try the failover locally, serve the frontend on port 5500 and block the `lambda-url` domain in the browser's DevTools (Network → request blocking): requests move to Render. Afterwards, unblock it and delete `devlens_api_primary_down_until` from Local Storage.

### AWS (primary)

Everything is defined in `template.yaml` (AWS SAM) and deployed by `deploy-aws.sh` as the CloudFormation stack `chatwithrepo` in `ap-south-1`.

| Resource | What it does |
|---|---|
| `ApiContainerFunction` | The API: container image (`Dockerfile`) running uvicorn behind [Lambda Web Adapter](https://github.com/awslabs/aws-lambda-web-adapter), arm64, 2 GB, 15 min timeout. Its Function URL (`InvokeMode: RESPONSE_STREAM`) streams answers token by token, with no 30-second API Gateway limit. |
| `IndexCluster` / `IndexTaskDefinition` | Fargate task that indexes one repository (`python -m backend.jobs.index_chat`). Same image as the API. Runs in the default VPC's public subnets with outbound-only access, so no NAT gateway is needed. A Lambda request can't run longer than 15 minutes, so indexing never runs inside one. |
| `RepoCacheBucket` | Private, encrypted S3 bucket for the raw repo JSON (`repos/<owner>_<repo>_<branch>.json`). Fargate uploads it after indexing; the Lambda downloads it on demand, because Lambda's `/tmp` doesn't survive between instances. |
| `index_progress` table | Indexing progress lives in Postgres (`PROGRESS_STORE=db`) so the Lambda can report the progress of a job running on Fargate. A job that stops updating for 10 minutes is reported as failed. |
| `FastAPIApp` + HTTP API | The earlier zip-packaged Lambda behind API Gateway (`https://e77kyrzage.execute-api.ap-south-1.amazonaws.com`). Kept as a backup but not used by the frontend: API Gateway buffers responses (no streaming) and cuts them off at 30 seconds. |

**Deploying** (needs Docker, AWS CLI, AWS SAM CLI and `uv`):

```bash
./deploy-aws.sh --no-execute-changeset   # build + push the image, create a changeset to review
./deploy-aws.sh                          # build + push, deploy (asks before applying the changeset)
```

The script builds the arm64 image, pushes it to the ECR repository `chatwithrepo-api` (last 3 images kept) with a unique tag, deploys the stack, and sends one warm-up request so the first user doesn't pay the slower first start of a new image. The same image is used by the Lambda and the Fargate task, so they always run the same code. AWS-only Python dependencies are in `requirements-aws.txt`; `requirements.txt` (Render) is unchanged.

**Configuration:** stack settings and non-secret parameters live in `samconfig.toml` (gitignored). Secrets (`DATABASE_URL`, `JWT_SECRET`, API keys) are `NoEcho` stack parameters: deploys keep their previous values, so they never pass through the script. To change one, update that parameter on the `chatwithrepo` stack in the CloudFormation console (*Update → Use current template*). The AWS-only variables (`INDEX_RUNNER=ecs`, `PROGRESS_STORE=db`, `REPO_CACHE_BUCKET`, `INDEX_*`) are set by the template.

**Logs:** CloudWatch `/aws/lambda/chatwithrepo-api-container` (API) and `/ecs/chatwithrepo-index` (indexing jobs). Check it: `curl https://qettzfl73r7xhbres3oqvx3qt40mmqrp.lambda-url.ap-south-1.on.aws/` should return `{"message": "✅ ChatWithRepo API is running!", ...}`.

**Tests** for the AWS pieces (progress store, Fargate launcher, S3 cache, job entry point) run against a throwaway SQLite database and fake AWS clients:

```bash
.venv/bin/python -m unittest backend.tests.test_aws_backend
```

**Cold starts:** a new Lambda instance starts in about 5 seconds; its first question takes longer (~20 s) while the RAG libraries load, later ones are fast. Indexing a small repository takes about 35 seconds including the Fargate task start.

### Render (fallback)

A Render web service running the same repository:

- **Build command:** `pip install -r requirements.txt`
- **Start command:** `uvicorn backend.app:app --host 0.0.0.0 --port $PORT`

Render indexes in-process (FastAPI background task) and keeps indexing progress in memory, so run a **single** worker. On the free tier the service sleeps after about 15 minutes without traffic. Since AWS normally handles everything, Render is usually asleep, and the first request after a failover can take ~30 seconds while it wakes up. An external uptime monitor pinging `https://chat-with-repo-4vwy.onrender.com/` every 10 minutes keeps it awake.

### Keep both backends in sync

- **Same environment variables on both**, especially `JWT_SECRET` (otherwise a login token from one backend is rejected by the other) and `DATABASE_URL` / `QDRANT_URL` (otherwise users and chats differ between them). `JWT_SECRET` falls back to a built-in default when unset - always set it.
- **Deploy the same commit to both.** The allowed frontend origins (CORS) are listed in `backend/app.py`; a new frontend domain must be added there and deployed to both backends.
- **Payments:** the Dodo Payments webhook URL is configured in the Dodo dashboard and points at one backend; `APP_URL` is the frontend URL users return to after checkout.

### Known limitations

- **Indexing during a failover.** AWS keeps indexing progress in Postgres, Render keeps it in memory. If the frontend switches to Render while AWS is indexing a repository, Render doesn't see that progress and starts indexing the same chat again.
- **The Android app** still uses Render only (see [Android](#android)).

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

![Qdrant](https://img.shields.io/badge/Qdrant-Vector%20Store-DC244C?style=flat-square)
![Neon](https://img.shields.io/badge/Neon-Postgres-00E699?style=flat-square&logo=neondatabase&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-4169E1?style=flat-square&logo=postgresql&logoColor=white)

![GitHub](https://img.shields.io/badge/GitHub-API-181717?style=flat-square&logo=github&logoColor=white)
![Dodo%20Payments](https://img.shields.io/badge/Dodo-Payments-9BE000?style=flat-square)

![HTML5](https://img.shields.io/badge/HTML5-E34F26?style=flat-square&logo=html5&logoColor=white)
![CSS3](https://img.shields.io/badge/CSS3-1572B6?style=flat-square&logo=css3&logoColor=white)
![JavaScript](https://img.shields.io/badge/JavaScript-F7DF1E?style=flat-square&logo=javascript&logoColor=black)

![AWS Lambda](https://img.shields.io/badge/AWS-Lambda-FF9900?style=flat-square&logo=awslambda&logoColor=white)
![AWS Fargate](https://img.shields.io/badge/AWS-Fargate-FF9900?style=flat-square&logo=amazonecs&logoColor=white)
![Amazon S3](https://img.shields.io/badge/Amazon-S3-569A31?style=flat-square&logo=amazons3&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white)
![Render](https://img.shields.io/badge/Render-46E3B7?style=flat-square&logo=render&logoColor=black)
![Vercel](https://img.shields.io/badge/Vercel-000000?style=flat-square&logo=vercel&logoColor=white)

</div>

---

<div align="center">

Built by [anikchand461](https://github.com/anikchand461)
and [shreyaghorui222004](https://github.com/shreyaghorui222004)

</div>
