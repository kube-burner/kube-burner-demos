(() => {
  const backendUrl = window.__BACKEND_URL__ || 'http://localhost:8080';
  const listEl = document.getElementById('interaction-list');
  const totalEl = document.getElementById('total-count');
  const sessionsEl = document.getElementById('active-sessions');
  const metricsLinkEl = document.getElementById('metrics-link');
  const buttons = document.querySelectorAll('.actions button');
  const sessionId = crypto.randomUUID();

  metricsLinkEl.textContent = `${backendUrl.replace(/\/$/, '')}/metrics`;

  async function fetchTotals() {
    try {
      const res = await fetch(`${backendUrl}/api/interactions`);
      if (!res.ok) throw new Error(`status ${res.status}`);
      const body = await res.json();
      renderTotals(body.totals || {}, body.totalCount || 0, body.activeSessions || 0);
    } catch (err) {
      console.error('Failed to fetch totals', err);
    }
  }

  function renderTotals(totals, totalCount, activeSessions) {
    listEl.innerHTML = '';
    const entries = Object.entries(totals);
    if (entries.length === 0) {
      listEl.innerHTML = '<li>No interactions recorded yet.</li>';
    } else {
      for (const [action, count] of entries) {
        const li = document.createElement('li');
        li.textContent = `${action.replace('_', ' ')}: ${count}`;
        listEl.appendChild(li);
      }
    }
    totalEl.textContent = totalCount;
    sessionsEl.textContent = activeSessions;
  }

  async function sendInteraction(action) {
    buttons.forEach((btn) => (btn.disabled = true));
    try {
      const res = await fetch(`${backendUrl}/api/interaction`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'x-session-id': sessionId
        },
        body: JSON.stringify({ action, sessionId })
      });
      if (!res.ok) throw new Error(`status ${res.status}`);
      await fetchTotals();
    } catch (err) {
      console.error('Failed to send interaction', err);
      alert('Could not record interaction. Check console for details.');
    } finally {
      buttons.forEach((btn) => (btn.disabled = false));
    }
  }

  buttons.forEach((button) => {
    button.addEventListener('click', () => {
      const action = button.dataset.action;
      sendInteraction(action);
    });
  });

  fetchTotals();
  setInterval(fetchTotals, 5000);
})();
