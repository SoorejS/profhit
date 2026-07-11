import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { escapeHTML } from '../utils/escape.js';

document.addEventListener('DOMContentLoaded', () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    loadPortfolio();
});

async function loadPortfolio() {
    const list = document.getElementById('portfolioList');
    try {
        const data = await ApiClient.get('/portfolio');
        
        if (!data || data.length === 0) {
            list.innerHTML = `<div class="text-muted text-center" style="padding: var(--spacing-6);">You haven't made any predictions yet.</div>`;
            document.getElementById('statWinRate').textContent = '0%';
            document.getElementById('statMarketsJoined').textContent = '0';
            document.getElementById('statTotalPayout').textContent = '0 PTS';
            return;
        }

        document.getElementById('statMarketsJoined').textContent = data.length;

        // Calculate win rate based on resolved markets
        const resolved = data.filter(p => p.market_status === 'Resolved');
        const won = resolved.filter(p => p.is_correct === true);
        
        if (resolved.length > 0) {
            const wr = Math.round((won.length / resolved.length) * 100);
            document.getElementById('statWinRate').textContent = `${wr}%`;
        } else {
            document.getElementById('statWinRate').textContent = '--%';
        }
        
        // Calculate potential payout
        let totalPayout = 0;
        data.forEach(p => {
            if (p.market_status !== 'Resolved') {
                totalPayout += p.potential_payout;
            }
        });
        document.getElementById('statTotalPayout').textContent = `${totalPayout} PTS`;

        list.innerHTML = data.map(p => {
            let status = '';
            if (p.market_status === 'Resolved') {
                const isWin = p.is_correct === true;
                status = isWin 
                    ? `<span class="text-success font-bold"><i class="ph-fill ph-check-circle"></i> WON</span>` 
                    : `<span class="text-danger font-bold"><i class="ph-fill ph-x-circle"></i> LOST</span>`;
            } else {
                status = `<span class="text-warning"><i class="ph-fill ph-clock"></i> Active</span>`;
            }

            return `
                <div class="card" style="margin-bottom: var(--spacing-4); border-color: var(--border-subtle); padding: var(--spacing-4);">
                    <div class="flex justify-between items-start">
                        <div>
                            <div class="text-muted" style="font-size: 0.8rem; margin-bottom: 2px;">Market</div>
                            <a href="market.html?id=${p.market_id}" class="font-semibold text-primary" style="font-size: 1.1rem;">${escapeHTML(p.market || 'Unknown Market')}</a>
                        </div>
                        <div style="text-align: right;">
                            ${status}
                        </div>
                    </div>
                    <div class="flex gap-6 mt-4" style="margin-top: var(--spacing-4); font-size: 0.9rem;">
                        <div>
                            <span class="text-muted">Prediction:</span> 
                            <span class="font-bold ${p.choice === 'Yes' ? 'text-yes' : 'text-no'}">${escapeHTML(p.choice)}</span>
                        </div>
                        <div>
                            <span class="text-muted">Potential:</span> 
                            <span class="font-bold">${p.potential_payout} PTS</span>
                        </div>
                    </div>
                </div>
            `;
        }).join('');

    } catch (err) {
        console.error(err);
        list.innerHTML = `<div class="text-danger text-center" style="padding: var(--spacing-6);">Failed to load portfolio.</div>`;
    }
}
