import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { escapeHTML } from '../utils/escape.js';

let currentTab = 'points';
let currentPage = 1;
let currentSearch = '';
let debounceTimer;

document.addEventListener('DOMContentLoaded', () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    loadLeaderboard();

    document.getElementById('searchInput').addEventListener('input', (e) => {
        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(() => {
            currentSearch = e.target.value.trim();
            currentPage = 1;
            loadLeaderboard();
        }, 300);
    });
});

window.switchTab = (tab) => {
    if (currentTab === tab) return;
    
    document.querySelectorAll('.leaderboard-tab').forEach(el => el.classList.remove('active'));
    document.getElementById(`tab-${tab}`).classList.add('active');
    
    currentTab = tab;
    currentPage = 1;
    loadLeaderboard();
};

window.changePage = (dir) => {
    currentPage += dir;
    if (currentPage < 1) currentPage = 1;
    loadLeaderboard();
};

async function loadLeaderboard() {
    const listEl = document.getElementById('leaderboardList');
    const userCardEl = document.getElementById('currentUserCard');
    
    listEl.innerHTML = `
        <div class="card skeleton" style="height: 80px; border-radius: var(--radius-lg);"></div>
        <div class="card skeleton" style="height: 80px; border-radius: var(--radius-lg);"></div>
        <div class="card skeleton" style="height: 80px; border-radius: var(--radius-lg);"></div>
    `;
    
    try {
        const query = new URLSearchParams({
            sort: currentTab,
            page: currentPage,
            limit: 50
        });
        if (currentSearch) query.set('search', currentSearch);

        const response = await ApiClient.get(`/leaderboard?${query.toString()}`);
        
        if (!response.data || response.data.length === 0) {
            listEl.innerHTML = `
                <div class="text-center text-muted" style="padding: var(--spacing-8);">
                    <i class="ph ph-ghost" style="font-size: 3rem; margin-bottom: 1rem;"></i>
                    <p>No leaderboard data available.</p>
                </div>
            `;
        } else {
            listEl.innerHTML = response.data.map(user => renderLeaderboardRow(user, currentTab)).join('');
        }
        
        // Update pagination
        const meta = response.meta;
        const totalPages = Math.ceil((meta.total || 0) / meta.limit);
        document.getElementById('pageIndicator').textContent = `Page ${currentPage} of ${Math.max(1, totalPages)}`;
        document.getElementById('prevPageBtn').disabled = currentPage <= 1;
        document.getElementById('nextPageBtn').disabled = currentPage >= totalPages;

        // Render Current User
        if (response.current_user) {
            userCardEl.style.display = 'block';
            userCardEl.innerHTML = `
                <div class="text-secondary" style="font-size: 0.85rem; font-weight: 600; text-transform: uppercase; margin-bottom: 8px;">Your Rank</div>
                ${renderLeaderboardRow(response.current_user, currentTab, true)}
            `;
        } else {
            userCardEl.style.display = 'none';
        }
        
    } catch (err) {
        console.error(err);
        listEl.innerHTML = `
            <div class="text-center text-danger" style="padding: var(--spacing-8);">
                <i class="ph ph-warning-circle" style="font-size: 3rem; margin-bottom: 1rem;"></i>
                <p>Failed to load leaderboard: ${escapeHTML(err.message)}</p>
            </div>
        `;
    }
}

function renderLeaderboardRow(user, tab, isCurrentUser = false) {
    const rank = user.rank;
    let rankBadgeClass = '';
    if (rank === 1) rankBadgeClass = 'rank-1';
    else if (rank === 2) rankBadgeClass = 'rank-2';
    else if (rank === 3) rankBadgeClass = 'rank-3';
    
    let scoreValue = 0;
    let scoreLabel = '';
    
    if (tab === 'points') {
        scoreValue = `${(user.points || 0).toLocaleString()} <span style="font-size:0.8rem">PTS</span>`;
        scoreLabel = 'Total Points';
    } else if (tab === 'streak') {
        scoreValue = `${user.longest_streak || 0} <i class="ph-fill ph-fire text-gold" style="font-size: 1rem;"></i>`;
        scoreLabel = 'Day Streak';
    } else if (tab === 'winrate') {
        const rate = (user.win_rate || 0).toFixed(1);
        scoreValue = `${rate}%`;
        scoreLabel = 'Win Rate';
    }
    
    const username = user.username || 'You';
    const avatar = `https://ui-avatars.com/api/?name=${encodeURIComponent(username)}&background=1E1E1E&color=D4AF37&size=48`;
    const bgClass = isCurrentUser ? 'style="border-color: var(--color-primary); background: rgba(212,175,55,0.05);"' : '';
    
    return `
        <div class="leaderboard-item" ${bgClass}>
            <div class="rank-badge ${rankBadgeClass}">${rank || '-'}</div>
            <div class="user-info">
                <img src="${avatar}" alt="${escapeHTML(username)}" class="user-avatar">
                <div style="min-width: 0;">
                    <div class="user-name" style="white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">@${escapeHTML(username)}</div>
                    ${isCurrentUser ? '<div class="user-tier text-primary">Current User</div>' : '<div class="user-tier"><i class="ph-fill ph-medal text-gold"></i> Top Forecaster</div>'}
                </div>
            </div>
            <div class="score-box">
                <div class="score-value">${scoreValue}</div>
                <div class="score-label">${scoreLabel}</div>
            </div>
        </div>
    `;
}
