import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from './api/client.js';
import { showToast } from './components/toast.js';

/**
 * PROPHIT - Admin Panel Logic
 */

document.addEventListener('DOMContentLoaded', async () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    // Verify Admin Role before rendering
    try {
        const user = await ApiClient.get('/me');
        if (user.role !== 'admin' && user.role !== 'super_admin') {
            window.location.href = 'dashboard.html';
            return;
        }
    } catch(err) {
        window.location.href = 'login.html';
        return;
    }

    // Initial load
    fetchProposedMarkets();
});

function switchTab(tabName, element) {
    // UI Update
    document.querySelectorAll('.admin-tab').forEach(el => el.classList.remove('active'));
    element.classList.add('active');

    document.querySelectorAll('.admin-tab-content').forEach(el => el.classList.add('hidden'));
    document.getElementById(`tab-${tabName}`).classList.remove('hidden');

    // Data Fetching
    if (tabName === 'markets') fetchProposedMarkets();
    if (tabName === 'kyc') fetchAdminKyc();
    if (tabName === 'withdrawals') fetchWithdrawals();
}

async function fetchProposedMarkets() {
    const tbody = document.querySelector('#proposalsTable tbody');
    try {
        const markets = await ApiClient.get('/markets/proposed');
        if (!markets || markets.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No pending market proposals.</td></tr>';
            return;
        }

        tbody.innerHTML = markets.map(m => `
            <tr>
                <td class="font-semibold text-primary">${m.title}</td>
                <td><span class="badge badge-outline">${m.category}</span></td>
                <td>ID: ${m.created_by}</td>
                <td>
                    <button class="btn btn-yes" style="padding: 0.25rem 0.75rem; font-size: 0.8rem;" onclick="approveMarket(${m.id})">Approve</button>
                </td>
            </tr>
        `).join('');
    } catch (err) {
        console.error(err);
        tbody.innerHTML = '<tr><td colspan="4" class="text-center text-danger">Failed to load proposals.</td></tr>';
    }
}

async function approveMarket(id) {
    if (!confirm("Make this market live?")) return;
    try {
        await ApiClient.post(`/admin/markets/${id}/approve`);
        showToast("Market approved successfully", "success");
        fetchProposedMarkets();
    } catch (err) {
        showToast(err.message, "error");
    }
}

async function fetchAdminKyc() {
    const tbody = document.querySelector('#kycTable tbody');
    try {
        const reqs = await ApiClient.get('/admin/kyc');
        if (!reqs || reqs.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="text-center text-muted">No KYC verification attempts found.</td></tr>';
            return;
        }

        tbody.innerHTML = reqs.map(r => {
            let badge = '';
            if (r.status === 'Verified') badge = `<span class="badge badge-success">Verified</span>`;
            else if (r.status === 'Rejected') badge = `<span class="badge badge-danger">Rejected</span>`;
            else badge = `<span class="badge badge-warning">${r.status}</span>`;

            return `
                <tr>
                    <td><div class="font-semibold">${r.username}</div><div class="text-muted" style="font-size: 0.75rem;">ID: ${r.user_id}</div></td>
                    <td>${badge}</td>
                    <td class="font-mono text-muted" style="font-size: 0.85rem;">${r.provider_reference}</td>
                    <td class="text-muted" style="font-size: 0.85rem;">${new Date(r.created_at).toLocaleString()}</td>
                    <td class="text-danger" style="font-size: 0.85rem;">${r.failure_reason || '--'}</td>
                </tr>
            `;
        }).join('');
    } catch (err) {
        console.error(err);
        tbody.innerHTML = '<tr><td colspan="5" class="text-center text-danger">Failed to load KYC logs.</td></tr>';
    }
}

async function fetchWithdrawals() {
    const tbody = document.querySelector('#withdrawalsTable tbody');
    try {
        const reqs = await ApiClient.get('/admin/withdrawals');
        if (!reqs || reqs.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No pending withdrawals.</td></tr>';
            return;
        }

        tbody.innerHTML = reqs.map(w => `
            <tr>
                <td>User ID: ${w.user_id}</td>
                <td class="font-bold text-gold">${w.amount} PTS</td>
                <td><span class="badge badge-warning">${w.status}</span></td>
                <td>
                    <button class="btn btn-yes" style="padding: 0.25rem 0.75rem; font-size: 0.8rem;" onclick="processWithdrawal(${w.id}, 'Approve')">Approve</button>
                    <button class="btn btn-no" style="padding: 0.25rem 0.75rem; font-size: 0.8rem;" onclick="processWithdrawal(${w.id}, 'Reject')">Reject</button>
                </td>
            </tr>
        `).join('');
    } catch (err) {
        console.error(err);
        tbody.innerHTML = '<tr><td colspan="4" class="text-center text-danger">Failed to load withdrawals.</td></tr>';
    }
}

async function processWithdrawal(id, action) {
    if (!confirm(`${action} this withdrawal?`)) return;
    try {
        await ApiClient.post(`/admin/withdrawals/${id}/${action.toLowerCase()}`);
        showToast(`Withdrawal ${action.toLowerCase()}d successfully`, "success");
        fetchWithdrawals();
    } catch (err) {
        showToast(err.message, "error");
    }
}

window.switchTab = switchTab;
window.fetchProposedMarkets = fetchProposedMarkets;
window.approveMarket = approveMarket;
window.resolveMarket = resolveMarket;
window.fetchKycRequests = fetchKycRequests;
window.reviewKyc = reviewKyc;
window.fetchWithdrawals = fetchWithdrawals;
window.approveWithdrawal = approveWithdrawal;
window.rejectWithdrawal = rejectWithdrawal;
