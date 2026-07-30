const API = "https://chat-with-repo-4vwy.onrender.com";

// Change this to your repository (username/repo)
const GITHUB_REPO = "shreyaghorui222004/chat-with-repo";

const authToken = localStorage.getItem("devlens_token");

const page = location.pathname.split("/").pop();

if (!authToken && page !== "login.html" && page !== "register.html" && page !== "login.html" && page !== "") {
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

  // Don't throw for subscription limits.
  // Let the caller handle them.

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
      const data = await request(`/auth/${kind}`, {
        method: "POST",
        body: JSON.stringify({
          email: document.querySelector("#email").value,
          password: document.querySelector("#password").value,
        }),
      });

      localStorage.setItem("devlens_token", data.access_token);
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
  // Attach UI handlers first so Create always works
  const createBtn = document.querySelector("#create");
  const cancelBtn = document.querySelector("#cancel");
  const modal = document.querySelector("#modal");
  const form = document.querySelector("#create-form");

  if (createBtn) {
    createBtn.onclick = () => {
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
        const data = await request("/chat/create", {
          method: "POST",
          body: JSON.stringify({
            owner: document.querySelector("#owner").value.trim(),
            repo: document.querySelector("#repo").value.trim(),
            branch: document.querySelector("#branch").value.trim() || "main",
          }),
        });

        if (data.upgrade_required) {
          if (modal) modal.hidden = true;
          showUpgradeModal(data.reason);
          return;
        }

        await loadChats();
        if (modal) modal.hidden = true;
        location.href = `chat.html?id=${data.chat_id}`;
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

  // API calls after handlers are wired
  await refreshGithubWarning();

  try {
    await loadChats();
  } catch (err) {
    console.error("loadChats failed:", err);
  }
}

// Shows/hides the "GitHub token not configured" banner based on the
// user's current state. Explicitly sets `hidden` both ways (not just
// "show if missing") so a token saved on the Profile page correctly
// clears the banner here too.
async function refreshGithubWarning() {
  const warning = document.querySelector("#github-warning");
  if (!warning) return;

  try {
    const user = await request("/auth/me");
    // Explicit both ways so a saved token always hides the banner
    warning.hidden = Boolean(user && user.has_github_token);
  } catch (err) {
    console.error("auth/me failed:", err);
    // On error, leave the banner as-is (don't force-show)
  }
}

// Re-check when returning from Profile (bfcache + normal navigation)
window.addEventListener("pageshow", () => {
  if (document.querySelector("#github-warning")) {
    refreshGithubWarning();
  }
});

document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible" && document.querySelector("#github-warning")) {
    refreshGithubWarning();
  }
});

