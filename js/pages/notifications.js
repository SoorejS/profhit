import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { escapeHTML } from '../utils/escape.js';
import { showToast } from '../components/toast.js';
let items = [], unreadOnly = false;
function render() {
 const visible = items.filter(item => !unreadOnly || !item.read_at);
 document.getElementById('unreadCount').textContent = items.filter(item => !item.read_at).length;
 document.getElementById('notificationList').innerHTML = visible.length ? visible.map(item => `<article class="card"><p>${escapeHTML(item.message)}</p><small>${new Date(item.created_at).toLocaleString()} · ${item.read_at ? 'Read' : 'Unread'}</small></article>`).join('') : '<p>No notifications to show.</p>';
}
async function load() { try { items = await ApiClient.get('/notifications?limit=100'); render(); } catch (error) { document.getElementById('notificationList').textContent = 'Notifications could not load. Refresh to retry.'; } }
document.addEventListener('DOMContentLoaded', () => {
 if (!ApiClient.isAuthenticated()) { window.location.href = '/login.html'; return; }
 document.getElementById('markRead').addEventListener('click', async () => {try {await ApiClient.post('/notifications/read'); await load();}catch(error){showToast(error.message,'error');}});
 document.getElementById('showAll').addEventListener('click', () => { unreadOnly = false; render(); });
 document.getElementById('showUnread').addEventListener('click', () => { unreadOnly = true; render(); });
 load();
});
