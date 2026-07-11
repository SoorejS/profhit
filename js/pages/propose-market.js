import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';

document.addEventListener('DOMContentLoaded', () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }
});

window.submitProposal = async (e) => {
    e.preventDefault();

    const title = document.getElementById('pTitle').value;
    const category = document.getElementById('pCategory').value;
    const description = document.getElementById('pDescription').value;
    const source = document.getElementById('pSource').value;
    const lockTime = document.getElementById('pLockTime').value;

    if (!title || !category || !description || !source || !lockTime) {
        showToast("Please fill all required fields.", "error");
        return;
    }

    const lockDate = new Date(lockTime);
    if (lockDate <= new Date()) {
        showToast("Lock time must be in the future.", "error");
        return;
    }

    const payload = {
        title: title,
        category: category,
        description: description,
        resolution_source: source,
        lock_time: lockDate.toISOString(),
        options: '["Yes","No"]',
    };

    const btn = document.getElementById('submitBtn');
    btn.disabled = true;
    btn.innerHTML = '<i class="ph ph-spinner ph-spin"></i> Submitting...';

    try {
        await ApiClient.post('/markets/propose', payload);
        showToast("Market proposal submitted successfully! Redirecting to dashboard...", "success");
        setTimeout(() => {
            window.location.href = 'dashboard.html';
        }, 2000);
    } catch (err) {
        showToast(err.message, "error");
        btn.disabled = false;
        btn.innerHTML = 'Submit Proposal for Review';
    }
};
