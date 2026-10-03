import '../components/sidebar.js';
import '../components/topbar.js';
import { escapeHTML } from '../utils/escape.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';
import { confirmAction } from '../components/dialog.js';

document.addEventListener('DOMContentLoaded', () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    loadCatalog();
    loadHistory();
});

async function loadCatalog() {
    const list = document.getElementById('catalogList');
    try {
        const data = await ApiClient.get('/rewards');
        
        if (!data || data.length === 0) {
            list.innerHTML = `<div class="text-muted text-center" style="padding: var(--spacing-6); grid-column: 1 / -1;">No rewards available right now.</div>`;
            return;
        }

        list.innerHTML = data.map(item => {
            const outOfStock = item.inventory === 0;
            return `
                <div class="card market-card" style="text-align: center; border: 1px solid var(--border-subtle); background: var(--bg-surface-elevated); opacity: ${outOfStock ? 0.6 : 1};">
                    <i class="ph-fill ph-gift" style="font-size: 3rem; margin-bottom: 12px; color: ${outOfStock ? 'var(--text-muted)' : 'var(--color-gold)'};"></i>
                    <div class="font-bold" style="font-size: 1.1rem; margin-bottom: 4px;">${escapeHTML(item.name)}</div>
                    <div class="text-gold font-bold" style="margin-bottom: var(--spacing-4); font-size: 1.1rem;">${item.cost} PTS</div>
                    <button class="btn btn-outline w-full" ${outOfStock ? 'disabled' : ''} data-reward-id="${item.id}">${outOfStock ? 'Out of Stock' : 'Redeem'}</button>
                </div>
            `;
        }).join('');
        list.querySelectorAll('[data-reward-id]').forEach(button => button.addEventListener('click', () => { const item = data.find(item => item.id === Number(button.dataset.rewardId)); window.redeemReward(item.id, item.name, item.cost); }));

    } catch (err) {
        list.innerHTML = `<div class="text-danger text-center" style="padding: var(--spacing-6); grid-column: 1 / -1;">Failed to load reward catalog.</div>`;
    }
}

async function loadHistory() {
    const list = document.getElementById('historyList');
    try {
        const res = await ApiClient.get('/me/redemptions');
        const data = Array.isArray(res) ? res : (res?.items || []);
        
        if (!data || data.length === 0) {
            list.innerHTML = `<div class="text-muted text-center" style="padding: var(--spacing-6);">No redemption history.</div>`;
            return;
        }

        list.innerHTML = data.map(item => {
            let statusColor = 'var(--text-muted)';
            if (item.status === 'Completed') statusColor = 'var(--color-success)';
            if (item.status === 'Rejected') statusColor = 'var(--color-danger)';
            if (item.status === 'Pending') statusColor = 'var(--color-warning)';
            
            return `
                <div style="padding: var(--spacing-3) 0; border-bottom: 1px solid var(--border-subtle);">
                    <div class="flex justify-between items-center" style="margin-bottom: 4px;">
                        <span class="font-bold">Item #${item.reward_item_id}</span>
                        <span style="color: ${statusColor}; font-weight: 600; font-size: 0.85rem;">${escapeHTML(item.status)}</span>
                    </div>
                    <div class="flex justify-between items-center text-sm text-muted">
                        <span>Cost: ${item.cost_paid} PTS</span>
                        <span>${new Date(item.created_at).toLocaleDateString()}</span>
                    </div>
                    ${item.voucher_code ? `<div class="mt-2 text-sm">Voucher: <span class="font-mono font-bold text-success">${escapeHTML(item.voucher_code)}</span></div>` : ''}
                </div>
            `;
        }).join('');

    } catch (err) {
        list.innerHTML = `<div class="text-danger text-center" style="padding: var(--spacing-6);">Failed to load history.</div>`;
    }
}

window.redeemReward = async (id, name, cost) => {
    if (!await confirmAction(`Redeem ${cost} PTS for "${name}"?`)) return;

    try {
        await ApiClient.post('/rewards/redeem', { reward_item_id: id });
        showToast(`Successfully submitted redemption request for ${name}!`, 'success');
        loadCatalog();
        loadHistory();
    } catch (err) {
        showToast(err.message, 'error');
    }
};
