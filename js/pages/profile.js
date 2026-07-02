import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';

/**
 * PROPHIT - Profile & Portfolio Logic
 */

document.addEventListener('DOMContentLoaded', () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    loadProfileData();
    loadPortfolio();
});

async function loadProfileData() {
    try {
        const user = await ApiClient.get('/me');
        document.getElementById('profileUsername').textContent = `@${user.username}`;
        document.getElementById('profileAvatar').src = `https://ui-avatars.com/api/?name=${user.username}&background=D4AF37&color=0A1128&size=120`;
        
        const refInput = document.getElementById('myReferralCode');
        if (refInput) {
            refInput.value = user.referral_code || 'N/A';
        }

        // Fetch streak
        const streak = await ApiClient.get('/me/streak');
        document.getElementById('statStreak').textContent = streak.current_streak || 0;

        // Fetch KYC for badge
        const kyc = await ApiClient.get('/kyc/status');
        const badge = document.getElementById('profileKycBadge');
        if (kyc.status === 'Verified') {
            badge.innerHTML = '<i class="fa-solid fa-shield-halved"></i> KYC Verified';
            badge.className = 'badge badge-success';
        } else {
            badge.innerHTML = '<i class="fa-solid fa-shield-halved"></i> Unverified';
            badge.className = 'badge badge-outline';
        }

    } catch (err) {
        console.error(err);
    }
}

async function loadPortfolio() {
    const list = document.getElementById('portfolioList');
    try {
        const data = await ApiClient.get('/me/portfolio');
        
        if (!data || data.length === 0) {
            list.innerHTML = `<div class="text-muted text-center" style="padding: var(--spacing-6);">You haven't made any predictions yet.</div>`;
            document.getElementById('statWinRate').textContent = '0%';
            document.getElementById('statMarketsJoined').textContent = '0';
            return;
        }

        document.getElementById('statMarketsJoined').textContent = data.length;

        // Calculate mock win rate based on resolved markets
        const resolved = data.filter(p => p.Market && p.Market.status === 'Resolved');
        const won = resolved.filter(p => p.prediction === p.Market.outcome);
        
        if (resolved.length > 0) {
            const wr = Math.round((won.length / resolved.length) * 100);
            document.getElementById('statWinRate').textContent = `${wr}%`;
        } else {
            document.getElementById('statWinRate').textContent = '--%';
        }

        list.innerHTML = data.map(p => {
            const m = p.Market || {};
            let status = '';
            if (m.status === 'Resolved') {
                const isWin = p.prediction === m.outcome;
                status = isWin ? `<span class="text-success font-bold">WON</span>` : `<span class="text-danger font-bold">LOST</span>`;
            } else {
                status = `<span class="text-warning">Active</span>`;
            }

            return `
                <div class="card" style="margin-bottom: var(--spacing-4); border-color: var(--border-subtle); padding: var(--spacing-4);">
                    <div class="flex justify-between items-start">
                        <div>
                            <div class="text-muted" style="font-size: 0.8rem; margin-bottom: 2px;">Market</div>
                            <a href="market.html?id=${m.id}" class="font-semibold text-primary" style="font-size: 1.1rem;">${m.title || 'Unknown Market'}</a>
                        </div>
                        <div style="text-align: right;">
                            ${status}
                        </div>
                    </div>
                    <div class="flex gap-6 mt-4" style="margin-top: var(--spacing-4); font-size: 0.9rem;">
                        <div>
                            <span class="text-muted">Prediction:</span> 
                            <span class="font-bold ${p.prediction === 'Yes' ? 'text-yes' : 'text-no'}">${p.prediction}</span>
                        </div>
                        <div>
                            <span class="text-muted">Stake:</span> 
                            <span class="font-bold">${p.amount} PTS</span>
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

window.copyReferral = () => {
    const el = document.getElementById('myReferralCode');
    if (el && el.value) {
        navigator.clipboard.writeText(el.value);
        showToast("Referral code copied to clipboard!", "success");
    }
};
