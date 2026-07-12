import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';
import { escapeHTML } from '../utils/escape.js';

/**
 * PROPHIT - Dashboard Logic
 */

document.addEventListener('DOMContentLoaded', () => {
    // Check auth
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    const urlParams = new URLSearchParams(window.location.search);
    const category = urlParams.get('category');
    const view = urlParams.get('view');
    
    if (category) {
        document.getElementById('categoryTitle').textContent = escapeHTML(category.charAt(0).toUpperCase() + category.slice(1) + ' Markets');
    }
    
    if (view === 'markets') {
        const grid = document.querySelector('.dashboard-grid');
        const rightSidebar = document.querySelector('.right-sidebar');
        if (grid) grid.style.gridTemplateColumns = '1fr';
        if (rightSidebar) rightSidebar.style.display = 'none';
        document.getElementById('categoryTitle').textContent = 'All Markets';
    }

    fetchMarkets(category);
    fetchStreak();
});

async function fetchStreak() {
    try {
        const streakData = await ApiClient.get('/me/streak');
        const el = document.getElementById('streakCount');
        if (el) el.textContent = streakData.current_streak || 0;
    } catch (err) {
        console.error(err);
    }
}

window.claimDailyReward = async () => {
    const btn = document.getElementById('claimDailyBtn');
    if (!btn) return;

    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-circle-notch fa-spin"></i> Claiming...';

    try {
        await ApiClient.post('/me/daily-login');
        showToast("Daily reward claimed successfully! +50 PTS", "success");
        fetchStreak();
        // Update topbar balance
        document.querySelector('app-topbar').fetchBalance();
        btn.innerHTML = 'Claimed <i class="fa-solid fa-check"></i>';
    } catch (err) {
        showToast(err.message, "error");
        btn.disabled = false;
        btn.innerHTML = 'Claim Today\'s Bonus';
    }
};

async function fetchMarkets(category) {
    const container = document.getElementById('marketsContainer');
    const trendingContainer = document.getElementById('trendingContainer');
    
    try {
        const markets = await ApiClient.get('/markets');
        
        let filtered = markets;
        if (category) {
            filtered = markets.filter(m => m.category && m.category.toLowerCase() === category.toLowerCase());
        }

        if (filtered.length === 0) {
            container.innerHTML = `
                <div class="card" style="grid-column: 1 / -1; text-align: center; padding: var(--spacing-12);">
                    <i class="fa-solid fa-ghost" style="font-size: 3rem; color: var(--border-strong); margin-bottom: var(--spacing-4);"></i>
                    <h3 style="color: var(--text-secondary);">No markets found in this category.</h3>
                </div>
            `;
        } else {
            container.innerHTML = filtered.map(renderMarketCard).join('');
        }

        // Render Trending Sidebar (Top 3 by volume or ID for now)
        if (trendingContainer) {
            const trending = [...markets].sort((a, b) => (b.id) - (a.id)).slice(0, 5); // Mock logic for trending
            trendingContainer.innerHTML = trending.map(m => `
                <div class="trending-item">
                    <a href="market.html?id=${m.id}" class="trending-item-title">${escapeHTML(m.title)}</a>
                    <div class="trending-item-vol"><i class="fa-solid fa-coins"></i> ${Math.floor(Math.random() * 50000) + 1000} Vol.</div>
                </div>
            `).join('');
        }

    } catch (err) {
        console.error(err);
        container.innerHTML = `<div class="card text-danger" style="grid-column: 1 / -1;">Failed to load markets. ${escapeHTML(err.message)}</div>`;
        if(trendingContainer) trendingContainer.innerHTML = `<div class="text-danger">Failed to load trending</div>`;
    }
}

function renderMarketCard(m) {
    // Use market ID as seed for deterministic probability display
    const seed = m.id % 60;
    const yesProb = 20 + seed;
    const noProb = 100 - yesProb;

    // Check if the market is closed
    const isClosed = m.lock_time ? new Date(m.lock_time) < new Date() : false;
    
    // Status Badge
    let statusBadge = '';
    if (m.resolution_status === 'Resolved') {
        statusBadge = `<span class="badge badge-success">Resolved: ${escapeHTML(m.correct_option)}</span>`;
    } else if (m.resolution_status === 'Locked' || m.resolution_status === 'Awaiting Resolution') {
        statusBadge = `<span class="badge badge-warning">Resolving</span>`;
    } else {
        statusBadge = `<span class="badge badge-primary">${escapeHTML(m.resolution_status || 'Live')}</span>`;
    }

    return `
        <div class="card market-card" onclick="window.location.href='market.html?id=${m.id}'" style="cursor: pointer;">
            <div>
                <div class="market-card-header">
                    <span class="market-card-category"><i class="fa-solid fa-tag"></i> ${escapeHTML(m.category)}</span>
                    ${statusBadge}
                </div>
                <h3 class="market-card-title">${escapeHTML(m.title)}</h3>
                <p style="font-size: 0.85rem; color: var(--text-muted); display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;">
                    ${escapeHTML(m.description)}
                </p>
            </div>
            
            <div style="margin-top: var(--spacing-4);">
                <div class="market-card-stats">
                    <span><i class="fa-solid fa-users"></i> ${Math.floor(Math.random() * 500) + 10} Traders</span>
                    <span><i class="fa-solid fa-clock"></i> ${new Date(m.close_time).toLocaleDateString()}</span>
                </div>
                
                <div class="flex justify-between" style="font-size: 0.85rem; font-weight: 600; margin-bottom: var(--spacing-1);">
                    <span class="text-yes">Yes ${yesProb}%</span>
                    <span class="text-no">No ${noProb}%</span>
                </div>
                <div class="prob-bar-container">
                    <div class="prob-bar-yes" style="width: ${yesProb}%;"></div>
                    <div class="prob-bar-no" style="width: ${noProb}%;"></div>
                </div>
            </div>
        </div>
    `;
}
