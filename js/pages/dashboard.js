import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';
import { escapeHTML, safeURL } from '../utils/escape.js';
import { initLiveFeed } from '../components/live-feed.js';
let dailyRewardRevision = 0;

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

    document.getElementById('legacyFeedControls').hidden = true;
    document.getElementById('marketsContainer').hidden = true;
    initLiveFeed(document.getElementById('liveFeed'));
    fetchStreak();
    fetchDailyRewardStatus();
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
            link.textContent = `${market.title} · ${Number(market.volume) ? `${Number(market.volume)} players` : 'Be the first to predict'}`;
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

function renderDailyReward(reward) {
    const btn = document.getElementById('claimDailyBtn');
    const status = document.getElementById('dailyRewardStatus');
    if (!btn) return;
    btn.disabled = reward.already_checked_in || reward.coins_earned > 0;
    btn.textContent = btn.disabled ? '10 coins credited today ✓' : 'Claim daily login: 10 coins';
    if (status) status.textContent = btn.disabled
        ? `Awarded once per day. Next reward: ${new Date(reward.next_claim_at).toLocaleString(undefined, {month:'short',day:'numeric',hour:'numeric',minute:'2-digit',timeZoneName:'short'})}.`
        : 'Your daily login reward is ready to claim.';
}

async function fetchDailyRewardStatus() {
    const revision = dailyRewardRevision;
    try {
        const reward = await ApiClient.get('/me/daily-login');
        if (revision === dailyRewardRevision) renderDailyReward(reward);
    } catch {
        if (revision !== dailyRewardRevision) return;
        document.getElementById('claimDailyBtn').disabled = false;
        document.getElementById('claimDailyBtn').textContent = 'Retry daily reward';
        document.getElementById('dailyRewardStatus').textContent = 'Could not confirm your reward status. Retry to check safely.';
    }
}

window.addEventListener('prophit-daily-reward', e => {
    dailyRewardRevision++;
    renderDailyReward(e.detail);
});

window.claimDailyReward = async () => {
    const btn = document.getElementById('claimDailyBtn');
    if (!btn || btn.disabled) return;

    btn.disabled = true;
    btn.textContent = 'Claiming…';

    try {
        const reward = await ApiClient.post('/me/daily-login');
        dailyRewardRevision++;
        showToast(reward.message, 'success');
        fetchStreak();
        document.querySelector('app-topbar')?.setBalance(reward.new_balance);
        document.querySelector('app-sidebar')?.setBalance(reward.new_balance);
        renderDailyReward(reward);
    } catch (err) {
        showToast(err.message, "error");
        btn.disabled = false;
        btn.textContent = 'Retry daily reward';
        document.getElementById('dailyRewardStatus').textContent = 'Reward could not be confirmed. Retry safely; you can only be credited once per day.';
    }
};



window.addEventListener('prophit-live', e => { if (['prediction_count_changed','market_activity_changed','market_locked','market_resolved','market_live','market_state_changed'].includes(e.detail.event)) { fetchTrendingMarkets(); fetchStreak(); } });
