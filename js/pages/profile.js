import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';
import { escapeHTML } from '../utils/escape.js';

/**
 * PROPHIT - Profile & Portfolio Logic
 */

document.addEventListener('DOMContentLoaded', () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    loadProfileData();
});

async function loadProfileData() {
    let user;
    try {
        user = await ApiClient.get('/me');
        document.getElementById('profileJoined').textContent = 'Joined ' + new Date(user.created_at).toLocaleDateString();
        document.getElementById('profileTier').textContent = user.tier + ' Tier';
        document.getElementById('profileUsername').textContent = `@${user.username}`;
        document.getElementById('profileAvatar').src = 'data:image/svg+xml,'+encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="120" height="120"><rect width="120" height="120" rx="60" fill="#D4AF37"/><text x="60" y="78" text-anchor="middle" font-size="60" fill="#0A1128">${escapeHTML(user.username.slice(0,1).toUpperCase())}</text></svg>`);
        document.getElementById('profileTier').textContent += user.badges?.length ? ` · ${user.badges.join(', ')}` : '';
        
        const refInput = document.getElementById('myReferralCode');
        if (refInput) {
            refInput.value = user.referral_code || 'N/A';
        }
    } catch (err) {
        document.getElementById('profileUsername').textContent = 'Profile unavailable';
        document.getElementById('profileJoined').textContent = 'Could not load account details.';
        document.getElementById('profileTier').textContent = 'Unavailable';
        document.getElementById('profileKycBadge').textContent = 'Status unavailable';
        document.getElementById('myReferralCode').value = 'Unavailable';
        document.getElementById('earnedAchievements').textContent = 'Could not load achievements.';
        document.getElementById('statMarketsJoined').textContent = '--';
        document.getElementById('statWinRate').textContent = '--';
        document.getElementById('statStreak').textContent = '--';
        showToast('Could not load profile.', 'error');
        return;
    }

    const [earnedResult, portfolioResult, statsResult, streakResult, kycResult] = await Promise.allSettled([
        ApiClient.get('/me/achievements'), ApiClient.get('/portfolio'), ApiClient.get('/me/stats'), ApiClient.get('/me/streak'), ApiClient.get('/kyc/status')
    ]);

    if (earnedResult.status === 'fulfilled') {
        const earnedVal = earnedResult.value;
        const earned = Array.isArray(earnedVal) ? earnedVal : (earnedVal?.items || []);
        document.getElementById('earnedAchievements').innerHTML = earned.length ? earned.map(a => `<article><strong>${escapeHTML(a.title)}</strong><p>${escapeHTML(a.description)}</p></article>`).join('') : '<p>No achievements earned yet.</p>';
    } else {
        document.getElementById('earnedAchievements').textContent = 'Could not load achievements.';
    }

    if (statsResult.status === 'fulfilled' && statsResult.value) {
        const stats = statsResult.value;
        document.getElementById('statMarketsJoined').textContent = stats.total_predictions || '0';
        document.getElementById('statWinRate').textContent = stats.win_rate || '0%';
    } else if (portfolioResult.status === 'fulfilled') {
        const portfolioVal = portfolioResult.value;
        const portfolio = Array.isArray(portfolioVal) ? portfolioVal : (portfolioVal?.items || []);
        const resolved = portfolio.filter(p => p.is_correct !== null);
        document.getElementById('statMarketsJoined').textContent = portfolio.length;
        document.getElementById('statWinRate').textContent = resolved.length ? `${Math.round(100 * resolved.filter(p => p.is_correct).length / resolved.length)}%` : '--%';
    } else {
        document.getElementById('statMarketsJoined').textContent = '--';
        document.getElementById('statWinRate').textContent = '--';
    }

    document.getElementById('statStreak').textContent = streakResult.status === 'fulfilled' ? (streakResult.value.current_streak || 0) : '--';

    const badge = document.getElementById('profileKycBadge');
    if (kycResult.status === 'fulfilled') {
        const kyc = kycResult.value;
        if (kyc.status === 'Verified') {
            badge.innerHTML = '<i class="ph-bold ph-shield-check"></i> KYC Verified';
            badge.className = 'badge badge-success';
        } else {
            badge.innerHTML = '<i class="ph-bold ph-shield-check"></i> Unverified';
            badge.className = 'badge badge-outline';
        }
    } else {
        badge.textContent = 'Status unavailable';
        badge.className = 'badge badge-warning';
    }
}

window.copyReferral = async () => {
    const el = document.getElementById('myReferralCode');
    if (el && el.value) {
        try { await navigator.clipboard.writeText(el.value); showToast("Referral code copied to clipboard!", "success"); } catch { showToast("Could not copy. Select and copy the code manually.", "error"); }
    }
};

ApiClient.get('/referrals/analytics').then(data=>{const target=document.getElementById('referralStatus');target.innerHTML=`<p>${Number(data.total_referred)} invited · ${Number(data.total_earnings)} coins paid</p>`+(data.history?.length?data.history.map(r=>`<p>Referral #${Number(r.id)} · ${Number(r.earnings)} coins · ${r.is_paid?'Paid':'Pending until '+new Date(r.pending_until).toLocaleString()}</p>`).join(''):'<p>No referral rewards pending.</p>');}).catch(()=>{document.getElementById('referralStatus').textContent='Referral status unavailable.';});
