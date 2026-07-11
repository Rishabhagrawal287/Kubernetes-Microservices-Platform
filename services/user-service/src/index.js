const express = require('express');
const mongoose = require('mongoose');
const usersRouter = require('./routes/users');
const { register, httpMetricsMiddleware } = require('./metrics');

const app = express();
app.use(express.json());
app.use(httpMetricsMiddleware);

const PORT = process.env.PORT || 3000;
const MONGO_URI = process.env.MONGO_URI || 'mongodb://localhost:27017/users_db';

let mongoConnected = false;

mongoose
  .connect(MONGO_URI)
  .then(() => {
    mongoConnected = true;
    console.log('user-service connected to MongoDB');
  })
  .catch((err) => {
    console.error('user-service failed to connect to MongoDB:', err.message);
  });

// Liveness: is the process up at all
app.get('/health', (req, res) => {
  res.status(200).json({ status: 'ok', service: 'user-service' });
});

// Readiness: only ready once Mongo is actually connected
app.get('/ready', (req, res) => {
  if (mongoConnected && mongoose.connection.readyState === 1) {
    return res.status(200).json({ status: 'ready', service: 'user-service' });
  }
  return res.status(503).json({ status: 'not-ready', service: 'user-service' });
});

// Scraped by Prometheus via the ServiceMonitor in helm/user-service
app.get('/metrics', async (req, res) => {
  res.set('Content-Type', register.contentType);
  res.end(await register.metrics());
});

app.use('/api/users', usersRouter);

app.listen(PORT, () => {
  console.log(`user-service listening on port ${PORT}`);
});
