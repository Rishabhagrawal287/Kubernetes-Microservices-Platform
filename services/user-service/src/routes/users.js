const express = require('express');
const jwt = require('jsonwebtoken');
const User = require('../models/User');
const { requireAuth } = require('../middleware/auth');
const { userRegistrationsTotal } = require('../metrics');

const router = express.Router();

function signToken(user) {
  return jwt.sign({ sub: user._id.toString(), email: user.email }, process.env.JWT_SECRET, {
    expiresIn: '1h',
  });
}

router.post('/register', async (req, res) => {
  const { email, password } = req.body;
  if (!email || !password) {
    return res.status(400).json({ error: 'email and password are required' });
  }

  const existing = await User.findOne({ email });
  if (existing) {
    return res.status(409).json({ error: 'email already registered' });
  }

  const passwordHash = await User.hashPassword(password);
  const user = await User.create({ email, passwordHash });
  userRegistrationsTotal.inc();

  return res.status(201).json({ token: signToken(user), user: { id: user._id, email: user.email } });
});

router.post('/login', async (req, res) => {
  const { email, password } = req.body;
  if (!email || !password) {
    return res.status(400).json({ error: 'email and password are required' });
  }

  const user = await User.findOne({ email });
  if (!user || !(await user.comparePassword(password))) {
    return res.status(401).json({ error: 'invalid credentials' });
  }

  return res.json({ token: signToken(user), user: { id: user._id, email: user.email } });
});

router.get('/me', requireAuth, async (req, res) => {
  const user = await User.findById(req.user.sub).select('-passwordHash');
  if (!user) {
    return res.status(404).json({ error: 'user not found' });
  }
  return res.json({ user });
});

module.exports = router;
