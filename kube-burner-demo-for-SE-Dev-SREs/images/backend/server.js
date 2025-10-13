const express = require('express');
const cors = require('cors');
const client = require('prom-client');

const app = express();
const port = process.env.PORT || 8080;
const appName = process.env.APP_NAME || 'kube-burner-demo';

app.use(cors());
app.use(express.json());

const register = new client.Registry();
register.setDefaultLabels({ app: appName });
client.collectDefaultMetrics({ register });

const httpRequestDuration = new client.Histogram({
  name: 'http_request_duration_seconds',
  help: 'Histogram of HTTP request processing duration',
  labelNames: ['method', 'route', 'action', 'status_code'],
  buckets: [0.05, 0.1, 0.2, 0.5, 1, 2, 5]
});
register.registerMetric(httpRequestDuration);

const interactionCounter = new client.Counter({
  name: 'app_interactions_total',
  help: 'Total number of recorded user interactions',
  labelNames: ['action']
});
register.registerMetric(interactionCounter);

const activeSessionsGauge = new client.Gauge({
  name: 'app_active_sessions',
  help: 'Current number of unique active sessions observed'
});
register.registerMetric(activeSessionsGauge);

const interactions = new Map();
const sessions = new Map();
const SESSION_TTL_MS = 1.5 * 60 * 1000; // Infinity: keeps sessions for the full demo lifetime

function pruneSessions(now) {
  for (const [sessionId, lastSeen] of sessions.entries()) {
    if (now - lastSeen > SESSION_TTL_MS) {
      sessions.delete(sessionId);
    }
  }
  activeSessionsGauge.set(sessions.size);
}

app.use((req, res, next) => {
  const end = httpRequestDuration.startTimer({ method: req.method, route: req.path, action: req.body?.action || 'unknown' });
  res.on('finish', () => {
    end({ status_code: res.statusCode });
  });
  next();
});

app.get('/healthz', (_req, res) => {
  res.json({ status: 'ok', app: appName });
});

app.get('/api/info', (_req, res) => {
  res.json({
    app: appName,
    message: 'Interactive demo backend for kube-burner',
    availableActions: ['book_ticket', 'cancel_ticket', 'give_feedback']
  });
});

app.get('/api/interactions', (_req, res) => {
  const totals = {};
  let sum = 0;
  interactions.forEach((value, key) => {
    totals[key] = value;
    sum += value;
  });
  res.json({ totals, totalCount: sum, activeSessions: sessions.size });
});

app.post('/api/interaction', (req, res) => {
  const action = req.body?.action || 'unknown';
  const sessionId = req.body?.sessionId || req.get('x-session-id') || 'anonymous';
  const now = Date.now();

  const currentValue = interactions.get(action) || 0;
  interactions.set(action, currentValue + 1);
  interactionCounter.inc({ action });

  sessions.set(sessionId, now);
  pruneSessions(now);

  res.status(201).json({
    ok: true,
    action,
    countForAction: interactions.get(action),
    totalInteractions: Array.from(interactions.values()).reduce((acc, v) => acc + v, 0)
  });
});

app.get('/metrics', async (_req, res) => {
  try {
    res.set('Content-Type', register.contentType);
    res.end(await register.metrics());
  } catch (err) {
    res.status(500).end(err.message);
  }
});

app.use((err, _req, res, _next) => {
  console.error('Unhandled error', err);
  res.status(500).json({ ok: false, error: 'internal_error' });
});

app.listen(port, () => {
  console.log(`Backend listening on port ${port}`);
});
