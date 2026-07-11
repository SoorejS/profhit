import '../components/sidebar.js';
import '../components/topbar.js';
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
        console.error(err);
    }
}

async function loadLedger() {
    const list = document.getElementById('ledgerList');
    try {
        const data = await ApiClient.get('/wallet/history');
        if (!data || data.length === 0) {
            list.innerHTML = `<div class="text-muted text-center" style="padding: var(--spacing-6);">No transactions yet.</div>`;
            return;
        }

        list.innerHTML = data.map(tx => {
            const change = tx.credit > 0 ? tx.credit : -tx.debit;
            const isPositive = change > 0;
            const sign = isPositive ? '+' : '';
            const color = isPositive ? 'var(--color-success)' : 'var(--color-danger)';
            return `
                <div class="transaction-item">
                    <div>
                        <div class="font-semibold">${tx.description || tx.type || 'Transaction'}</div>
                        <div class="text-muted" style="font-size: 0.8rem;">${new Date(tx.created_at).toLocaleString()}</div>
                    </div>
                    <div style="color: ${color}; font-weight: 700;">
                        ${sign}${Math.abs(change)} PTS
                    </div>
                </div>
            `;
        }).join('');
    } catch (err) {
        console.error(err);
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
        } else if (data.status === 'Pending') {
            icon.innerHTML = '<i class="fa-solid fa-clock text-warning"></i>';
            text.textContent = 'Verification Pending';
            text.style.color = 'var(--color-warning)';
            btn.textContent = 'Check Status';
        } else if (data.status === 'Rejected') {
            icon.innerHTML = '<i class="fa-solid fa-circle-xmark text-danger"></i>';
            text.textContent = 'Verification Failed';
            text.style.color = 'var(--color-danger)';
            btn.textContent = 'Retry KYC';
        } else {
            // Unverified / No record
            icon.innerHTML = '<i class="fa-solid fa-shield-halved text-muted"></i>';
            text.textContent = 'Unverified';
        }
    } catch (err) {
        console.error(err);
    }
}

async function startKyc() {
    try {
        const btn = document.getElementById('kycBtn');
        btn.disabled = true;
        btn.innerHTML = '<i class="fa-solid fa-circle-notch fa-spin"></i> Initializing...';

        const res = await ApiClient.post('/kyc/start');
        
        if (res.verification_url) {
            window.location.href = res.verification_url;
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
    document.getElementById('depositModal').classList.remove('hidden');
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
                        razorpay_signature: response.razorpay_signature,
                        points: parseFloat(amt) // Added required points argument for Verification
                    });
                    showToast("Deposit successful!", "success");
                    document.getElementById('depositModal').classList.add('hidden');
                    loadWalletData();
                    loadLedger();
                } catch (err) {
                    showToast("Payment verification failed", "error");
                }
            },
            theme: { color: "#8b5cf6" }
        };
        const rzp = new window.Razorpay(options);
        rzp.open();
        
    } catch (err) {
        showToast(err.message, 'error');
    }
}

// Withdraw Flow
async function openWithdraw(amount, itemName) {
    if (!amount) {
        amount = prompt("Enter amount of PTS to withdraw (1 PTS = 1 INR):");
        itemName = "Cash Withdrawal";
        if (!amount) return;
    }
    
    if (parseFloat(amount) < 100) {
        showToast('Minimum withdrawal is 100 PTS.', 'error');
        return;
    }

    if (!confirm(`Redeem ${amount} PTS for ${itemName}?`)) return;

    try {
        await ApiClient.post('/payments/redeem', {
            reward_id: 1, // Defaulting to generic cash withdrawal for now
            amount: parseFloat(amount)
        });
        showToast(`Redemption request for ${itemName} submitted! Wait for admin approval.`, "success");
        loadWalletData();
        loadLedger();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

window.openWithdraw = openWithdraw;
window.openDeposit = openDeposit;
window.processDeposit = processDeposit;
window.startKyc = startKyc;
