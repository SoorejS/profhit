// ============================================================
// PROPHIT Admin Panel — admin.js
// ============================================================

const API = (typeof CONFIG !== 'undefined' ? CONFIG.API_URL : null) || window.API_URL || 'http://localhost:8080/api';

let adminToken = localStorage.getItem('admin_token');
let adminUser  = JSON.parse(localStorage.getItem('admin_user') || 'null');

// Roles that have access to the admin panel
const ADMIN_ROLES = ['super_admin', 'admin', 'content_creator', 'it_support'];

// ============================================================
// STARTUP
// ============================================================
(function init() {
    if (adminToken && adminUser && ADMIN_ROLES.includes(adminUser.role)) {
        showAdminApp();
    } else {
        document.getElementById('adminLoginGate').style.display = 'flex';
        document.getElementById('adminApp').style.display = 'none';
    }
})();

// ============================================================
// AUTH
// ============================================================
async function adminLogin() {
    const email    = document.getElementById('adminEmail').value.trim();
    const password = document.getElementById('adminPassword').value;
    const errEl    = document.getElementById('loginError');

    errEl.style.display = 'none';

    if (!email || !password) {
        showLoginError('Please enter your email and password.');
        return;
    }

    try {
        const res  = await fetch(`${API}/auth/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password })
        });
        const data = await res.json();

        if (!res.ok) {
            showLoginError(data.error || 'Login failed.');
            return;
        }

        // Check if user has any admin-level role
        if (!ADMIN_ROLES.includes(data.user.role)) {
            showLoginError('Access denied. This portal is for staff only.');
            return;
        }

        adminToken = data.token;
        adminUser  = data.user;
        localStorage.setItem('admin_token', adminToken);
        localStorage.setItem('admin_user', JSON.stringify(adminUser));

        showAdminApp();
    } catch (err) {
        showLoginError('Cannot connect to server. Is the backend running?');
    }
}

// Allow pressing Enter on password field
document.getElementById('adminPassword')?.addEventListener('keydown', e => {
    if (e.key === 'Enter') adminLogin();
});

function showLoginError(msg) {
    const el = document.getElementById('loginError');
    el.textContent = msg;
    el.style.display = 'block';
}

function adminLogout() {
    localStorage.removeItem('admin_token');
    localStorage.removeItem('admin_user');
    location.reload();
}

function getHeaders() {
    return { 'Content-Type': 'application/json', 'Authorization': `Bearer ${adminToken}` };
}

// ============================================================
// SHOW APP
// ============================================================
function showAdminApp() {
    document.getElementById('adminLoginGate').style.display = 'none';
    document.getElementById('adminApp').style.display = 'flex';

    // Populate user info in sidebar
    document.getElementById('adminUserInfo').innerHTML = `
        <div class="username">${adminUser.username}</div>
        <div class="email">${adminUser.email}</div>
        <div style="margin-top:8px;"><span class="role-badge badge-${adminUser.role}">${formatRole(adminUser.role)}</span></div>
    `;

    // Hide tabs not permitted for this role
    applyRoleVisibility();

    // Load initial data
    loadDashboard();
    setupAdminCreateForm();
}

// ============================================================
// ROLE VISIBILITY — hide tabs/sections the user can't access
// ============================================================
function applyRoleVisibility() {
    const role = adminUser.role;

    // Sidebar nav items
    document.querySelectorAll('.admin-only').forEach(el => {
        el.style.display = ['super_admin', 'admin', 'it_support'].includes(role) ? '' : 'none';
    });
    document.querySelectorAll('.super-only').forEach(el => {
        el.style.display = role === 'super_admin' ? '' : 'none';
    });
    // Content sections
    document.querySelectorAll('.content-creator-only').forEach(el => {
        el.style.display = ['super_admin', 'admin', 'content_creator'].includes(role) ? '' : 'none';
    });
    document.querySelectorAll('.admin-only-section').forEach(el => {
        el.style.display = ['super_admin', 'admin'].includes(role) ? '' : 'none';
    });
}

// ============================================================
// TAB SWITCHING
// ============================================================
function switchTab(tabName, btn) {
    document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.nav-item').forEach(b => b.classList.remove('active'));

    document.getElementById(`tab-${tabName}`)?.classList.add('active');
    btn?.classList.add('active');

    // Load data for the tab
    if (tabName === 'dashboard') loadDashboard();
    else if (tabName === 'markets') loadMarkets();
    else if (tabName === 'users') loadUsers();
    else if (tabName === 'roles') loadRolesTab();
    else if (tabName === 'queues') loadQueues();
    else if (tabName === 'logs') loadLogs();
}

// ============================================================
// DASHBOARD
// ============================================================
async function loadDashboard() {
    try {
        const res  = await fetch(`${API}/admin/stats`, { headers: getHeaders() });
        const data = await res.json();

        if (!res.ok) { console.warn('Stats error:', data.error); return; }

        document.getElementById('statTotalUsers').textContent    = data.users.total;
        document.getElementById('statActiveUsers').textContent   = data.users.active;
        document.getElementById('statBannedUsers').textContent   = data.users.banned;
        document.getElementById('statOpenMarkets').textContent   = data.markets.open;
        document.getElementById('statTotalTrades').textContent   = data.trades.total;
        document.getElementById('statVolume').textContent        = Number(data.trades.total_volume).toLocaleString();

        // Role breakdown
        const roles = data.users.by_role || [];
        const roleColors = {
            super_admin: '#FCE883', admin: '#93C5FD',
            content_creator: '#C4B5FD', it_support: '#67E8F9', user: '#9CA3AF'
        };
        document.getElementById('roleBreakdownContent').innerHTML = roles.map(r => `
            <div class="role-item">
                <div class="role-count" style="color:${roleColors[r.role] || '#fff'}">${r.count}</div>
                <div class="role-name">${formatRole(r.role)}</div>
            </div>
        `).join('');
    } catch (err) {
        console.error('Dashboard error:', err);
    }
}

// ============================================================
// MARKETS
// ============================================================
async function loadMarkets() {
    // Active markets
    try {
        const res     = await fetch(`${API}/markets`, { headers: getHeaders() });
        const markets = await res.json();
        const tbody   = document.querySelector('#marketsTable tbody');
        if (!tbody) return;

        tbody.innerHTML = markets.length === 0
            ? `<tr><td colspan="5" style="text-align:center; color:var(--text-secondary); padding:2rem;">No open markets</td></tr>`
            : markets.map(m => `
                <tr>
                    <td>#${m.id}</td>
                    <td style="max-width:300px;">${m.title}</td>
                    <td>${Number(m.volume).toLocaleString()}</td>
                    <td><span class="badge-active">Open</span></td>
                    <td style="display:flex; gap:6px;">
                        <button class="btn-sm btn-success" onclick="resolveMarket(${m.id},'Yes')">Yes ✓</button>
                        <button class="btn-sm btn-ban" onclick="resolveMarket(${m.id},'No')">No ✗</button>
                    </td>
                </tr>
            `).join('');
    } catch(e) { console.error(e); }

    // Proposals
    try {
        const res  = await fetch(`${API}/markets/proposed`, { headers: getHeaders() });
        const data = await res.json();
        const tbody = document.querySelector('#proposalsTable tbody');
        if (!tbody) return;

        document.getElementById('proposalCount').textContent = data.length;

        tbody.innerHTML = data.length === 0
            ? `<tr><td colspan="4" style="text-align:center; color:var(--text-secondary); padding:2rem;">No pending proposals 🎉</td></tr>`
            : data.map(m => `
                <tr>
                    <td style="max-width:280px;">${m.title}</td>
                    <td>${m.category}</td>
                    <td>#${m.creator_id}</td>
                    <td>
                        <button class="btn-sm btn-success" onclick="approveProposal(${m.id})">Approve</button>
                    </td>
                </tr>
            `).join('');
    } catch(e) { console.error(e); }
}

async function resolveMarket(id, outcome) {
    confirmAction(
        `Resolve Market #${id} to ${outcome}?`,
        `This will distribute payouts to all winning traders. This action is irreversible.`,
        async () => {
            const res  = await fetch(`${API}/markets/${id}/resolve`, {
                method: 'POST',
                headers: getHeaders(),
                body: JSON.stringify({ winner: outcome })
            });
            const data = await res.json();
            if (res.ok) { showToast(`✅ Market resolved to ${outcome}!`); loadMarkets(); }
            else showToast(`❌ ${data.error}`, true);
        }
    );
}

async function approveProposal(id) {
    const res  = await fetch(`${API}/markets/${id}/approve`, { method: 'POST', headers: getHeaders() });
    const data = await res.json();
    if (res.ok) { showToast('✅ Market is now live!'); loadMarkets(); }
    else showToast(`❌ ${data.error}`, true);
}

function setupAdminCreateForm() {
    const form = document.getElementById('adminCreateForm');
    if (!form) return;
    form.addEventListener('submit', async e => {
        e.preventDefault();
        const res = await fetch(`${API}/markets`, {
            method: 'POST',
            headers: getHeaders(),
            body: JSON.stringify({
                title:       document.getElementById('adminTitle').value,
                description: document.getElementById('adminDesc').value,
                category:    document.getElementById('adminCategory').value,
                end_date:    document.getElementById('adminDate').value + 'T23:59:59Z'
            })
        });
        const data = await res.json();
        if (res.ok) { showToast('✅ Market created!'); form.reset(); loadMarkets(); }
        else showToast(`❌ ${data.error}`, true);
    });
}

// ============================================================
// USERS
// ============================================================
let allUsers = [];

async function loadUsers() {
    try {
        const res  = await fetch(`${API}/admin/users`, { headers: getHeaders() });
        const data = await res.json();
        allUsers   = data.users || [];
        renderUsers(allUsers);
    } catch(e) { console.error(e); }
}

function filterUsers() {
    const search = document.getElementById('userSearch').value.toLowerCase();
    const role   = document.getElementById('roleFilter').value;
    const status = document.getElementById('statusFilter').value;

    const filtered = allUsers.filter(u => {
        const matchSearch = !search || u.username.toLowerCase().includes(search) || u.email.toLowerCase().includes(search);
        const matchRole   = !role   || u.role === role;
        const matchStatus = !status || (status === 'active' ? u.is_active : !u.is_active);
        return matchSearch && matchRole && matchStatus;
    });
    renderUsers(filtered);
}

function renderUsers(users) {
    const tbody = document.querySelector('#usersTable tbody');
    if (!tbody) return;

    tbody.innerHTML = users.length === 0
        ? `<tr><td colspan="9" style="text-align:center; color:var(--text-secondary); padding:2rem;">No users found</td></tr>`
        : users.map(u => {
            const canModify = adminUser.role === 'super_admin' ||
                              (adminUser.role !== 'super_admin' && u.role !== 'super_admin' && u.id !== adminUser.id);
            const banBtn = canModify
                ? (u.is_active
                    ? `<button class="btn-sm btn-ban" onclick="banUser(${u.id},'${u.username}')">Ban</button>`
                    : `<button class="btn-sm btn-unban" onclick="unbanUser(${u.id},'${u.username}')">Unban</button>`)
                : '—';
            return `
                <tr>
                    <td>#${u.id}</td>
                    <td><strong>${u.username}</strong></td>
                    <td style="color:var(--text-secondary);">${u.email}</td>
                    <td><span class="role-badge badge-${u.role}">${formatRole(u.role)}</span></td>
                    <td>${u.tier}</td>
                    <td>${u.points.toLocaleString()}</td>
                    <td>${u.kyc_status ? '✅' : '—'}</td>
                    <td><span class="${u.is_active ? 'badge-active' : 'badge-banned'}">${u.is_active ? 'Active' : 'Banned'}</span></td>
                    <td>${banBtn}</td>
                </tr>
            `;
        }).join('');
}

async function banUser(id, username) {
    confirmAction(`Ban @${username}?`, `They will be locked out immediately.`, async () => {
        const res  = await fetch(`${API}/admin/users/${id}/ban`, { method: 'POST', headers: getHeaders() });
        const data = await res.json();
        if (res.ok) { showToast(`✅ @${username} has been banned.`); loadUsers(); }
        else showToast(`❌ ${data.error}`, true);
    });
}

async function unbanUser(id, username) {
    const res  = await fetch(`${API}/admin/users/${id}/unban`, { method: 'POST', headers: getHeaders() });
    const data = await res.json();
    if (res.ok) { showToast(`✅ @${username} has been reinstated.`); loadUsers(); }
    else showToast(`❌ ${data.error}`, true);
}

// ============================================================
// ROLE ASSIGNMENT (super_admin only)
// ============================================================
async function loadRolesTab() {
    if (adminUser.role !== 'super_admin') return;
    try {
        const res  = await fetch(`${API}/admin/users`, { headers: getHeaders() });
        const data = await res.json();
        const users = data.users || [];
        const tbody = document.querySelector('#rolesTable tbody');

        tbody.innerHTML = users.map(u => `
            <tr>
                <td>#${u.id}</td>
                <td><strong>${u.username}</strong></td>
                <td style="color:var(--text-secondary);">${u.email}</td>
                <td><span class="role-badge badge-${u.role}">${formatRole(u.role)}</span></td>
                <td>
                    <div style="display:flex; gap:8px; align-items:center;">
                        <select class="role-select" id="role-select-${u.id}">
                            <option value="super_admin" ${u.role==='super_admin'?'selected':''}>Super Admin</option>
                            <option value="admin" ${u.role==='admin'?'selected':''}>Admin</option>
                            <option value="content_creator" ${u.role==='content_creator'?'selected':''}>Content Creator</option>
                            <option value="it_support" ${u.role==='it_support'?'selected':''}>IT Support</option>
                            <option value="user" ${u.role==='user'?'selected':''}>User</option>
                        </select>
                        <button class="btn-sm btn-success" onclick="assignRole(${u.id},'${u.username}')">Apply</button>
                    </div>
                </td>
            </tr>
        `).join('');
    } catch(e) { console.error(e); }
}

async function assignRole(id, username) {
    const newRole = document.getElementById(`role-select-${id}`)?.value;
    if (!newRole) return;

    confirmAction(
        `Change @${username}'s role to ${formatRole(newRole)}?`,
        `This will immediately change their permissions.`,
        async () => {
            const res  = await fetch(`${API}/admin/users/${id}/role`, {
                method: 'PUT',
                headers: getHeaders(),
                body: JSON.stringify({ role: newRole })
            });
            const data = await res.json();
            if (res.ok) { showToast(`✅ Role updated!`); loadRolesTab(); }
            else showToast(`❌ ${data.error}`, true);
        }
    );
}

// ============================================================
// ACTIVITY LOGS
// ============================================================
async function loadLogs() {
    try {
        const res    = await fetch(`${API}/stats/activity`, { headers: getHeaders() });
        const trades = await res.json();
        const tbody  = document.getElementById('logsBody');

        tbody.innerHTML = (trades || []).map(t => `
            <tr>
                <td style="color:var(--text-secondary); font-size:0.8rem;">${new Date(t.created_at).toLocaleString()}</td>
                <td><strong>${t.username || '#'+t.user_id}</strong></td>
                <td>Bought <span style="color:${t.outcome==='Yes'?'var(--green)':'var(--red)'}">${t.outcome}</span> — ${t.amount} pts</td>
                <td style="color:var(--text-secondary);">#${t.market_id}</td>
            </tr>
        `).join('');
    } catch(e) { console.error(e); }
}

// ============================================================
// CONFIRM DIALOG
// ============================================================
let _confirmCallback = null;

function confirmAction(title, msg, callback) {
    document.getElementById('confirmTitle').textContent = title;
    document.getElementById('confirmMsg').textContent   = msg;
    _confirmCallback = callback;
    document.getElementById('confirmModal').style.display = 'flex';
}

document.getElementById('confirmBtn').addEventListener('click', async () => {
    closeConfirm();
    if (_confirmCallback) await _confirmCallback();
    _confirmCallback = null;
});

function closeConfirm() {
    document.getElementById('confirmModal').style.display = 'none';
}

// ============================================================
// TOAST
// ============================================================
function showToast(msg, isError = false) {
    let toast = document.getElementById('adminToast');
    if (!toast) {
        toast = document.createElement('div');
        toast.id = 'adminToast';
        document.body.appendChild(toast);
    }
    toast.style.borderLeftColor = isError ? 'var(--red)' : 'var(--green)';
    toast.textContent = msg;
    toast.style.opacity = '1';
    toast.style.transform = 'translateY(0)';
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateY(20px)';
    }, 4000);
}

