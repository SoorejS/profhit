import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { escapeHTML } from '../utils/escape.js';
let page=1;

document.addEventListener('DOMContentLoaded', () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    document.getElementById('portfolioPrev').addEventListener('click',()=>{if(page>1){page--;loadPortfolio();}});
    document.getElementById('portfolioNext').addEventListener('click',()=>{page++;loadPortfolio();});
    loadPortfolio();
});

async function loadPortfolio() {
    const list = document.getElementById('portfolioList');
    list.textContent='Loading portfolio...';
    document.getElementById('portfolioPrev').disabled=true;
    document.getElementById('portfolioNext').disabled=true;
    try {
        const [res, statsRes] = await Promise.allSettled([
            ApiClient.get('/portfolio?page='+page),
            ApiClient.get('/me/stats')
        ]);

        if(res.status!=='fulfilled')throw res.reason;
        const rawData = res.value;
        const data = Array.isArray(rawData) ? rawData : (rawData?.items || []);
        document.getElementById('portfolioPrev').disabled=page===1;
        document.getElementById('portfolioNext').disabled=page>=(rawData.total_pages||1);
        document.getElementById('portfolioPage').textContent=`Page ${page} of ${Math.max(1,rawData.total_pages||1)}`;

        if (statsRes.status === 'fulfilled' && statsRes.value) {
            const stats = statsRes.value;
            document.getElementById('statMarketsJoined').textContent = stats.total_predictions || '0';
            document.getElementById('statWinRate').textContent = stats.win_rate || '0%';
        } else {
            document.getElementById('statMarketsJoined').textContent = rawData.total ?? data.length;
            document.getElementById('statWinRate').textContent = 'Unavailable';
        }

        if (!data || data.length === 0) {
            list.innerHTML = `<div class="text-muted text-center" style="padding: var(--spacing-6);">You haven't made any predictions yet.</div>`;
            document.getElementById('statTotalPayout').textContent = '0 PTS';
            return;
        }

        let totalPayout = 0;
        data.forEach(p => {
            if (p.is_correct === null) {
                totalPayout += (p.potential_payout || 0);
            }
        });
        document.getElementById('statTotalPayout').textContent = statsRes.status==='fulfilled' ? `${statsRes.value.pending_potential} PTS` : 'Unavailable';

        list.innerHTML = data.map(p => {
            let status = '';
            if (p.is_correct !== null) {
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
        for(const id of ['statWinRate','statMarketsJoined','statTotalPayout'])document.getElementById(id).textContent='Unavailable';
        list.innerHTML = `<div class="text-danger text-center" style="padding: var(--spacing-6);">Failed to load portfolio.</div>`;
    }
}
