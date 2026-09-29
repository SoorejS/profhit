import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';
import { escapeHTML } from '../utils/escape.js';

/**
 * PROPHIT - Admin Panel Logic
 */
let currentAdmin = null;

document.addEventListener('DOMContentLoaded', async () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    // Verify Admin Role before rendering
    try {
        currentAdmin = await ApiClient.get('/me');
        if (currentAdmin.role !== 'admin' && currentAdmin.role !== 'super_admin') {
            window.location.href = 'dashboard.html';
            return;
        }
    } catch(err) {
        document.querySelector('.content-wrapper').innerHTML = '<div class="card text-danger" role="alert">Admin data could not load. Check the local API and refresh to retry.</div>';
        return;
    }

    // Initial load is routed through the selected hash tab so each endpoint is
    // requested once.
    const openHash = () => { const name = window.location.hash.slice(1) || 'markets'; const button = document.querySelector('[data-tab="' + (['markets','kyc','withdrawals','analytics','moderation'].includes(name) ? name : 'markets') + '"]'); switchTab(button.dataset.tab, button); };
    window.addEventListener('hashchange', openHash); openHash();
    let searchTimer;
    document.getElementById('adminUserSearch').addEventListener('input', () => { clearTimeout(searchTimer); searchTimer = setTimeout(() => { usersPage = 0; fetchAdminUsers(); }, 300); });
});

