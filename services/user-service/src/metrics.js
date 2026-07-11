const client = require('prom-client');

const register = new client.Registry();
client.collectDefaultMetrics({ register });

// ---------- Golden signals (traffic, errors, latency) ----------
const httpRequestDuration = new client.Histogram({
  name: 'http_request_duration_seconds',
  help: 'Duration of HTTP requests in seconds',
  labelNames: ['method', 'route', 'status_code'],
  buckets: [0.01, 0.05, 0.1, 0.3, 0.5, 1, 2, 5],
  registers: [register],
});

const httpRequestsTotal = new client.Counter({
  name: 'http_requests_total',
  help: 'Total number of HTTP requests',
  labelNames: ['method', 'route', 'status_code'],
  registers: [register],
});

// ---------- Business metric ----------
const userRegistrationsTotal = new client.Counter({
  name: 'user_registrations_total',
  help: 'Total number of successful user registrations',
  registers: [register],
});

function httpMetricsMiddleware(req, res, next) {
  const start = process.hrtime();
  res.on('finish', () => {
    const [seconds, nanoseconds] = process.hrtime(start);
    const duration = seconds + nanoseconds / 1e9;
    // req.route is only set once Express has matched a route; fall back to
    // the raw path for 404s so they don't all collapse into one label value.
    const route = req.route ? req.route.path : req.path;
    const labels = { method: req.method, route, status_code: res.statusCode };
    httpRequestDuration.observe(labels, duration);
    httpRequestsTotal.inc(labels);
  });
  next();
}

module.exports = { register, httpMetricsMiddleware, userRegistrationsTotal };
