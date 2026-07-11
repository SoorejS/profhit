import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';

document.addEventListener('DOMContentLoaded', () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }
});
