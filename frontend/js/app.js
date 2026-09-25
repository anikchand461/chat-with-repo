// const API = "https://chat-with-repo-4vwy.onrender.com";
const API = "http://127.0.0.1:8000"; // local testing only

// Change this to your repository (username/repo)
const GITHUB_REPO = "shreyaghorui222004/chat-with-repo";

const FREE_CHAT_LIMIT = 2;

const authToken = localStorage.getItem("devlens_token");

const page = location.pathname.split("/").pop();

if (
  !authToken &&
  page !== "login.html" &&
  page !== "register.html" &&
  page !== "index.html" &&
  page !== "manual.html" &&
  page !== ""
) {
  location.href = "login.html";
}

function token() {
  return localStorage.getItem("devlens_token") || "";
}

async function request(path, options = {}) {
  const response = await fetch(API + path, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(token() ? { Authorization: `Bearer ${token()}` } : {}),
    },
  });

  const text = await response.text();
  const data = text ? JSON.parse(text) : {};

  if (!response.ok) {
    throw Error(data.detail || "Request failed");
  }

  return data;
}

/* ==================== UI: theme, nav, drawer ==================== */

function applyTheme(theme) {
  document.documentElement.setAttribute("data-theme", theme);
  localStorage.setItem("devlens_theme", theme);
}

function setupTheme() {
  const stored =
    localStorage.getItem("devlens_theme") ||
    (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");

  applyTheme(stored);

  document.querySelectorAll(".theme-toggle").forEach((btn) => {
    btn.addEventListener("click", () => {
      const next =
        document.documentElement.getAttribute("data-theme") === "dark" ? "light" : "dark";
      applyTheme(next);
    });
  });
}

function setupNav() {
  document.querySelectorAll(".github-link").forEach((el) => {
    el.href = `https://github.com/${GITHUB_REPO}`;
  });

  const nav = document.querySelector(".nav");
  const burger = document.querySelector(".hamburger");

  if (nav && burger) {
    burger.addEventListener("click", () => {
      const open = nav.classList.toggle("open");
      burger.setAttribute("aria-expanded", String(open));
    });
  }

  const drawerBtn = document.querySelector(".chat-drawer-btn");
  const backdrop = document.querySelector(".sidebar-backdrop");

  if (drawerBtn) {
    drawerBtn.addEventListener("click", () =>
      document.body.classList.toggle("drawer-open")
    );
  }

  if (backdrop) {
    backdrop.addEventListener("click", () =>
      document.body.classList.remove("drawer-open")
    );
  }

  document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape") return;
    document.body.classList.remove("drawer-open");
    nav?.classList.remove("open");
  });
}

setupTheme();
document.addEventListener("DOMContentLoaded", setupNav);

/* ==================== auth ==================== */

function setupAuth(kind) {
  document.querySelector("#auth-form").addEventListener("submit", async (e) => {
    e.preventDefault();

    const button = e.target.querySelector("button");
    const original = button ? button.textContent : "";
    if (button) {
      button.disabled = true;
      button.textContent = "Please wait...";
    }

    try {
      const email = document.querySelector("#email").value;

      const data = await request(`/auth/${kind}`, {
        method: "POST",
        body: JSON.stringify({
          email,
          password: document.querySelector("#password").value,
        }),
      });

      localStorage.setItem("devlens_token", data.access_token);
      // Remember the email so the profile page can greet the user right away
      localStorage.setItem("devlens_email", email.trim());
      location.href = "dashboard.html";
    } catch (error) {
      document.querySelector("#error").textContent = error.message;
    } finally {
      if (button) {
        button.disabled = false;
        button.textContent = original;
      }
    }
  });
}

/* ==================== dashboard ==================== */

