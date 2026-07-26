const API = "http://127.0.0.1:8000";

// Change this to your repository (username/repo)
const GITHUB_REPO = "username/repo";

const authToken = localStorage.getItem("devlens_token");

const page = location.pathname.split("/").pop();

if (!authToken && page !== "login.html" && page !== "register.html" && page !== "index.html" && page !== "") {
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
  try {
    const user = await request("/auth/me");
    if (!user.has_github_token) {
      const warning = document.querySelector("#github-warning");
      if (warning) warning.hidden = false;
    }
  } catch (err) {
    console.error("auth/me failed:", err);
  }

  try {
    await loadChats();
  } catch (err) {
    console.error("loadChats failed:", err);
  }
}

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

function addMessage(text, role) {
  const node = document.createElement("div");
  node.className = `message ${role}`;
  node.textContent = text;

  const container = document.querySelector("#messages");
  container.appendChild(node);
  container.scrollTop = container.scrollHeight;
}

/* ==================== profile ==================== */

function setupProfile() {
  document.querySelector("#token-form").onsubmit = async (e) => {
    e.preventDefault();

    try {
      await request("/auth/github-token", {
        method: "POST",
        body: JSON.stringify({
          github_access_token: document.querySelector("#token").value,
        }),
      });

      document.querySelector("#status").textContent = "Token saved.";
      e.target.reset();
    } catch (error) {
      document.querySelector("#status").textContent = error.message;
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