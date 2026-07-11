import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';

document.addEventListener('DOMContentLoaded', async () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    try {
        const data = await ApiClient.get('/me');
        if (data) {
            document.getElementById('sUsername').value = data.username || '';
            document.getElementById('sEmail').value = data.email || '';
            document.getElementById('sTier').value = data.tier || 'Standard';
        }
    } catch (err) {
        console.error("Failed to load profile data", err);
        document.getElementById('sUsername').value = 'Error loading data';
        document.getElementById('sEmail').value = 'Error loading data';
        document.getElementById('sTier').value = 'Error loading data';
    }
});