function switchTab(tabName, element) {
    // UI Update
    document.querySelectorAll('.admin-tab').forEach(el => el.classList.remove('active'));
    element.classList.add('active');

    document.querySelectorAll('.admin-tab-content').forEach(el => el.classList.add('hidden'));
    document.getElementById(`tab-${tabName}`).classList.remove('hidden');
    if (window.location.hash !== `#${tabName}`) history.replaceState(null, '', `#${tabName}`);

    // Data Fetching
    if (tabName === 'markets') {
        fetchProposedMarkets();
        fetchActiveMarkets();
    }
    if (tabName === 'analytics') fetchAnalytics();
    if (tabName === 'moderation') { fetchAdminUsers(); fetchReports(); }
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
                <td class="font-semibold text-primary">${escapeHTML(m.title)}</td>
                <td><span class="badge badge-outline">${escapeHTML(m.category)}</span></td>
                <td>ID: ${m.creator_id}</td>
                <td>
                    <button class="btn btn-yes" style="padding: 0.25rem 0.75rem; font-size: 0.8rem;" onclick="approveMarket(${m.id})">Approve</button>
                </td>
            </tr>
        `).join('');
    } catch (err) {
        tbody.innerHTML = '<tr><td colspan="4" class="text-center text-danger">Failed to load proposals.</td></tr>';
    }
}

async function fetchActiveMarkets() {
    const tbody = document.querySelector('#activeMarketsTable tbody');
    if (!tbody) return;
    try {
        const markets = await ApiClient.get('/markets');
        if (!markets || markets.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No active markets.</td></tr>';
            return;
        }

        // Filter for Open/Closed (not Proposed)
        const activeOrClosed = markets.filter(m => m.resolution_status !== 'Proposed');

        if (activeOrClosed.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No active markets.</td></tr>';
            return;
        }

        tbody.innerHTML = activeOrClosed.map(m => {
            let statusBadge = m.resolution_status === 'Open' ? `<span class="badge badge-primary">Open</span>` : `<span class="badge badge-success">${escapeHTML(m.resolution_status)}</span>`;
            
            // If the market is open or closed but not resolved, we can resolve it
            let resolveBtn = '';
            if (['Locked', 'Awaiting Resolution'].includes(m.resolution_status)) {
                resolveBtn = `<button class="btn btn-yes" style="padding: 0.25rem 0.75rem; font-size: 0.8rem;" onclick="resolveMarket(${m.id})">Resolve</button>`;
            }

            return `
            <tr>
                <td class="font-semibold text-primary">${escapeHTML(m.title)}</td>
                <td><span class="badge badge-outline">${escapeHTML(m.category)}</span></td>
                <td>${statusBadge}</td>
                <td>
                    ${resolveBtn}
                </td>
            </tr>
            `;
        }).join('');
    } catch (err) {
        tbody.innerHTML = '<tr><td colspan="4" class="text-center text-danger">Failed to load active markets.</td></tr>';
    }
}

async function approveMarket(id) {
    if (!confirm("Make this market live?")) return;
    try {
        await ApiClient.post(`/markets/${id}/approve`);
        showToast("Market approved successfully", "success");
        fetchProposedMarkets();
    } catch (err) {
        showToast(err.message, "error");
    }
}

async function resolveMarket(id) {
    const outcome = prompt("Enter the winning option exactly as it appears (e.g. Yes or No):");
    if (!outcome) return;

    if (!confirm(`Are you sure you want to resolve Market ${id} with winner: ${outcome}? This will trigger payouts and cannot be undone.`)) return;

    try {
        const res = await ApiClient.post(`/markets/${id}/resolve`, { winner: outcome });
        showToast(`Market resolved! ${res.winners_paid} winners paid.`, "success");
        fetchActiveMarkets();
    } catch (err) {
        showToast(err.message, "error");
    }
}

async function fetchAdminKyc() {
    const tbody = document.querySelector('#kycTable tbody');
    try {
        const res = await ApiClient.get('/admin/kyc');
        const reqs = Array.isArray(res) ? res : (res?.items || []);
        if (!reqs || reqs.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="text-center text-muted">No KYC verification attempts found.</td></tr>';
            return;
        }

        tbody.innerHTML = reqs.map(r => {
            let badge = '';
            if (r.status === 'Verified') badge = `<span class="badge badge-success">Verified</span>`;
            else if (r.status === 'Rejected') badge = `<span class="badge badge-danger">Rejected</span>`;
            else badge = `<span class="badge badge-warning">${escapeHTML(r.status)}</span>`;

            return `
                <tr>
                    <td><div class="font-semibold">${escapeHTML(r.username)}</div><div class="text-muted" style="font-size: 0.75rem;">ID: ${r.user_id}</div></td>
                    <td>${badge}</td>
                    <td class="font-mono text-muted" style="font-size: 0.85rem;">${escapeHTML(r.provider_reference)}</td>
                    <td class="text-muted" style="font-size: 0.85rem;">${new Date(r.created_at).toLocaleString()}</td>
                    <td class="text-danger" style="font-size: 0.85rem;">${escapeHTML(r.failure_reason || '--')}</td>
                </tr>
            `;
        }).join('');
    } catch (err) {
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
                <td class="font-bold text-gold">INR ${w.amount}</td>
                <td><span class="badge badge-warning">${escapeHTML(w.status)}</span></td>
                <td>
                    <button class="btn btn-yes" style="padding: 0.25rem 0.75rem; font-size: 0.8rem;" onclick="processWithdrawal(${w.id}, 'Approve')">Approve</button>
                    <button class="btn btn-no" style="padding: 0.25rem 0.75rem; font-size: 0.8rem;" onclick="processWithdrawal(${w.id}, 'Reject')">Reject</button>
                </td>
            </tr>
        `).join('');
    } catch (err) {
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
window.fetchActiveMarkets = fetchActiveMarkets;
window.approveMarket = approveMarket;
window.resolveMarket = resolveMarket;
window.fetchAdminKyc = fetchAdminKyc;
window.fetchWithdrawals = fetchWithdrawals;
window.processWithdrawal = processWithdrawal;

async function fetchAnalytics() {
    try {
        const stats = await ApiClient.get('/admin/stats');
        document.getElementById('adminTotalUsers').textContent = stats.users.total;
        document.getElementById('adminTotalPredictions').textContent = stats.trades.total;
        const health = await ApiClient.get('/health');
        document.getElementById('adminHealth').textContent = health.message;
    } catch (error) {
        document.getElementById('adminHealth').textContent = error.message;
    }
}

let usersRequest = 0, usersPage = 0;
window.changeUsersPage = dir => { usersPage = Math.max(0, usersPage + dir); fetchAdminUsers(); };
async function fetchAdminUsers() {
    const request = ++usersRequest;
    const list = document.getElementById('adminUsers');
    try {
        const response = await ApiClient.get('/admin/users?limit=100&offset=' + (usersPage * 100) + '&search=' + encodeURIComponent(document.getElementById('adminUserSearch').value));
        if (request !== usersRequest) return;
        document.getElementById("usersPrev").disabled = usersPage === 0;
        document.getElementById("usersNext").disabled = response.users.length < 100;
        list.replaceChildren();
        if (!response.users.length) list.textContent = 'No matching users.';
        for (const user of response.users) {
            const row = document.createElement('div');
            row.className = 'flex justify-between items-center mb-2';
            const name = document.createElement('span');
            name.textContent = `${user.username} · ${user.role} · ${user.is_active ? 'Active' : 'Banned'}`;
            const button = document.createElement('button');
            button.className = 'btn btn-outline';
            button.textContent = user.is_active ? 'Ban' : 'Unban';
            const protectedTarget = user.id === currentAdmin?.id || user.role === 'super_admin' ||
                (user.role === 'admin' && currentAdmin?.role !== 'super_admin');
            if (protectedTarget) {
                button.disabled = true;
                button.title = user.id === currentAdmin?.id ? 'You cannot moderate your own account' : 'This account cannot be moderated by your role';
            }
            button.addEventListener('click', async () => {
                if (!confirm(`${button.textContent} ${user.username}?`)) return;
                try { await ApiClient.post(`/admin/users/${user.id}/${user.is_active ? 'ban' : 'unban'}`); await fetchAdminUsers(); }
                catch (error) { showToast(error.message, 'error'); }
            });
            row.append(name, button); list.append(row);
        }
    } catch (error) { if (request === usersRequest) list.textContent = error.message; }
}

async function fetchReports() {
    const list = document.getElementById('adminReports');
    try {
        const reports = await ApiClient.get('/admin/reports?status=Pending&limit=100');
        list.replaceChildren();
        if (!reports.length) list.textContent = 'No pending reports.';
        for (const report of reports) {
            const row = document.createElement('article');
            const text = document.createElement('p');
            text.textContent = `${report.target_type} #${report.target_id}: ${report.reason} — ${report.description}`;
            const button = document.createElement('button');
            button.className = 'btn btn-outline'; button.textContent = 'Review report';
            button.addEventListener('click', async () => {
                const actions = report.target_type === 'User' ? 'Dismiss, Mute, Suspend, Ban' : report.target_type === 'Comment' ? 'Dismiss, DeleteComment' : 'Dismiss';
                const action = prompt(`Choose an action: ${actions}`);
                if (!action || !confirm(`Apply ${action} to report #${report.id}? Suspensions last 7 days.`)) return;
                try { await ApiClient.post(`/admin/reports/${report.id}/resolve`, {action, duration_days: 7}); await fetchReports(); }
                catch (error) { showToast(error.message, 'error'); }
            });
            row.append(text, button); list.append(row);
        }
    } catch (error) { list.textContent = error.message; }
}
window.fetchAdminUsers = fetchAdminUsers;