async function setupDashboard() {
  const createBtn = document.querySelector("#create");
  const cancelBtn = document.querySelector("#cancel");
  const modal = document.querySelector("#modal");
  const form = document.querySelector("#create-form");

  // ---- Indexing progress panel (shown in place of the form while a new
  // chat's repo is being fetched + indexed in the background) ----
  const progressPanel = document.querySelector("#index-progress");
  const progressMessage = document.querySelector("#index-progress-message");
  const progressSpinner = progressPanel?.querySelector(".spinner-ring");
  const progressHint = document.querySelector("#index-progress-hint");
  const progressErrorBox = document.querySelector("#index-progress-error");
  const progressErrorText = document.querySelector("#index-progress-error-text");
  const progressTokenLink = document.querySelector("#index-progress-token-link");
  const progressClose = document.querySelector("#index-progress-close");
  const progressBarWrap = document.querySelector("#index-progress-bar-wrap");
  const progressBar = document.querySelector("#index-progress-bar");

  let pollTimer = null;
  let pollAttempts = 0;
  const MAX_POLL_ATTEMPTS = 400; // ~10 min at 1.5s/poll - generous for a large repo

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
    pollAttempts = 0;
  }

  function showForm() {
    stopPolling();
    if (form) form.hidden = false;
    if (progressPanel) progressPanel.hidden = true;
  }

  function showProgress() {
    if (form) form.hidden = true;
    if (progressPanel) progressPanel.hidden = false;
    if (progressSpinner) progressSpinner.hidden = false;
    if (progressHint) progressHint.hidden = false;
    if (progressErrorBox) progressErrorBox.hidden = true;
    if (progressTokenLink) progressTokenLink.hidden = true;
    if (progressClose) progressClose.hidden = true;
    if (progressMessage) progressMessage.textContent = "Setting up your chat…";
    setProgressBar(null); // hidden until we actually have a done/total to show
  }

  // done/total only exist while the backend is in the "indexing" (embedding)
  // stage - earlier stages (fetching the repo, etc.) don't have a
  // meaningful percentage, so the bar just stays hidden and the spinner
  // alone carries those. Pass null to hide it.
  function setProgressBar(fraction) {
    if (!progressBarWrap || !progressBar) return;

    if (fraction == null) {
      progressBarWrap.hidden = true;
      progressBar.style.width = "0%";
      return;
    }

    progressBarWrap.hidden = false;
    progressBar.style.width = `${Math.round(Math.min(1, Math.max(0, fraction)) * 100)}%`;
  }

  function showProgressError(message) {
    stopPolling();
    if (progressSpinner) progressSpinner.hidden = true;
    if (progressHint) progressHint.hidden = true;
    if (progressMessage) progressMessage.textContent = "Couldn't finish indexing";
    if (progressErrorBox) progressErrorBox.hidden = false;
    if (progressErrorText) progressErrorText.textContent = message;
    if (progressClose) progressClose.hidden = false;
    setProgressBar(null);

    // The backend sends this exact wording when GitHub's rate limit is hit
    // (github/client.py) - point the user at the fix instead of just erroring.
    if (progressTokenLink) progressTokenLink.hidden = !/rate limit/i.test(message);
  }

  async function pollIndexStatus(chatId) {
    pollAttempts += 1;
    if (pollAttempts > MAX_POLL_ATTEMPTS) {
      showProgressError(
        "This is taking longer than expected. It may still finish in the background - check back in a bit, or open the chat from the sidebar."
      );
      return;
    }

    try {
      const status = await request(`/chat/${chatId}/index-status`);

      if (status.status === "ready") {
        stopPolling();
        location.href = `chat.html?id=${chatId}`;
        return;
      }

      if (status.status === "error") {
        showProgressError(status.message || "Something went wrong while indexing.");
        return;
      }

      if (progressMessage) {
        progressMessage.textContent = status.message || progressMessage.textContent;
      }

      setProgressBar(
        status.total ? status.done / status.total : null
      );
    } catch (err) {
      console.error("index-status poll failed:", err);
      // Transient network hiccup - keep polling rather than killing the flow.
    }
  }

  if (progressClose) {
    progressClose.onclick = () => {
      showForm();
      if (modal) modal.hidden = true;
    };
  }

  // If the POST to /chat/create fails at the network level (e.g. a dev
  // server reload drops the connection mid-response), the chat may well
  // have already been created server-side - the browser just never saw
  // the reply. Check /chat/list for a matching chat before assuming the
  // whole thing failed.
  async function findRecentChat(owner, repo, branch) {
    try {
      const chats = await request("/chat/list");
      const match = chats.find(
        (c) => c.owner === owner && c.repo === repo && c.branch === branch
      );
      if (!match) return null;
      return { ...match, indexing: true };
    } catch (_) {
      return null;
    }
  }

  // Backoff schedule (ms) between retries - generous, since a dev-server
  // restart (picking up a code change, or a data/ file write if --reload
  // isn't scoped to backend/) can take several seconds to come back up.
  const CREATE_RETRY_DELAYS = [1000, 2500, 4000];

  async function submitCreateChat(owner, repo, branch, onRetry) {
    const payload = { owner, repo, branch };

    for (let attempt = 0; ; attempt++) {
      try {
        return await request("/chat/create", { method: "POST", body: JSON.stringify(payload) });
      } catch (err) {
        console.error(`chat/create attempt ${attempt + 1} failed:`, err);

        // The request may well have reached the server even though this
        // client never saw the reply (e.g. a dev-server restart mid-response).
        const recovered = await findRecentChat(owner, repo, branch);
        if (recovered) return recovered;

        if (attempt >= CREATE_RETRY_DELAYS.length) throw err;

        const delay = CREATE_RETRY_DELAYS[attempt];
        if (onRetry) onRetry(attempt + 1, delay);
        await new Promise((r) => setTimeout(r, delay));
      }
    }
  }

  if (createBtn) {
    createBtn.onclick = async () => {
      // Soft check: if free user already has 2 chats, show upgrade instead of create form
      try {
        const [chats, status] = await Promise.all([
          request("/chat/list"),
          request("/payment/status").catch(() => ({ plan: "FREE", is_pro: false })),
        ]);

        if (!status.is_pro && chats.length >= FREE_CHAT_LIMIT) {
          showUpgradeModal("repo_limit");
          return;
        }
      } catch (err) {
        console.error(err);
      }

      showForm();
      if (modal) modal.hidden = false;
    };
  }

  if (cancelBtn) {
    cancelBtn.onclick = () => {
      if (modal) modal.hidden = true;
    };
  }

  if (form) {
    form.onsubmit = async (e) => {
      e.preventDefault();

      const error = document.querySelector("#form-error");
      if (error) error.textContent = "";

      const button = form.querySelector('button[type="submit"]');
      const original = button ? button.textContent : "";
      if (button) {
        button.disabled = true;
        button.textContent = "Analyzing...";
      }

      try {
        const owner = document.querySelector("#owner").value.trim();
        const repo = document.querySelector("#repo").value.trim();
        const branch = document.querySelector("#branch").value.trim() || "main";

        const data = await submitCreateChat(owner, repo, branch, (attempt, delayMs) => {
          if (error) {
            error.style.color = "";
            error.textContent = `Connection hiccup - retrying (${attempt}/3)…`;
          }
          if (button) button.textContent = `Retrying… (${attempt}/3)`;
        });

        if (error) error.textContent = "";

        if (data.upgrade_required) {
          if (modal) modal.hidden = true;
          showUpgradeModal(data.reason);
          return;
        }

        await loadChats();

        if (data.already_indexed) {
          // Reopening a chat that's already indexed - nothing to wait for.
          if (modal) modal.hidden = true;
          location.href = `chat.html?id=${data.chat_id}`;
          return;
        }

        // New chat: the backend indexes it in the background. Show progress
        // and jump in automatically once it's ready.
        showProgress();
        pollIndexStatus(data.chat_id);
        pollTimer = setInterval(() => pollIndexStatus(data.chat_id), 1500);
      } catch (err) {
        console.error(err);
        if (error) error.textContent = err.message;
      } finally {
        if (button) {
          button.disabled = false;
          button.textContent = original;
        }
      }
    };
  }

  const upgradeBtn = document.getElementById("upgrade-now");
  if (upgradeBtn) {
    upgradeBtn.addEventListener("click", startCheckout);
  }

  await refreshGithubWarning();

  try {
    await loadChats();
  } catch (err) {
    console.error("loadChats failed:", err);
  }

  // chat.html redirects back here (?resume=<id>) when it's opened for a
  // chat that isn't actually indexed yet - all indexing waits happen on
  // this page, never inside the chat itself. Pick the wait back up.
  const resumeId = new URLSearchParams(location.search).get("resume");
  if (resumeId) {
    history.replaceState(null, "", "dashboard.html");
    if (modal) modal.hidden = false;
    showProgress();
    pollIndexStatus(resumeId);
    pollTimer = setInterval(() => pollIndexStatus(resumeId), 1500);
  }
}

