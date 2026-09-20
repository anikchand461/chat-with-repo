const API = "https://chat-with-repo-4vwy.onrender.com";
// const API = "http://127.0.0.1:8000"; // local testing only

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

  await refreshGithubWarning();

  try {
    await loadChats();
  } catch (err) {
    console.error("loadChats failed:", err);
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

    messages.forEach((m) => addMessage(m.content, m.role));
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

      try {
        const data = await request(`/chat/${id}/ask`, {
          method: "POST",
          body: JSON.stringify({ question: q }),
        });

        typing.remove();

        if (data.upgrade_required) {
          addMessage(
            "⚠️ " + data.message + "\n\nUpgrade to Pro to continue chatting.",
            "assistant"
          );
          showUpgradeModal(data.reason);
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
    bannerTitle.textContent = has ? "GitHub token configured" : "GitHub token not configured";
    bannerDesc.textContent = has
      ? "Private repos and higher rate limits are available. The dashboard warning is hidden."
      : "Public API limits apply. Add a token for better performance.";

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