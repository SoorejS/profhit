// PROPHIT API Configuration
// Change BACKEND_URL to your Render.com backend URL before deploying to Vercel
const CONFIG = {
  // Local development
  // API_URL: 'http://localhost:8080/api',

  // Production (update this after deploying backend to Render)
  API_URL: window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
    ? 'http://localhost:8080/api'
    : 'https://profhit.onrender.com/api'
};
