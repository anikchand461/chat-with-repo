

const API = 'http://127.0.0.1:8000';
const authToken = localStorage.getItem("devlens_token");

const page = location.pathname.split("/").pop();

if (
    !authToken &&
    page !== "login.html" &&
    page !== "register.html"
) {
    location.href = "login.html";
}

function token() {
  return localStorage.getItem('devlens_token') || '';
}

async function request(path, options = {}) {
  const response = await fetch(API + path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token() ? { Authorization: `Bearer ${token()}` } : {}),
    },
  });

  const text = await response.text();
  const data = text ? JSON.parse(text) : {};

  if (!response.ok) {
    throw Error(data.detail || 'Request failed');
  }

  return data;
}

// ====================== LOGOUT FUNCTION ======================

// ============================================================

function setupAuth(kind) {
  document.querySelector('#auth-form').addEventListener('submit', async (e) => {
    e.preventDefault();

    try {
      const data = await request(`/auth/${kind}`, {
        method: 'POST',
        body: JSON.stringify({
          email: document.querySelector('#email').value,
          password: document.querySelector('#password').value,
        }),
      });

      localStorage.setItem('devlens_token', data.access_token);
      location.href = 'dashboard.html';
    } catch (error) {
      document.querySelector('#error').textContent = error.message;
    }
  });
}

async function setupDashboard() {
  // Check whether the user has configured a GitHub token
  const user = await request("/auth/me");

  if (!user.has_github_token) {
    document.querySelector("#github-warning").hidden = false;
  }

  await loadChats();

  document.querySelector("#create").onclick = () => {
    document.querySelector("#modal").hidden = false;
  };

  document.querySelector("#cancel").onclick = () => {
    document.querySelector("#modal").hidden = true;
  };

  const form = document.querySelector("#create-form");

  form.onsubmit = async (e) => {
    e.preventDefault();

    const error = document.querySelector("#form-error");
    error.textContent = "";

    try {
      const data = await request("/chat/create", {
        method: "POST",
        body: JSON.stringify({
          owner: document.querySelector("#owner").value.trim(),
          repo: document.querySelector("#repo").value.trim(),
          branch: document.querySelector("#branch").value.trim() || "main",
        }),
      });

      await loadChats();

      document.querySelector("#modal").hidden = true;

      location.href = `chat.html?id=${data.chat_id}`;
    } catch (err) {
      console.error(err);
      error.textContent = err.message;
    }
  };
}

async function loadChats() {
  try {
    const chats = await request('/chat/list');

    document.querySelector('#chats').innerHTML = chats.length
      ? chats
          .map(
            (c) =>
              `<a class="chat-link" href="chat.html?id=${c.chat_id}">
                <strong>${c.title}</strong><br>
                <small>branch: ${c.branch}</small>
              </a>`
          )
          .join('')
      : '<p>No chats yet. Create one to index a repository.</p>';
  } catch (error) {
    location.href = 'login.html';
  }
}

async function setupChat() {
    const id = new URLSearchParams(location.search).get("id");

    if (!id) return;

    // Load all chats
    const chats = await request("/chat/list");

    const list = document.querySelector("#chat-list");
    list.innerHTML = "";

    // Current chat
    const current = chats.find(c => c.chat_id == id);

    if (current) {
        document.querySelector("#chat-title").textContent = current.title;
        document.querySelector("#branch").textContent =
            `Branch: ${current.branch}`;
    }

    // Show current chat first
    const sorted = [
        ...chats.filter(c => c.chat_id == id),
        ...chats.filter(c => c.chat_id != id),
    ];

    sorted.forEach(chat => {
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

        messages.forEach(m => {
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

            addMessage(q, "user");
            input.value = "";

            try {
                const data = await request(`/chat/${id}/ask`, {
                    method: "POST",
                    body: JSON.stringify({
                        question: q,
                    }),
                });

                addMessage(data.answer, "assistant");

            } catch (error) {
                addMessage(error.message, "assistant");
            }
        };
    }
}

    // if (!id) return;

    // // ... your existing code ...

    // try {
    //     const messages = await request(`/chat/${id}/messages`);

    //     document.querySelector('#messages').innerHTML = "";

    //     messages.forEach((m) => {
    //         addMessage(m.content, m.role);
    //     });
    // } catch (err) {
    //     console.error(err);
    // }

    // <-- MOVE THIS BLOCK HERE
    // document.querySelector('#ask').onsubmit = async (e) => {
    //     e.preventDefault();

    //     const input = document.querySelector('#question');
    //     const q = input.value.trim();

    //     if (!q) return;

    //     addMessage(q, 'user');
    //     input.value = '';

    //     try {
    //         const data = await request(`/chat/${id}/ask`, {
    //             method: 'POST',
    //             body: JSON.stringify({
    //                 question: q,
    //             }),
    //         });

    //         addMessage(data.answer, 'assistant');
    //     } catch (error) {
    //         addMessage(error.message, 'assistant');
    //     }
    // };
// }

// document.querySelector('#ask').onsubmit = async (e) => {
//     e.preventDefault();

//     const input = document.querySelector('#question');
//     const q = input.value.trim();

//     if (!q) return;

//     addMessage(q, 'user');
//     input.value = '';

//     try {
//         const data = await request(`/chat/${id}/ask`, {
//             method: 'POST',
//             body: JSON.stringify({
//                 question: q,
//             }),
//         });

//         addMessage(data.answer, 'assistant');
//     } catch (error) {
//         addMessage(error.message, 'assistant');
//     }
// };

function addMessage(text, role) {
  const node = document.createElement('div');
  node.className = `message ${role}`;
  node.textContent = text;

  document.querySelector('#messages').appendChild(node);
  node.scrollIntoView({
    behavior: 'smooth',
    block: 'end',
  });
}

function setupProfile() {
  document.querySelector('#token-form').onsubmit = async (e) => {
    e.preventDefault();

    try {
      await request('/auth/github-token', {
        method: 'POST',
        body: JSON.stringify({
          github_access_token: document.querySelector('#token').value,
        }),
      });

      document.querySelector('#status').textContent = 'Token saved.';
      e.target.reset();
    } catch (error) {
      document.querySelector('#status').textContent = error.message;
    }
  };
}

if (document.querySelector("#auth-form")) {
  const page = location.pathname.split("/").pop();

  if (page === "register.html") {
    setupAuth("register");
  } else {
    setupAuth("login");
  }
}

if (document.querySelector("#create-form")) {
  setupDashboard();
}

if (document.querySelector("#ask")) {
  setupChat();
}

if (document.querySelector("#token-form")) {
  setupProfile();
}

const logoutBtn = document.querySelector("#logout");

if (logoutBtn) {
    logoutBtn.addEventListener("click", () => {
        console.log("Logout clicked");

        localStorage.removeItem("devlens_token");

        window.location.replace("login.html");
    });
}