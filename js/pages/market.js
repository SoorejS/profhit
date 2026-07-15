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
    initChart();

    // Register amount input listener once (prevents stacking on repeated Yes/No clicks)
    const amountInput = document.getElementById('tradeAmount');
    if (amountInput) {
        amountInput.addEventListener('input', () => {
            const amt = parseFloat(amountInput.value);
            if (amt > 0) {
                document.getElementById('potentialReturn').textContent = `+${Math.floor(amt * 1.8)} PTS`;
            } else {
                document.getElementById('potentialReturn').textContent = '--';
            }
        });
    }
});

async function loadMarketDetails() {
    try {
        const market = await ApiClient.get(`/markets/${currentMarketId}`);
        
        document.getElementById('marketTitle').textContent = market.title;
        document.getElementById('marketDesc').textContent = market.description;
        document.getElementById('marketCategory').textContent = market.category;
        
        document.getElementById('marketCloseDate').textContent = market.lock_time 
            ? new Date(market.lock_time).toLocaleDateString() 
            : (market.end_date ? new Date(market.end_date).toLocaleDateString() : 'TBD');

        const isClosed = market.lock_time ? new Date(market.lock_time) < new Date() : false;
        const statusEl = document.getElementById('marketStatus');
        
        if (market.resolution_status === 'Resolved') {
            statusEl.textContent = `Resolved: ${market.correct_option}`;
            statusEl.className = 'badge badge-success';
            document.querySelector('.trade-card').innerHTML = `<h3 class="text-success text-center">Market Resolved: ${escapeHTML(market.correct_option)}</h3>`;
        } else if (market.resolution_status === 'Locked' || market.resolution_status === 'Awaiting Resolution') {
            statusEl.textContent = 'Resolving';
            statusEl.className = 'badge badge-warning';
            document.querySelector('.trade-card').innerHTML = `<h3 class="text-warning text-center">Market is closed. Awaiting resolution.</h3>`;
        } else {
            statusEl.textContent = market.resolution_status || 'Active';
            statusEl.className = 'badge badge-primary';
        }

    } catch (err) {
        console.error(err);
        showToast('Failed to load market details.', 'error');
    }
}

function initChart() {
    // Using Lightweight Charts for a modern trading feel
    const container = document.getElementById('chartContainer');
    const chart = LightweightCharts.createChart(container, {
        layout: {
            background: { type: 'solid', color: '#18181b' },
            textColor: '#a1a1aa',
        },
        grid: {
            vertLines: { color: '#27272a' },
            horzLines: { color: '#27272a' },
        },
        timeScale: {
            timeVisible: true,
            secondsVisible: false,
        }
    });

    const yesSeries = chart.addLineSeries({
        color: '#22c55e',
        lineWidth: 2,
    });
    
    const noSeries = chart.addLineSeries({
        color: '#ef4444',
        lineWidth: 2,
    });

    // Generate mock probability history
    const dataYes = [];
    const dataNo = [];
    let curYes = 50;
    const now = Math.floor(Date.now() / 1000);
    
    for (let i = 30; i >= 0; i--) {
        curYes += (Math.random() - 0.5) * 10;
        if (curYes > 95) curYes = 95;
        if (curYes < 5) curYes = 5;
        
        const time = now - (i * 86400); // Daily points
        dataYes.push({ time, value: curYes });
        dataNo.push({ time, value: 100 - curYes });
    }

    yesSeries.setData(dataYes);
    noSeries.setData(dataNo);
    chart.timeScale().fitContent();

    // Handle resize
    window.addEventListener('resize', () => {
        chart.applyOptions({ width: container.clientWidth });
    });
}

function selectPrediction(outcome) {
    currentSelection = outcome;
    document.getElementById('tradeForm').classList.remove('hidden');
    
    // Update button styles
    document.querySelector('.btn-yes').style.opacity = outcome === 'Yes' ? '1' : '0.5';
    document.querySelector('.btn-no').style.opacity = outcome === 'No' ? '1' : '0.5';
}

async function executeTrade() {
    const amount = document.getElementById('tradeAmount').value;
    if (!amount || amount < 10) {
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
        document.querySelector('.btn-yes').style.opacity = '1';
        document.querySelector('.btn-no').style.opacity = '1';
        
        // Trigger topbar to fetch new balance
        document.querySelector('app-topbar').fetchBalance();
        document.querySelector('app-sidebar').fetchBalance();

    } catch (err) {
        showToast(err.message, 'error');
    }
}

async function loadComments() {
    try {
        const comments = await ApiClient.get(`/markets/${currentMarketId}/comments`);
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
        console.error(err);
    }
}

async function postComment() {
    const input = document.getElementById('commentInput');
    const content = input.value.trim();
    if (!content) return;

    try {
        await ApiClient.post(`/markets/${currentMarketId}/comments`, { content });
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