async function refreshGithubWarning() {
  const warning = document.querySelector("#github-warning");
  if (!warning) return;

  try {
    const user = await request("/auth/me");
    warning.hidden = Boolean(user && user.has_github_token);
  } catch (err) {
    console.error("auth/me failed:", err);
  }
}

window.addEventListener("pageshow", () => {
  if (document.querySelector("#github-warning")) {
    refreshGithubWarning();
  }
});

document.addEventListener("visibilitychange", () => {
  if (
    document.visibilityState === "visible" &&
    document.querySelector("#github-warning")
  ) {
    refreshGithubWarning();
  }
});

async function loadChats() {
  try {
    const chats = await request("/chat/list");

    document.querySelector("#chats").innerHTML = chats.length
      ? chats
          .map(
            (c) =>
              `<a class="chat-link" href="chat.html?id=${c.chat_id}">
                <strong>${c.title}</strong>
                <small>branch: ${c.branch}</small>
              </a>`
          )
          .join("")
      : "<p>No chats yet. Create one to index a repository.</p>";
  } catch (error) {
    location.href = "login.html";
  }
}

/* ==================== chat ==================== */

async function setupChat() {
  const id = new URLSearchParams(location.search).get("id");
  if (!id) return;

  const chats = await request("/chat/list");
  const list = document.querySelector("#chat-list");
  list.innerHTML = "";

  const current = chats.find((c) => c.chat_id == id);

  // All indexing waits happen on the dashboard's create-chat panel, never
  // in here - if this chat isn't actually ready (a stale link, a sidebar
  // click on a chat whose index died, etc.), bounce back there instead of
  // rendering anything. dashboard.html resumes the same progress panel and
  // sends the user back here once it's genuinely done.
  try {
    const status = await request(`/chat/${id}/index-status`);

    if (status.status === "error" && current) {
      // A failed chat is only ever retried by a deliberate action, never
      // by just checking its status (the dashboard polls this same
      // endpoint automatically every ~1.5s - if a passive check could
      // trigger a retry, a permanent failure like "no token configured"
      // would silently retry-and-fail forever, never actually surfacing
      // the error). Opening the chat counts as that action: retry once
      // via the same path the create modal uses, then let the dashboard
      // show progress (or the error, if it fails again) as usual.
      try {
        await request("/chat/create", {
          method: "POST",
          body: JSON.stringify({
            owner: current.owner,
            repo: current.repo,
            branch: current.branch,
          }),
        });
      } catch (err) {
        console.error("retry via chat/create failed:", err);
      }
      location.replace(`dashboard.html?resume=${id}`);
      return;
    }

    if (status.status !== "ready") {
      location.replace(`dashboard.html?resume=${id}`);
      return;
    }
  } catch (err) {
    console.error("index-status check failed:", err);
    // If we can't even check, fall through and let the normal chat flow
    // (and its own error handling) take it from here.
  }

  if (current) {
    document.querySelector("#chat-title").textContent = current.title;
    document.querySelector("#branch").textContent = `Branch: ${current.branch}`;
    const header = document.querySelector("#chat-header-title");
    if (header) header.textContent = current.title;
  }

  const sorted = [
    ...chats.filter((c) => c.chat_id == id),
    ...chats.filter((c) => c.chat_id != id),
  ];

  sorted.forEach((chat) => {
    const a = document.createElement("a");
    a.href = `chat.html?id=${chat.chat_id}`;
    a.className = "chat-item";
    if (chat.chat_id == id) a.classList.add("active");
    a.innerHTML = `
      <strong>${chat.title}</strong>
      <small>branch: ${chat.branch}</small>
    `;
    list.appendChild(a);
  });

  try {
    const messages = await request(`/chat/${id}/messages`);
    const container = document.querySelector("#messages");
    container.innerHTML = "";

    if (!messages.length) {
      container.innerHTML =
        '<div class="empty"><h2>Ask your repository</h2><p>Ask anything about the indexed codebase — architecture, files, functions or bugs.</p></div>';
    }

    // Timings are stored on the server, so web and mobile show the same values.
    messages.forEach((m) => {
      const node = addMessage(m.content, m.role);
      if (m.role === "assistant" && m.response_seconds != null) {
        node.appendChild(
          buildResponseMeta({
            total: m.response_seconds,
            first: m.first_word_seconds,
          })
        );
      }
    });
  } catch (err) {
    console.error(err);
  }

  const askForm = document.querySelector("#ask");
  if (askForm) {
    askForm.onsubmit = async (e) => {
      e.preventDefault();

      const input = document.querySelector("#question");
      const q = input.value.trim();
      if (!q) return;

      document.querySelector(".empty")?.remove();
      addMessage(q, "user");
      input.value = "";

      const typing = showTyping();
      const startedAt = performance.now();

      try {
        await streamAnswer(id, q, typing, startedAt, current);
      } catch (error) {
        typing.remove();

        // The backend sends this exact wording when GitHub's rate limit is
        // hit (github/client.py) - point the user at the fix instead of
        // just erroring.
        if (/rate limit/i.test(error.message || "")) {
          addTokenPromptMessage(error.message);
        } else {
          addMessage(error.message, "assistant");
        }
      }
    };
  }
}