// ============================================================
// HELPERS
// ============================================================
function formatRole(role) {
    return {
        super_admin:     'Super Admin',
        admin:           'Admin',
        content_creator: 'Content Creator',
        it_support:      'IT Support',
        user:            'User'
    }[role] || role;
}

// ============================================================
// QUEUES (KYC & WITHDRAWALS)
// ============================================================
async function loadQueues() {
    await Promise.all([loadKycQueue(), loadWithdrawalQueue()]);
}

async function loadKycQueue() {
    const tbody = document.getElementById('kycQueueBody');
    try {
        const res = await fetch(`${API}/admin/kyc-requests`, { headers: getHeaders() });
        const data = await res.json();
        
        if (!res.ok) throw new Error(data.error);
        if (!data || data.length === 0) {
            tbody.innerHTML = `<tr><td colspan="5" class="empty-state">No pending KYC requests</td></tr>`;
            return;
        }

        tbody.innerHTML = data.map(req => `
            <tr>
                <td>#${req.id}</td>
                <td>User ${req.user_id}</td>
                <td><strong>${req.document_id}</strong></td>
                <td>${new Date(req.created_at).toLocaleString()}</td>
                <td class="action-cell">
                    <button class="btn-primary" onclick="approveKyc(${req.id})">Approve</button>
                    <button class="btn-danger" style="margin-left: 8px;" onclick="rejectKyc(${req.id})">Reject</button>
                </td>
            </tr>
        `).join('');
    } catch (err) {
        tbody.innerHTML = `<tr><td colspan="5" class="error-msg">Error loading KYC requests</td></tr>`;
    }
}

