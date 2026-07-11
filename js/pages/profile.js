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

window.copyReferral = () => {
    const el = document.getElementById('myReferralCode');
    if (el && el.value) {
        navigator.clipboard.writeText(el.value);
        showToast("Referral code copied to clipboard!", "success");
    }
};