// Assistant-style bubble used when a question fails because GitHub's API
// rate limit was hit - same wording the "create chat" progress panel
// shows, but here indexing failed mid-chat rather than during creation.
function addTokenPromptMessage(message) {
  const node = document.createElement("div");
  node.className = "message assistant token-prompt";
  node.innerHTML = `
    <p>${escapeHtml(message)}</p>
    <a class="button" href="profile.html">Go to profile → add a GitHub token</a>
  `;

  const container = document.querySelector("#messages");
  container.appendChild(node);
  container.scrollTop = container.scrollHeight;
  return node;
}

// Assistant-style bubble used when a free-plan limit (daily questions or
// repo count) is hit mid-chat. chat.html has no upgrade modal of its own
// (only dashboard.html does), so this renders an inline button that starts
// the same Dodo checkout flow directly instead of silently no-opping.
function addUpgradePromptMessage(message) {
  const node = document.createElement("div");
  node.className = "message assistant token-prompt";
  node.innerHTML = `
    <p>⚠️ ${escapeHtml(message)}</p>
    <button type="button" class="button">Upgrade to Pro</button>
  `;
  node.querySelector("button").addEventListener("click", startCheckout);

  const container = document.querySelector("#messages");
  container.appendChild(node);
  container.scrollTop = container.scrollHeight;
  return node;
}

// Streams /ask/stream (SSE), rendering tokens as they arrive.
// Indexing waits happen on the dashboard page (setupChat() redirects there
// if a chat isn't ready before this ever runs), so this assumes the index
// is already built and doesn't show any indexing progress of its own.
async function streamAnswer(id, question, typing, startedAt, repoInfo) {
  const response = await fetch(`${API}/chat/${id}/ask/stream`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token() ? { Authorization: `Bearer ${token()}` } : {}),
    },
    body: JSON.stringify({ question }),
  });

  const contentType = response.headers.get("content-type") || "";

  // Errors and upgrade_required come back as plain JSON.
  if (!contentType.includes("text/event-stream")) {
    const data = await response.json().catch(() => ({}));
    typing.remove();

    if (!response.ok) throw Error(data.detail || "Request failed");

    if (data.upgrade_required) {
      addUpgradePromptMessage(data.message);
    }
    return;
  }

  const container = document.querySelector("#messages");
  const node = document.createElement("div");
  node.className = "message assistant";

  let text = "";
  let sources = [];
  let firstTokenAt = null;
  let started = false;
  let pending = false;
  let failed = null;
  let serverTimes = null; // total/first measured by the server (sent with "done")

  const render = () => {
    pending = false;
    node.innerHTML = sourcesRowHtml(sources, repoInfo) + renderMarkdown(text);
    container.scrollTop = container.scrollHeight;
  };

  const handle = (payload) => {
    if (payload.error) {
      failed = payload.error;
      return;
    }
    if (payload.done) {
      serverTimes = { total: payload.total, first: payload.first };
      return;
    }
    if (payload.sources) {
      sources = payload.sources;
      showSources(typing, payload.sources);
      return;
    }
    if (!payload.token) return;

    if (!started) {
      started = true;
      firstTokenAt = performance.now();
      typing.remove();
      container.appendChild(node);
    }

    text += payload.token;
    if (!pending) {
      pending = true;
      requestAnimationFrame(render);
    }
  };

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });

    let sep;
    while ((sep = buffer.indexOf("\n\n")) !== -1) {
      const event = buffer.slice(0, sep);
      buffer = buffer.slice(sep + 2);

      event.split("\n").forEach((line) => {
        if (!line.startsWith("data:")) return;
        try {
          handle(JSON.parse(line.slice(5).trim()));
        } catch (_) {}
      });
    }
  }

  typing.remove();

  if (failed && !started) throw Error(failed);

  render();
  highlightCodeBlocks(node);

  // Prefer the server's timing (it is what web and mobile both see later);
  // fall back to our own clock if the stream ended without a "done" event.
  const info =
    serverTimes && serverTimes.total != null
      ? { total: serverTimes.total, first: serverTimes.first, interrupted: false }
      : {
          total: (performance.now() - startedAt) / 1000,
          first: firstTokenAt ? (firstTokenAt - startedAt) / 1000 : null,
          interrupted: Boolean(failed),
        };
  node.appendChild(buildResponseMeta(info));
  container.scrollTop = container.scrollHeight;
}