async function approveKyc(id) {
    if (!confirm(`Are you sure you want to approve KYC Request #${id}? This will upgrade the user's tier.`)) return;
    try {
        const res = await fetch(`${API}/admin/kyc-requests/${id}/approve`, { method: 'POST', headers: getHeaders() });
        if (res.ok) loadKycQueue();
        else alert('Failed to approve KYC');
    } catch (err) { alert(err.message); }
}

async function rejectKyc(id) {
    if (!confirm(`Reject KYC Request #${id}?`)) return;
    try {
        const res = await fetch(`${API}/admin/kyc-requests/${id}/reject`, { method: 'POST', headers: getHeaders() });
        if (res.ok) loadKycQueue();
        else alert('Failed to reject KYC');
    } catch (err) { alert(err.message); }
}

async function loadWithdrawalQueue() {
    const tbody = document.getElementById('withdrawalQueueBody');
    try {
        const res = await fetch(`${API}/admin/withdrawals`, { headers: getHeaders() });
        const data = await res.json();
        
        if (!res.ok) throw new Error(data.error);
        if (!data || data.length === 0) {
            tbody.innerHTML = `<tr><td colspan="5" class="empty-state">No pending withdrawals</td></tr>`;
            return;
        }

        tbody.innerHTML = data.map(req => `
            <tr>
                <td>#${req.id}</td>
                <td>User ${req.user_id}</td>
                <td style="color:var(--loss-color); font-weight:bold;">${req.amount} PTS</td>
                <td>${new Date(req.created_at).toLocaleString()}</td>
                <td class="action-cell">
                    <button class="btn-primary" onclick="approveWithdrawal(${req.id})">Approve & Send</button>
                    <button class="btn-danger" style="margin-left: 8px;" onclick="rejectWithdrawal(${req.id})">Reject & Refund</button>
                </td>
            </tr>
        `).join('');
    } catch (err) {
        tbody.innerHTML = `<tr><td colspan="5" class="error-msg">Error loading withdrawals</td></tr>`;
    }
}

async function approveWithdrawal(id) {
    if (!confirm(`Mark Withdrawal #${id} as paid? Ensure you have actually sent the funds to the user.`)) return;
    try {
        const res = await fetch(`${API}/admin/withdrawals/${id}/approve`, { method: 'POST', headers: getHeaders() });
        if (res.ok) loadWithdrawalQueue();
        else alert('Failed to approve withdrawal');
    } catch (err) { alert(err.message); }
}

async function rejectWithdrawal(id) {
    if (!confirm(`Reject Withdrawal #${id}? The points will be refunded to the user's wallet.`)) return;
    try {
        const res = await fetch(`${API}/admin/withdrawals/${id}/reject`, { method: 'POST', headers: getHeaders() });
        if (res.ok) loadWithdrawalQueue();
        else alert('Failed to reject withdrawal');
    } catch (err) { alert(err.message); }
}