// Browsers can restore this page from the back/forward cache (bfcache)
// when navigating "back" from Profile instead of doing a fresh load —
// in that case none of the script above re-runs, so a token saved on
// Profile would never clear the banner. Re-check whenever the page
// becomes visible again, which covers both bfcache restores and users
// switching back to this tab.
window.addEventListener("pageshow", (event) => {
  if (event.persisted && document.querySelector("#create-form")) {
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

  // Load all chats
  const chats = await request("/chat/list");

  const list = document.querySelector("#chat-list");
  list.innerHTML = "";

  // Current chat
  const current = chats.find((c) => c.chat_id == id);

  if (current) {
    document.querySelector("#chat-title").textContent = current.title;
    document.querySelector("#branch").textContent = `Branch: ${current.branch}`;
    const header = document.querySelector("#chat-header-title");
    if (header) header.textContent = current.title;
  }

  // Show current chat first
  const sorted = [
    ...chats.filter((c) => c.chat_id == id),
    ...chats.filter((c) => c.chat_id != id),
  ];

  sorted.forEach((chat) => {
    const a = document.createElement("a");

    a.href = `chat.html?id=${chat.chat_id}`;
    a.className = "chat-item";

    if (chat.chat_id == id) {
      a.classList.add("active");
    }

    a.innerHTML = `
            <strong>${chat.title}</strong>
            <small>branch: ${chat.branch}</small>
        `;

    list.appendChild(a);
  });

  // Load old messages
  try {
    const messages = await request(`/chat/${id}/messages`);

    const container = document.querySelector("#messages");
    container.innerHTML = "";

    if (!messages.length) {
      container.innerHTML =
        '<div class="empty"><h2>Ask your repository</h2><p>Ask anything about the indexed codebase — architecture, files, functions or bugs.</p></div>';
    }

    messages.forEach((m) => {
      addMessage(m.content, m.role);
    });
  } catch (err) {
    console.error(err);
  }

  // Ask form
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

      try {
        const data = await request(`/chat/${id}/ask`, {
          method: "POST",
          body: JSON.stringify({
            question: q,
          }),
        });

        typing.remove();

        if (data.upgrade_required) {
          addMessage(
            "⚠️ " + data.message + "\n\nUpgrade to Pro to continue chatting.",
            "assistant"
          );
          return;
        }

        addMessage(data.answer, "assistant");
      } catch (error) {
        typing.remove();
        addMessage(error.message, "assistant");
      }
    };
  }
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

/* ---- lightweight markdown rendering for chat messages ---- */

function escapeHtml(str) {
  return str
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function renderMarkdown(raw) {
  // 1. Pull out fenced code blocks first so nothing inside them gets
  //    touched by the inline/list formatting passes below.
  const codeBlocks = [];
  let text = raw.replace(/```(\w+)?\n?([\s\S]*?)```/g, (_, lang, code) => {
    codeBlocks.push({ lang: (lang || "").trim(), code: code.replace(/\n$/, "") });
    return `\u0000CODEBLOCK${codeBlocks.length - 1}\u0000`;
  });

  // 2. Escape everything else so raw HTML in repo content can't leak in.
  text = escapeHtml(text);

  // 3. Inline code spans.
  text = text.replace(/`([^`\n]+)`/g, (_, code) => `<code>${code}</code>`);

  // 4. Bold, then italic (order matters so **x** isn't eaten by *x*).
  text = text.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  text = text.replace(/__([^_]+)__/g, "<strong>$1</strong>");
  text = text.replace(/(^|[^*])\*([^*\n]+)\*(?!\*)/g, "$1<em>$2</em>");
  text = text.replace(/(^|[^_])_([^_\n]+)_(?!_)/g, "$1<em>$2</em>");

  // 5. Walk line by line to build paragraphs / bullet / numbered lists.
  const lines = text.split("\n");
  let html = "";
  let inUl = false;
  let inOl = false;
  let para = [];

  const closeLists = () => {
    if (inUl) { html += "</ul>"; inUl = false; }
    if (inOl) { html += "</ol>"; inOl = false; }
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
      if (inOl) { html += "</ol>"; inOl = false; }
      if (!inUl) { html += "<ul>"; inUl = true; }
      html += `<li>${bullet[1]}</li>`;
    } else if (numbered) {
      flushPara();
      if (inUl) { html += "</ul>"; inUl = false; }
      if (!inOl) { html += "<ol>"; inOl = true; }
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

  // 6. Re-insert the fenced code blocks as proper <pre><code> panels.
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

// Syntax-highlights every fenced code block inside `scope` using highlight.js
// (loaded via CDN in chat.html). If a language was given in the fence
// (```go, ```js, ...) and hljs recognizes it, that's used; otherwise hljs
// auto-detects, and — if the fence had no language at all — the detected
// name is written into the little header label too.
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
  node.innerHTML = role === "user" ? `<p>${escapeHtml(text)}</p>` : renderMarkdown(text);

  highlightCodeBlocks(node);

  const container = document.querySelector("#messages");
  container.appendChild(node);
  container.scrollTop = container.scrollHeight;
}

// One delegated listener handles every "Copy" button, including ones
// added to messages long after page load.
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

function setupProfile() {
  const form = document.querySelector("#token-form");
  if (!form) return;

  form.onsubmit = async (e) => {
    e.preventDefault();

    const status = document.querySelector("#status");
    const input = document.querySelector("#token");
    const button = form.querySelector('button[type="submit"]');
    const original = button ? button.textContent : "";

    if (status) {
      status.textContent = "";
      status.className = "";
    }

    if (!input || !input.value.trim()) {
      if (status) {
        status.textContent = "Please enter a GitHub token.";
        status.className = "err";
      }
      return;
    }

    if (button) {
      button.disabled = true;
      button.textContent = "Saving...";
    }

    try {
      await request("/auth/github-token", {
        method: "POST",
        body: JSON.stringify({
          github_access_token: input.value.trim(),
        }),
      });

      if (status) {
        status.textContent = "Token saved. Dashboard warning will clear.";
        status.className = "ok";
      }
      form.reset();

      // Optional: confirm backend now reports the token
      try {
        const me = await request("/auth/me");
        if (status && me.has_github_token) {
          status.textContent = "Token saved and verified ✓";
          status.className = "ok";
        } else if (status) {
          status.textContent =
            "Token sent, but /auth/me still reports no token. Check backend.";
          status.className = "err";
        }
      } catch (_) {}
    } catch (error) {
      if (status) {
        status.textContent = error.message || "Failed to save token.";
        status.className = "err";
      }
    } finally {
      if (button) {
        button.disabled = false;
        button.textContent = original;
      }
    }
  };
}

// ====================== INIT ======================

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
  window.location.replace("login.html");
}

document.querySelectorAll("#logout, .logout-btn").forEach((btn) => {
  btn.addEventListener("click", logout);
});

async function upgradeToPro() {
  try {
    const data = await request("/payment/checkout", {
      method: "POST",
    });

    window.location.href = data.checkout_url;
  } catch (err) {
    alert(err.message);
  }
}

async function loadSubscription() {
  try {
    const status = await request("/payment/status");

    const badge = document.querySelector("#plan");

    if (!badge) return;

    badge.textContent = status.plan;
  } catch (err) {
    console.error(err);
  }
}

function showUpgradeModal(reason) {
  const modal = document.getElementById("upgrade-modal");
  const message = document.getElementById("upgrade-message");

  switch (reason) {
    case "repo_limit":
      message.textContent =
        "You've reached your monthly repository limit. Upgrade to Pro to create unlimited repository chats.";
      break;

    case "daily_limit":
      message.textContent =
        "You've reached today's question limit. Upgrade to Pro for unlimited questions.";
      break;

    default:
      message.textContent = "This feature requires Chat WithRepo Pro.";
  }

  modal.hidden = false;
}

function hideUpgradeModal() {
  document.getElementById("upgrade-modal").hidden = true;
}

async function startCheckout() {
  try {
    const data = await request("/payment/checkout", {
      method: "POST",
    });

    window.location.href = data.checkout_url;
  } catch (err) {
    console.error(err);
    alert(err.message || "Unable to start checkout.");
  }
}