function formatSeconds(sec) {
  return sec < 10 ? `${sec.toFixed(1)}s` : `${Math.round(sec)}s`;
}

// Footer shown under an assistant answer: clock icon, total time, time to first word.
function buildResponseMeta(info) {
  if (typeof info === "string") info = { label: info };

  const meta = document.createElement("div");
  meta.className = "response-meta";

  const clock =
    '<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" ' +
    'stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' +
    '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>';

  const parts = info.label
    ? [escapeHtml(info.label)]
    : [`Responded in <b>${formatSeconds(info.total)}</b>`];

  if (info.first != null) parts.push(`first word ${formatSeconds(info.first)}`);
  if (info.interrupted) parts.push("interrupted");

  meta.innerHTML = clock + parts.map((p) => `<span>${p}</span>`).join("");
  return meta;
}

function showTyping() {
  const node = document.createElement("div");
  node.className = "typing";
  node.setAttribute("aria-label", "Assistant is typing");
  node.innerHTML = "<i></i><i></i><i></i>";

  const container = document.querySelector("#messages");
  container.appendChild(node);
  container.scrollTop = container.scrollHeight;
  return node;
}

// Morphs the "typing" placeholder into a list of the files the answer's
// context came from, revealed one at a time. It stays in place of the
// typing dots until the first real answer token arrives, at which point
// the caller's usual `typing.remove()` clears it away.
function showSources(panel, files) {
  if (!panel || !panel.isConnected || !files || !files.length) return;

  panel.className = "sources-panel";
  panel.setAttribute("aria-label", "Searching the codebase");
  panel.innerHTML = `
    <div class="sources-header"><i class="sources-dot"></i> Searching the codebase…</div>
    <ul class="sources-list"></ul>
  `;

  const list = panel.querySelector(".sources-list");
  const container = document.querySelector("#messages");

  files.slice(0, 8).forEach((path, i) => {
    const li = document.createElement("li");
    li.textContent = path;
    li.style.animationDelay = `${i * 90}ms`;
    list.appendChild(li);
  });

  container.scrollTop = container.scrollHeight;
}

// Per-extension color for the small dot on each source chip - not exact
// brand logos (no bundled/CDN icon set), just a recognizable color cue next
// to the real filename. Full path is on hover via the native title tooltip.
const SOURCE_COLORS = {
  py: "#3776AB",
  js: "#F7DF1E", mjs: "#F7DF1E", cjs: "#F7DF1E",
  jsx: "#61DAFB",
  ts: "#3178C6", tsx: "#3178C6",
  go: "#00ADD8", mod: "#00ADD8", sum: "#00ADD8",
  java: "#EA2D2E",
  rb: "#CC342D",
  php: "#777BB4",
  c: "#5C6BC0", h: "#5C6BC0",
  cpp: "#00599C", cc: "#00599C", hpp: "#00599C",
  cs: "#9B4F96",
  rs: "#DEA584",
  kt: "#7F52FF",
  swift: "#F05138",
  scala: "#DC322F",
  dart: "#0175C2",
  vue: "#42B883",
  svelte: "#FF3E00",
  html: "#E34F26", htm: "#E34F26",
  css: "#1572B6",
  scss: "#CC6699", sass: "#CC6699",
  json: "#8A8A8A",
  yaml: "#CB171E", yml: "#CB171E",
  toml: "#9C4221",
  md: "#4A5568", mdx: "#4A5568",
  sql: "#336791",
  sh: "#4EAA25", bash: "#4EAA25",
  dockerfile: "#2496ED",
  xml: "#0060AC",
  ini: "#6B7280", cfg: "#6B7280", txt: "#6B7280",
};
const DEFAULT_SOURCE_COLOR = "#6B7280";

function fileExtension(path) {
  const name = path.split("/").pop() || path;
  const dot = name.lastIndexOf(".");
  return dot > 0 ? name.slice(dot + 1).toLowerCase() : name.toLowerCase();
}

function fileBaseName(path) {
  return path.split("/").pop() || path;
}

