import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';
import { escapeHTML } from '../utils/escape.js';

/**
 * PROPHIT - Market Detail Logic
 */

let currentMarketId = null;
let currentSelection = null;
let currentPayout = 0;

document.addEventListener('DOMContentLoaded', () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    const urlParams = new URLSearchParams(window.location.search);
    currentMarketId = urlParams.get('id');

    if (!currentMarketId) {
        window.location.href = 'dashboard.html';
        return;
    }

    loadMarketDetails();
    loadComments();


    // Register amount input listener once (prevents stacking on repeated Yes/No clicks)
    const amountInput = document.getElementById('tradeAmount');
    if (amountInput) {
        amountInput.addEventListener('input', () => {
            const amt = parseFloat(amountInput.value);
            if (amt > 0) {
                document.getElementById('potentialReturn').textContent = `+${currentPayout} PTS`;
            } else {
                document.getElementById('potentialReturn').textContent = '--';
            }
        });
    }
});

async function loadMarketDetails() {
    try {
        const market = await ApiClient.get(`/markets/${encodeURIComponent(currentMarketId)}`);
        currentPayout = Number(market.payout);
        document.getElementById('marketFixedPayout').textContent = `${currentPayout} PTS`;
        document.getElementById('marketPredictionCount').textContent = String(Number(market.volume) || 0);
        const options = JSON.parse(market.options);
        const optionsContainer = document.querySelector('.btn-yes')?.parentElement;
        if (optionsContainer) { optionsContainer.replaceChildren(); options.forEach(option => { const button = document.createElement('button'); button.className = 'btn btn-outline'; button.textContent = option; button.dataset.option = option; button.addEventListener('click', () => selectPrediction(option)); optionsContainer.append(button); }); }
        document.getElementById('potentialReturn').textContent = `${currentPayout} PTS`;
        document.getElementById('chartContainer').textContent = 'Historical probability data is not available for fixed-payout markets.';
        
        document.getElementById('marketTitle').textContent = market.title;
        document.getElementById('marketDesc').textContent = market.description;
        document.getElementById('resolutionRules').textContent = market.resolution_source
            ? `An authorized reviewer will resolve this market using the configured official source: ${market.resolution_source}`
            : 'An authorized reviewer will resolve this market from documented, verifiable evidence.';
        document.getElementById('marketCategory').textContent = market.category;
        
        document.getElementById('marketCloseDate').textContent = market.lock_time 
            ? new Date(market.lock_time).toLocaleDateString() 
            : (market.end_date ? new Date(market.end_date).toLocaleDateString() : 'TBD');

        const isClosed = new Date(market.lock_time || market.end_date) <= new Date();
        const notStarted = market.start_time && new Date(market.start_time) > new Date();
        const statusEl = document.getElementById('marketStatus');
        
        if (['Resolved', 'Archived'].includes(market.resolution_status) && market.correct_option) {
            statusEl.textContent = `Resolved: ${market.correct_option}`;
            statusEl.className = 'badge badge-success';
            document.querySelector('.trade-card').innerHTML = `<h3 class="text-success text-center">Market Resolved: ${escapeHTML(market.correct_option)}</h3>`;
        } else if (market.resolution_status === 'Scheduled' || notStarted) {
            statusEl.textContent = 'Scheduled';
            document.querySelector('.trade-card').textContent = 'Predictions open when this market starts.';
        } else if (!['Open', 'Live'].includes(market.resolution_status) || isClosed) {
            statusEl.textContent = 'Resolving';
            statusEl.className = 'badge badge-warning';
            document.querySelector('.trade-card').innerHTML = `<h3 class="text-warning text-center">Market is closed. Awaiting resolution.</h3>`;
        } else {
            statusEl.textContent = market.resolution_status || 'Active';
            statusEl.className = 'badge badge-primary';
        }

    } catch (err) {
        document.getElementById('marketCategory').textContent = 'Unavailable';
        document.getElementById('marketStatus').textContent = 'Error';
        document.getElementById('marketTitle').textContent = 'Market unavailable';
        document.getElementById('marketDesc').textContent = 'Could not load market details. Refresh to retry.';
        document.getElementById('chartContainer').textContent = 'Market history is unavailable.';
        document.getElementById('resolutionRules').textContent = 'Resolution criteria are unavailable.';
        document.querySelector('.trade-card').textContent = 'Predictions are unavailable while market details cannot be loaded.';
        document.getElementById('marketPredictionCount').textContent = 'Unavailable';
        showToast('Failed to load market details.', 'error');
    }
}

function selectPrediction(outcome) {
    currentSelection = outcome;
    document.getElementById('tradeForm').classList.remove('hidden');
    
    // Update button styles
    document.querySelectorAll('[data-option]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.option === outcome)));
}

async function executeTrade() {
    const amount = document.getElementById('tradeAmount').value;
    if (!Number.isInteger(Number(amount)) || Number(amount) < 10) {
        showToast('Minimum trade amount is 10 PTS.', 'error');
        return;
    }

    if (!currentSelection) return;

    try {
        await ApiClient.post('/predictions', {
            market_id: parseInt(currentMarketId),
            choice: currentSelection,
            amount: parseInt(amount, 10)
        });
        
        showToast(`Successfully predicted ${currentSelection} with ${amount} PTS!`, 'success');
        
        // Reset form
        document.getElementById('tradeForm').classList.add('hidden');
        document.getElementById('tradeAmount').value = '';
        currentSelection = null;
        document.querySelectorAll('[data-option]').forEach(button => button.setAttribute('aria-pressed', 'false'));
        
        // Trigger topbar to fetch new balance
        document.querySelector('app-topbar').fetchBalance();
        document.querySelector('app-sidebar').fetchBalance();

    } catch (err) {
        showToast(err.message, 'error');
    }
}

async function loadComments() {
    try {
        const res = await ApiClient.get(`/markets/${encodeURIComponent(currentMarketId)}/comments`);
        const comments = Array.isArray(res) ? res : (res?.items || []);
        const list = document.getElementById('commentsList');
        
        if (!comments || comments.length === 0) {
            list.innerHTML = `<div class="text-muted text-center" style="padding: var(--spacing-4);">No comments yet. Be the first to share your thoughts!</div>`;
            return;
        }

        list.innerHTML = comments.map(c => `
            <div class="comment-item">
                <div class="flex justify-between items-center" style="margin-bottom: 4px;">
                    <div class="font-bold text-primary">@${escapeHTML(c.username || 'Unknown')}</div>
                    <div class="text-muted" style="font-size: 0.8rem;">${new Date(c.created_at).toLocaleString()}</div>
                </div>
                <div class="text-secondary" style="font-size: 0.95rem;">${escapeHTML(c.content)}</div>
            </div>
        `).join('');

    } catch (err) {
        document.getElementById('commentsList').textContent = 'Comments could not load. Refresh to retry.';
    }
}

async function postComment() {
    const input = document.getElementById('commentInput');
    const content = input.value.trim();
    if (!content) return;

    try {
        await ApiClient.post(`/markets/${encodeURIComponent(currentMarketId)}/comments`, { content });
        input.value = '';
        showToast('Comment posted.', 'success');
        loadComments();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

window.selectPrediction = selectPrediction;
window.executeTrade = executeTrade;
window.postComment = postComment;
