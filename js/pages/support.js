import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';

document.addEventListener('DOMContentLoaded', () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }
});

window.submitSupport = (e) => {
    e.preventDefault();
    const category = document.getElementById('sCategory').value;
    const description = document.getElementById('sDescription').value;

    const subject = encodeURIComponent(`PROPHIT Support Request: ${category}`);
    const body = encodeURIComponent(description);
    
    // Open default mail client
    window.location.href = `mailto:support@profhit.com?subject=${subject}&body=${body}`;
};