// github.com/{owner}/{repo}/blob/{branch}/{path}, each path segment (and
// the branch) individually percent-encoded so filenames with spaces/etc.
// still produce a valid URL. null when repoInfo isn't available (e.g. the
// owner/repo/branch lookup failed) - callers fall back to a plain chip.
function githubFileUrl(repoInfo, path) {
  if (!repoInfo || !repoInfo.owner || !repoInfo.repo) return null;

  const branch = encodeURIComponent(repoInfo.branch || "main");
  const encodedPath = path.split("/").map(encodeURIComponent).join("/");

  return `https://github.com/${repoInfo.owner}/${repoInfo.repo}/blob/${branch}/${encodedPath}`;
}

// Row of small per-file chips shown at the top of a finished answer bubble
// - the real filenames its context actually came from, in the order
// retrieval ranked them, each with a language-colored dot, the full path on
// hover, and (when repoInfo is known) a link to that file on GitHub.
// Empty string (not just "") when there are none, so callers can safely
// prepend it unconditionally.
function sourcesRowHtml(files, repoInfo) {
  if (!files || !files.length) return "";

  const chips = files.slice(0, 8).map((path) => {
    const color = SOURCE_COLORS[fileExtension(path)] || DEFAULT_SOURCE_COLOR;
    const inner =
      `<i class="source-dot" style="background:${color}"></i>` +
      `${escapeHtml(fileBaseName(path))}`;
    const url = githubFileUrl(repoInfo, path);

    return url
      ? `<a class="source-chip" href="${escapeHtml(url)}" target="_blank" rel="noopener noreferrer" title="${escapeHtml(path)}">${inner}</a>`
      : `<span class="source-chip" title="${escapeHtml(path)}">${inner}</span>`;
  });

  return `<div class="sources-row">${chips.join("")}</div>`;
}

/* ---- markdown rendering ---- */

