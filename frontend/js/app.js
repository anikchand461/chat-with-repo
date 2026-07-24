const API = 'http://127.0.0.1:8000';

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

function setupDashboard() {
  loadChats();

  document.querySelector("#create").onclick = () => {
    console.log("Open modal");
    document.querySelector("#modal").hidden = false;
  };

  document.querySelector("#cancel").onclick = () => {
    document.querySelector("#modal").hidden = true;
  };

  const form = document.querySelector("#create-form");

  form.onsubmit = async (e) => {
    e.preventDefault();

    console.log("FORM SUBMITTED");

    const error = document.querySelector("#form-error");
    error.textContent = "";

    try {
      console.log("Sending request...");

      const data = await request("/chat/create", {
        method: "POST",
        body: JSON.stringify({
          owner: document.querySelector("#owner").value.trim(),
          repo: document.querySelector("#repo").value.trim(),
          branch: document.querySelector("#branch").value.trim() || "main",
        }),
      });

      console.log("SUCCESS", data);

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
  const id = new URLSearchParams(location.search).get('id');

  if (!id) return;

  const chats = await request('/chat/list');
  const chat = chats.find((item) => String(item.chat_id) === id);

  if (chat) {
    document.querySelector('#title').textContent = chat.title;
    document.querySelector('#branch').textContent = `Branch: ${chat.branch}`;
  }

  const previous = document.querySelector('#previous');

  if (previous) {
    previous.innerHTML = chats
      .map(
        (item) =>
          `<a class="chat-link" href="chat.html?id=${item.chat_id}">
            ${item.title}
          </a>`
      )
      .join('');
  }

  document.querySelector('#ask').onsubmit = async (e) => {
    e.preventDefault();

    const input = document.querySelector('#question');
    const q = input.value.trim();

    if (!q) return;

    addMessage(q, 'user');
    input.value = '';

    try {
      const data = await request(`/chat/${id}/ask`, {
        method: 'POST',
        body: JSON.stringify({
          question: q,
        }),
      });

      addMessage(data.answer, 'assistant');
    } catch (error) {
      addMessage(error.message, 'assistant');
    }
  };
}

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