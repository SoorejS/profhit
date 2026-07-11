import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';

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
                    <div class="font-bold" style="font-size: 1.1rem; margin-bottom: 4px;">${item.name}</div>
                    <div class="text-gold font-bold" style="margin-bottom: var(--spacing-4); font-size: 1.1rem;">${item.cost} PTS</div>
                    <button class="btn btn-outline w-full" ${outOfStock ? 'disabled' : ''} onclick="redeemReward(${item.id}, '${item.name}', ${item.cost})">${outOfStock ? 'Out of Stock' : 'Redeem'}</button>
                </div>
            `;
        }).join('');

    } catch (err) {
        console.error(err);
        list.innerHTML = `<div class="text-danger text-center" style="padding: var(--spacing-6); grid-column: 1 / -1;">Failed to load reward catalog.</div>`;
    }
}

async function loadHistory() {
    const list = document.getElementById('historyList');
    try {
        const data = await ApiClient.get('/me/redemptions');
        
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
                        <span style="color: ${statusColor}; font-weight: 600; font-size: 0.85rem;">${item.status}</span>
                    </div>
                    <div class="flex justify-between items-center text-sm text-muted">
                        <span>Cost: ${item.cost_paid} PTS</span>
                        <span>${new Date(item.created_at).toLocaleDateString()}</span>
                    </div>
                    ${item.voucher_code ? `<div class="mt-2 text-sm">Voucher: <span class="font-mono font-bold text-success">${item.voucher_code}</span></div>` : ''}
                </div>
            `;
        }).join('');

    } catch (err) {
        console.error(err);
        list.innerHTML = `<div class="text-danger text-center" style="padding: var(--spacing-6);">Failed to load history.</div>`;
    }
}

window.redeemReward = async (id, name, cost) => {
    if (!confirm(`Are you sure you want to redeem ${cost} PTS for "${name}"?`)) return;

    try {
        await ApiClient.post('/rewards/redeem', { reward_item_id: id });
        showToast(`Successfully submitted redemption request for ${name}!`, 'success');
        loadCatalog();
        loadHistory();
    } catch (err) {
        showToast(err.message, 'error');
    }
};