function escapeHtml(str) {
  return str
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function renderMarkdown(raw) {
  const codeBlocks = [];
  let text = raw.replace(/```(\w+)?\n?([\s\S]*?)```/g, (_, lang, code) => {
    codeBlocks.push({ lang: (lang || "").trim(), code: code.replace(/\n$/, "") });
    return `\u0000CODEBLOCK${codeBlocks.length - 1}\u0000`;
  });

  text = escapeHtml(text);
  text = text.replace(/`([^`\n]+)`/g, (_, code) => `<code>${code}</code>`);
  text = text.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  text = text.replace(/__([^_]+)__/g, "<strong>$1</strong>");
  text = text.replace(/(^|[^*])\*([^*\n]+)\*(?!\*)/g, "$1<em>$2</em>");
  text = text.replace(/(^|[^_])_([^_\n]+)_(?!_)/g, "$1<em>$2</em>");

  const lines = text.split("\n");
  let html = "";
  let inUl = false;
  let inOl = false;
  let para = [];

  const closeLists = () => {
    if (inUl) {
      html += "</ul>";
      inUl = false;
    }
    if (inOl) {
      html += "</ol>";
      inOl = false;
    }
  };
  const flushPara = () => {
    if (para.length) {
      html += `<p>${para.join("<br>")}</p>`;
      para = [];
    }
  };

  lines.forEach((line) => {
    const trimmed = line.trim();
    const codeBlockMatch = /^\u0000CODEBLOCK(\d+)\u0000$/.exec(trimmed);
    const heading = /^(#{1,6})\s+(.*)/.exec(trimmed);
    const bullet = /^[*\-•]\s+(.*)/.exec(trimmed);
    const numbered = /^\d+\.\s+(.*)/.exec(trimmed);

    if (codeBlockMatch) {
      closeLists();
      flushPara();
      html += trimmed;
    } else if (heading) {
      closeLists();
      flushPara();
      const level = Math.min(heading[1].length, 6);
      html += `<h${level}>${heading[2]}</h${level}>`;
    } else if (bullet) {
      flushPara();
      if (inOl) {
        html += "</ol>";
        inOl = false;
      }
      if (!inUl) {
        html += "<ul>";
        inUl = true;
      }
      html += `<li>${bullet[1]}</li>`;
    } else if (numbered) {
      flushPara();
      if (inUl) {
        html += "</ul>";
        inUl = false;
      }
      if (!inOl) {
        html += "<ol>";
        inOl = true;
      }
      html += `<li>${numbered[1]}</li>`;
    } else if (trimmed === "") {
      closeLists();
      flushPara();
    } else {
      closeLists();
      para.push(trimmed);
    }
  });
  closeLists();
  flushPara();

  html = html.replace(/\u0000CODEBLOCK(\d+)\u0000/g, (_, idx) => {
    const block = codeBlocks[Number(idx)];
    const label = block.lang || "text";
    const escaped = escapeHtml(block.code);
    const encoded = encodeURIComponent(block.code);
    return (
      `<div class="code-block">` +
      `<div class="code-block-head">` +
      `<span class="code-block-lang">${escapeHtml(label)}</span>` +
      `<button type="button" class="copy-btn" data-code="${encoded}">Copy</button>` +
      `</div>` +
      `<pre><code data-lang="${escapeHtml(block.lang)}">${escaped}</code></pre>` +
      `</div>`
    );
  });

  return html;
}

function highlightCodeBlocks(scope) {
  if (!window.hljs) return;

  scope.querySelectorAll("pre code[data-lang]").forEach((block) => {
    const requested = block.dataset.lang;
    const known = requested && hljs.getLanguage(requested);
    const result = known
      ? hljs.highlight(block.textContent, { language: requested })
      : hljs.highlightAuto(block.textContent);

    block.innerHTML = result.value;
    block.classList.add("hljs");

    const detected = known ? requested : result.language;
    if (detected) {
      block.classList.add(`language-${detected}`);
      if (!requested) {
        const label = block.closest(".code-block")?.querySelector(".code-block-lang");
        if (label) label.textContent = detected;
      }
    }
  });
}

function addMessage(text, role) {
  const node = document.createElement("div");
  node.className = `message ${role}`;
  node.innerHTML =
    role === "user" ? `<p>${escapeHtml(text)}</p>` : renderMarkdown(text);

  highlightCodeBlocks(node);

  const container = document.querySelector("#messages");
  container.appendChild(node);
  container.scrollTop = container.scrollHeight;
  return node;
}

document.addEventListener("click", (e) => {
  const btn = e.target.closest(".copy-btn");
  if (!btn) return;

  const code = decodeURIComponent(btn.dataset.code || "");
  navigator.clipboard
    .writeText(code)
    .then(() => {
      const original = btn.textContent;
      btn.textContent = "Copied!";
      btn.classList.add("copied");
      setTimeout(() => {
        btn.textContent = original;
        btn.classList.remove("copied");
      }, 1500);
    })
    .catch(() => {
      btn.textContent = "Failed";
    });
});

/* ==================== profile ==================== */

// Email from the login JWT, used only as an instant fallback before /auth/me answers
function emailFromToken() {
  try {
    let part = token().split(".")[1];
    if (!part) return "";
    part = part.replace(/-/g, "+").replace(/_/g, "/");
    part += "=".repeat((4 - (part.length % 4)) % 4);
    const bytes = Uint8Array.from(atob(part), (c) => c.charCodeAt(0));
    const payload = JSON.parse(new TextDecoder().decode(bytes));
    if (payload.email) return String(payload.email);
    if (payload.sub && String(payload.sub).includes("@")) return String(payload.sub);
  } catch (_) {}
  return "";
}

// "shreya.ghorui@gmail.com" -> "Shreya"
function greetingName(email) {
  const local = String(email || "").split("@")[0].trim();
  const first = local.split(/[._+-]/)[0];
  return first ? first.charAt(0).toUpperCase() + first.slice(1) : "";
}

function setupProfile() {
  const form = document.querySelector("#token-form");
  if (!form) return;

  const nameEl = document.querySelector("#user-name");
  const emailEl = document.querySelector("#user-email");
  const banner = document.querySelector("#token-status");
  const bannerTitle = document.querySelector("#status-title");
  const bannerDesc = document.querySelector("#status-desc");
  const status = document.querySelector("#status");
  const input = document.querySelector("#token");
  const saveBtn = document.querySelector("#save-btn");
  const saveLabel = saveBtn ? saveBtn.querySelector(".label") : null;
  const removeBtn = document.querySelector("#remove-token");

  const ICON_OK =
    '<path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/>';
  const ICON_WARN =
    '<path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>';

  let hasToken = false;

  function setStatus(message, kind) {
    if (!status) return;
    status.textContent = message || "";
    status.className = kind || "";
  }

  function setBusy(button, busy) {
    if (!button) return;
    button.classList.toggle("loading", busy);
    button.disabled = busy;
  }

  function showUser(email) {
    nameEl.textContent = greetingName(email) || "there";
    emailEl.textContent = email || "";
    emailEl.hidden = !email;
  }

  function showTokenState(has) {
    hasToken = has;
    banner.className = "token-status " + (has ? "configured" : "missing");
    banner.querySelector(".icon").innerHTML = has ? ICON_OK : ICON_WARN;
    bannerTitle.textContent = has ? "GitHub token configured" : "GitHub token required";
    bannerDesc.textContent = has
      ? "Private repos and higher rate limits are available. The dashboard warning is hidden."
      : "Without one, GitHub's public API allows only 60 requests/hour - not enough to index most repositories.";

    if (removeBtn) removeBtn.hidden = !has;
    if (saveLabel) saveLabel.textContent = has ? "Update token" : "Save token";

    if (has) {
      // The server never sends the token back, so this is a generic mask
      input.placeholder = "••••••••••••••••••••••••";
      input.removeAttribute("required"); // empty = keep the saved token
    } else {
      input.placeholder = "ghp_xxxxxxxxxxxxxxxxxxxx";
      input.setAttribute("required", "");
    }
  }

  // Plan status (Free vs Pro) + upgrade button
  const planBanner = document.querySelector("#plan-status");
  const planTitle = document.querySelector("#plan-title");
  const planDesc = document.querySelector("#plan-desc");
  const upgradeBtn = document.querySelector("#upgrade-btn");

  if (planBanner) {
    request("/payment/status")
      .then((sub) => {
        const isPro = Boolean(sub && sub.is_pro);
        planBanner.className = "token-status " + (isPro ? "configured" : "missing");
        planBanner.querySelector(".icon").innerHTML = isPro ? ICON_OK : ICON_WARN;
        planTitle.textContent = isPro ? "Pro plan" : "Free plan";
        planDesc.textContent = isPro
          ? "Unlimited questions and repository chats."
          : "10 questions/day and 2 repository chats. Upgrade for unlimited access.";
        if (upgradeBtn) upgradeBtn.hidden = isPro;
      })
      .catch((err) => {
        console.error("payment/status failed:", err);
        planBanner.className = "token-status";
        planBanner.querySelector(".icon").innerHTML = ICON_WARN;
        planTitle.textContent = "Couldn't check your plan";
        planDesc.textContent = "The server didn't answer. Refresh the page to try again.";
      });
  }

  if (upgradeBtn) upgradeBtn.addEventListener("click", startCheckout);

  // 1) Show what we already know straight away (email typed at login / JWT)
  showUser(localStorage.getItem("devlens_email") || emailFromToken());

  // 2) Then ask the server: real email + whether a GitHub token is already saved
  request("/auth/me")
    .then((me) => {
      if (me && me.email) {
        localStorage.setItem("devlens_email", me.email);
        showUser(me.email);
      }
      showTokenState(Boolean(me && me.has_github_token));
    })
    .catch((err) => {
      console.error("auth/me failed:", err);
      banner.className = "token-status";
      banner.querySelector(".icon").innerHTML = ICON_WARN;
      bannerTitle.textContent = "Couldn't check your token";
      bannerDesc.textContent = "The server didn't answer. Refresh the page to try again.";
    });

  // Save / update token
  form.onsubmit = async (e) => {
    e.preventDefault();
    setStatus("");

    const value = input.value.trim();

    if (!value) {
      input.classList.add("invalid");
      if (hasToken) {
        setStatus("Paste a new token to replace the saved one.");
      } else {
        setStatus("Please enter a GitHub token.", "err");
      }
      return;
    }
    input.classList.remove("invalid");

    setBusy(saveBtn, true);

    try {
      await request("/auth/github-token", {
        method: "POST",
        body: JSON.stringify({ github_access_token: value }),
      });

      form.reset();

      let me = null;
      try {
        me = await request("/auth/me");
      } catch (_) {}

      if (me && me.has_github_token) {
        showTokenState(true);
        setStatus("Token saved and verified ✓", "ok");
      } else if (me) {
        showTokenState(false);
        setStatus("Token sent, but the server still reports no token. Check the backend.", "err");
      } else {
        showTokenState(true);
        setStatus("Token saved.", "ok");
      }
    } catch (error) {
      setStatus(error.message || "Failed to save token.", "err");
    } finally {
      setBusy(saveBtn, false);
    }
  };

  // Remove token
  if (removeBtn) {
    removeBtn.onclick = async () => {
      setStatus("");
      removeBtn.disabled = true;

      try {
        // NOTE: needs a DELETE /auth/github-token route on the backend
        await request("/auth/github-token", { method: "DELETE" });
        showTokenState(false);
        setStatus("Token removed. The dashboard warning will appear again.", "ok");
      } catch (error) {
        setStatus(error.message || "Couldn't remove the token.", "err");
      } finally {
        removeBtn.disabled = false;
      }
    };
  }
}

/* ====================== INIT ====================== */

if (document.querySelector("#auth-form")) {
  const currentPage = location.pathname.split("/").pop();
  if (currentPage === "register.html") {
    setupAuth("register");
  } else {
    setupAuth("login");
  }
}

if (document.querySelector("#create-form") && !window.__dashboardSetupDone) {
  window.__dashboardSetupDone = true;
  setupDashboard();
}

if (document.querySelector("#ask") && !window.__chatSetupDone) {
  window.__chatSetupDone = true;
  setupChat();
}

if (document.querySelector("#token-form")) {
  setupProfile();
}

function logout() {
  localStorage.removeItem("devlens_token");
  localStorage.removeItem("devlens_email");
  window.location.replace("login.html");
}

document.querySelectorAll("#logout, #logout-bottom, .logout-btn").forEach((btn) => {
  btn.addEventListener("click", logout);
});

async function upgradeToPro() {
  try {
    const data = await request("/payment/checkout", { method: "POST" });
    window.location.href = data.checkout_url;
  } catch (err) {
    alert(err.message);
  }
}

function showUpgradeModal(reason) {
  const modal = document.getElementById("upgrade-modal");
  const message = document.getElementById("upgrade-message");

  if (!modal || !message) return;

  switch (reason) {
    case "repo_limit":
      message.textContent =
        "You've reached the free limit of 2 repository chats. Upgrade to Pro for unlimited chats.";
      break;
    case "daily_questions":
    case "daily_limit":
      message.textContent =
        "You've reached today's free question limit. Upgrade to Pro for unlimited questions.";
      break;
    default:
      message.textContent = "This feature requires ChatWithRepo Pro.";
  }

  modal.hidden = false;
}

function hideUpgradeModal() {
  const modal = document.getElementById("upgrade-modal");
  if (modal) modal.hidden = true;
}

async function startCheckout() {
  try {
    const data = await request("/payment/checkout", { method: "POST" });
    window.location.href = data.checkout_url;
  } catch (err) {
    console.error(err);
    alert(err.message || "Unable to start checkout.");
  }
}