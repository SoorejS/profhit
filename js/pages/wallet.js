import '../components/sidebar.js';
import '../components/topbar.js';
import { escapeHTML, safeURL } from '../utils/escape.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';

/**
 * PROPHIT - Wallet & Identity Logic
 */

document.addEventListener('DOMContentLoaded', () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    loadWalletData();
    loadLedger();
    checkKycStatus();
});

async function loadWalletData() {
    try {
        const data = await ApiClient.get('/me');
        document.getElementById('walletBalance').textContent = data.points;
    } catch (err) {
        document.getElementById('walletBalance').textContent = 'Unavailable';
    }
}

async function loadLedger() {
    const list = document.getElementById('ledgerList');
    try {
        const res = await ApiClient.get('/wallet/history');
        const data = Array.isArray(res) ? res : (res.items || []);
        if (!data || data.length === 0) {
            list.innerHTML = `<div class="text-muted text-center" style="padding: var(--spacing-6);">No transactions yet.</div>`;
            return;
        }

        list.innerHTML = data.map(tx => {
            const change = tx.credit > 0 ? tx.credit : -tx.debit;
            const isPositive = change > 0;
            const sign = isPositive ? '+' : '-';
            const color = isPositive ? 'var(--color-success)' : 'var(--color-danger)';
            return `
                <div class="transaction-item">
                    <div>
                        <div class="font-semibold">${escapeHTML(tx.description || tx.type || 'Transaction')}</div>
                        <div class="text-muted" style="font-size: 0.8rem;">${new Date(tx.created_at).toLocaleString()}</div>
                    </div>
                    <div style="color: ${color}; font-weight: 700;">
                        ${sign}${Math.abs(change)} PTS
                    </div>
                </div>
            `;
        }).join('');
    } catch (err) {
        list.innerHTML = `<div class="text-danger text-center" style="padding: var(--spacing-6);">Failed to load ledger.</div>`;
    }
}

async function checkKycStatus() {
    const icon = document.getElementById('kycIcon');
    const text = document.getElementById('kycStatusText');
    const btn = document.getElementById('kycBtn');

    try {
        const data = await ApiClient.get('/kyc/status');
        
        if (data.status === 'Verified') {
            icon.innerHTML = '<i class="fa-solid fa-circle-check text-success"></i>';
            text.textContent = 'Verified Identity';
            text.style.color = 'var(--color-success)';
            btn.style.display = 'none';
        } else if (['Pending', 'Started', 'DocumentsUploaded'].includes(data.status)) {
            icon.innerHTML = '<i class="fa-solid fa-clock text-warning"></i>';
            text.textContent = 'Verification Pending';
            text.style.color = 'var(--color-warning)';
            btn.textContent = 'Check Status';
            btn.onclick = checkKycStatus;
        } else if (data.status === 'Rejected') {
            icon.innerHTML = '<i class="fa-solid fa-circle-xmark text-danger"></i>';
            text.textContent = 'Verification Failed';
            text.style.color = 'var(--color-danger)';
            btn.textContent = 'Retry KYC';
            btn.onclick = startKyc;
        } else {
            // Unverified / No record
            icon.innerHTML = '<i class="fa-solid fa-shield-halved text-muted"></i>';
            text.textContent = 'Unverified'; btn.onclick = startKyc;
        }
    } catch (err) {
        text.textContent = 'Could not load verification status'; btn.textContent = 'Retry status'; btn.onclick = checkKycStatus;
    }
}

async function startKyc() {
    try {
        const btn = document.getElementById('kycBtn');
        btn.disabled = true;
        btn.innerHTML = '<i class="fa-solid fa-circle-notch fa-spin"></i> Initializing...';

        const res = await ApiClient.post('/kyc/start');
        
        if (res.verification_url) {
            const redirect = safeURL(res.verification_url);
 if (!redirect || !redirect.startsWith("https://")) throw new Error("Invalid verification URL");
 window.location.href = redirect;
        } else {
            throw new Error('No verification URL returned');
        }
    } catch (err) {
        showToast(`Failed to start KYC: ${err.message}`, 'error');
        const btn = document.getElementById('kycBtn');
        btn.disabled = false;
        btn.textContent = 'Start HyperVerge KYC';
    }
}

// Deposit Flow
function openDeposit() {
    document.getElementById('depositModal').showModal();
}

async function processDeposit() {
    const amt = document.getElementById('depositAmount').value;
    if (!amt || amt < 10) {
        showToast('Minimum deposit is 10 INR', 'error');
        return;
    }

    try {
        const order = await ApiClient.post('/payments/order', { amount: parseFloat(amt) });
        
        const options = {
            key: order.key, 
            amount: order.amount,
            currency: order.currency,
            name: "PROPHIT",
            description: "Deposit to Wallet",
            order_id: order.order_id,
            handler: async function (response) {
                try {
                    await ApiClient.post('/payments/verify', {
                        razorpay_order_id: response.razorpay_order_id,
                        razorpay_payment_id: response.razorpay_payment_id,
                        razorpay_signature: response.razorpay_signature
                    });
                    showToast("Deposit successful!", "success");
                    document.getElementById('depositModal').close();
                    loadWalletData();
                    loadLedger();
                    document.querySelector("app-topbar").fetchBalance();
                    document.querySelector("app-sidebar").fetchBalance();
                } catch (err) {
                    showToast("Payment verification pending: " + err.message + ". Check wallet history before paying again.", "error");
                }
            },
            theme: { color: "#8b5cf6" }
        };
        if (!window.Razorpay) throw new Error("Payment checkout could not load. Please retry.");
        const rzp = new window.Razorpay(options);
        rzp.on("payment.failed", response => showToast(response.error?.description || "Payment failed", "error"));
        rzp.open();
        
    } catch (err) {
        showToast(err.message, 'error');
    }
}

// Withdraw Flow
async function openWithdraw() {
 const tiers = {Bronze: [500, 50], Silver: [1200, 150], Gold: [2500, 350], Platinum: [5000, 800], Diamond: [10000, 2000]};
 const entered = prompt('Choose a voucher tier: Bronze (500 PTS / INR 50), Silver (1200 / 150), Gold (2500 / 350), Platinum (5000 / 800), Diamond (10000 / 2000)');
 if (!entered) return;
 const tier = Object.keys(tiers).find(key => key.toLowerCase() === entered.trim().toLowerCase());
 if (!tier) { showToast('Choose one of the listed voucher tiers.', 'error'); return; }
 if (!confirm(`Redeem ${tiers[tier][0]} PTS for an INR ${tiers[tier][1]} ${tier} voucher?`)) return;
 try { const result = await ApiClient.post('/payments/redeem', {tier}); showToast(result.message, 'success'); loadWalletData(); loadLedger(); document.querySelector('app-topbar')?.fetchBalance(); document.querySelector('app-sidebar')?.fetchBalance(); }
 catch (err) { showToast(err.message, 'error'); }
}
window.loadLedger = loadLedger;
window.openWithdraw = openWithdraw;
window.openDeposit = openDeposit;
window.processDeposit = processDeposit;
window.startKyc = startKyc;
