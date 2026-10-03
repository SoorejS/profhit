import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';
import { escapeHTML, safeURL } from '../utils/escape.js';
import { initLiveFeed } from '../components/live-feed.js';

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
        document.getElementById('categoryTitle').textContent = category.charAt(0).toUpperCase() + category.slice(1) + ' Markets';
    }
    
    if (view === 'markets') {
        const grid = document.querySelector('.dashboard-grid');
        const rightSidebar = document.querySelector('.right-sidebar');
        if (grid) grid.style.gridTemplateColumns = '1fr';
        if (rightSidebar) rightSidebar.style.display = 'none';
        document.getElementById('categoryTitle').textContent = 'All Markets';
    }

    document.querySelectorAll('[data-sort]').forEach(button => button.addEventListener('click', () => { document.querySelectorAll('[data-sort]').forEach(b => b.setAttribute('aria-pressed', String(b === button))); fetchMarkets(category, button.dataset.sort); }));
    if (view === 'markets') { document.getElementById('liveFeed').hidden = true; fetchMarkets(category); }
    else { document.getElementById('legacyFeedControls').style.display = 'none'; document.getElementById('marketsContainer').hidden = true; initLiveFeed(document.getElementById('liveFeed')); }
    fetchStreak();
    fetchNews();
    fetchTrendingMarkets();
});

async function fetchTrendingMarkets() {
    const container = document.getElementById('trendingContainer');
    if (!container) return;
    try {
        const data = await ApiClient.get('/live-feed?section=trending&limit=3');
        container.replaceChildren();
        if (!data.items.length) container.textContent = 'No current playable predictions yet.';
        for (const market of data.items) {
            const link = document.createElement('a');
            link.href = `market.html?id=${Number(market.id)}`;
            link.textContent = `${market.title} · ${Number(market.volume)} players`;
            const row = document.createElement('p'); row.append(link); container.append(row);
        }
    } catch (error) { container.textContent = 'Current activity unavailable.'; }
}

async function fetchNews() {
    const container = document.getElementById('newsContainer');
    if (!container) return;

    try {
        const articles = await ApiClient.get('/news');
        if (!articles || articles.length === 0) {
            container.innerHTML = '<p class="text-sm text-secondary">No news available.</p>';
            return;
        }

        container.innerHTML = '';
        // Only show top 3 to fit the sidebar nicely
        articles.slice(0, 3).forEach(article => {
            const el = document.createElement('a');
            el.href = safeURL(article.url);
 if (!el.href) return;
 el.rel = 'noopener noreferrer';
            el.target = '_blank';
            el.className = 'flex items-center gap-3 p-2 rounded hover-bg transition-colors';
            el.style.textDecoration = 'none';
            el.style.color = 'inherit';

            const imgSrc = safeURL(article.image || '');
            
            el.innerHTML = `
                <img src="${escapeHTML(imgSrc)}" alt="News" style="width: 50px; height: 50px; object-fit: cover; border-radius: 4px;">
                <div class="flex-1 min-w-0">
                    <div class="text-sm font-semibold truncate" style="color: var(--text-primary); margin-bottom: 2px;">${escapeHTML(article.title)}</div>
                    <div class="text-xs truncate" style="color: var(--text-secondary);">${escapeHTML(article.description || '')}</div>
                </div>
            `;
            container.appendChild(el);
        });

    } catch (err) {
        container.innerHTML = '<p class="text-sm text-error">Could not load news.</p>';
    }
}

async function fetchStreak() {
    try {
        const streakData = await ApiClient.get('/me/streak');
        const el = document.getElementById('streakCount');
        if (el) el.textContent = streakData.current_streak || 0;
    } catch (err) {
        const el = document.getElementById('streakCount');
        if (el) el.textContent = 'Unavailable';
    }
}

window.claimDailyReward = async () => {
    const btn = document.getElementById('claimDailyBtn');
    if (!btn) return;

    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-circle-notch fa-spin"></i> Claiming...';

    try {
        const reward = await ApiClient.post('/me/daily-login');
        showToast(reward.message, 'success');
        fetchStreak();
        // Update topbar balance
        document.querySelector('app-topbar').fetchBalance();
        document.querySelector('app-sidebar').fetchBalance();
        btn.innerHTML = 'Claimed <i class="fa-solid fa-check"></i>';
    } catch (err) {
        showToast(err.message, "error");
        btn.disabled = false;
        btn.innerHTML = 'Claim Today\'s Bonus';
    }
};

async function fetchMarkets(category, sort = "trending") {
    const container = document.getElementById('marketsContainer');
    const trendingContainer = document.getElementById('trendingContainer');
    
    try {
        const markets = await ApiClient.get('/markets?' + new URLSearchParams({sort, search:new URLSearchParams(location.search).get('search') || '', ...(category ? {category} : {})}));
        
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

        // Render Trending Sidebar (Top 5 by volume)
        if (trendingContainer) {
            const trending = [...markets].sort((a, b) => (b.volume || 0) - (a.volume || 0)).slice(0, 5);
            trendingContainer.innerHTML = trending.map(m => `
                <div class="trending-item">
                    <a href="market.html?id=${m.id}" class="trending-item-title">${escapeHTML(m.title)}</a>
                    <div class="trending-item-vol"><i class="fa-solid fa-coins"></i> ${(m.volume || 0).toLocaleString()} Vol.</div>
                </div>
            `).join('');
        }

    } catch (err) {
        container.innerHTML = `<div class="card text-danger" style="grid-column: 1 / -1;">Failed to load markets. ${escapeHTML(err.message)}</div>`;
        if(trendingContainer) trendingContainer.innerHTML = `<div class="text-danger">Failed to load trending</div>`;
    }
}

function renderMarketCard(m) {
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
        <a class="card market-card" href="market.html?id=${m.id}" style="text-decoration: none; color: inherit;">
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
                    <span><i class="fa-solid fa-users"></i> ${m.volume || 0} Predictions</span>
                    <span><i class="fa-solid fa-clock"></i> ${m.lock_time ? new Date(m.lock_time).toLocaleDateString() : (m.end_date ? new Date(m.end_date).toLocaleDateString() : 'TBD')}</span>
                </div>
                
                <div class="text-gold">Fixed payout: ${Number(m.payout)} PTS</div>

            </div>
        </a>
    `;
}

let liveRefresh;
window.addEventListener('prophit-live',e=>{if(['prediction_count_changed','market_activity_changed','market_locked','market_resolved','market_live'].includes(e.detail.event)){clearTimeout(liveRefresh);liveRefresh=setTimeout(()=>{fetchMarkets(new URLSearchParams(location.search).get('category'),document.querySelector('[data-sort][aria-pressed="true"]')?.dataset.sort||'trending');fetchStreak();},150);}});
