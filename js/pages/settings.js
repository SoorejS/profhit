import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';

function renderTwoFactor(enabled, pending = false) {
    const status = document.getElementById('twoFactorStatus');
    status.textContent = enabled ? 'Enabled' : (pending ? 'Setup pending' : 'Disabled');
    status.className = `badge ${enabled ? 'badge-success' : (pending ? 'badge-warning' : 'badge-outline')}`;
    document.getElementById('twoFactorOff').classList.toggle('hidden', enabled);
    document.getElementById('twoFactorOn').classList.toggle('hidden', !enabled);
    if (!pending) document.getElementById('twoFactorSetup').classList.add('hidden');
}

async function loadTwoFactor() {
    const state = await ApiClient.get('/me/2fa');
    renderTwoFactor(state.enabled, state.setup_pending);
}

document.addEventListener('DOMContentLoaded', async () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    try {
        const data = await ApiClient.get('/me');
        if (data) {
            document.getElementById('sUsername').value = data.username || '';
            document.getElementById('sEmail').value = data.email || '';
            document.getElementById('sTier').value = data.tier || 'Bronze';
            for(const [id,key] of [['sFullName','full_name'],['sPhone','phone'],['sCity','city'],['sCountry','country'],['sInterests','interests']]) document.getElementById(id).value=data[key]||'';
        }
        await loadTwoFactor();
    } catch (err) {
        document.getElementById('sUsername').value = 'Error loading data';
        document.getElementById('sEmail').value = 'Error loading data';
        document.getElementById('sTier').value = 'Error loading data';
        document.getElementById('twoFactorStatus').textContent = 'Unavailable';
    }

    document.getElementById('profileForm').addEventListener('submit', async e=>{e.preventDefault();const button=e.target.querySelector('button');button.disabled=true;try{const data={};for(const [id,key] of [['sFullName','full_name'],['sPhone','phone'],['sCity','city'],['sCountry','country'],['sInterests','interests']])data[key]=document.getElementById(id).value.trim();const result=await ApiClient.put('/me/profile',data);showToast(result.message,'success');document.querySelector('app-topbar')?.fetchBalance();document.querySelector('app-sidebar')?.fetchBalance();}catch(err){showToast(err.message,'error');}finally{button.disabled=false;}});

    document.getElementById('startTwoFactor').addEventListener('click', async () => {
        const password = document.getElementById('twoFactorPassword').value;
        if (!password) { showToast('Enter your current password.', 'error'); return; }
        try {
            const setup = await ApiClient.post('/me/2fa/setup', { current_password: password });
            document.getElementById('twoFactorQr').src = setup.qr_code_data_url;
            document.getElementById('twoFactorSecret').textContent = setup.secret;
            document.getElementById('twoFactorSetup').classList.remove('hidden');
            renderTwoFactor(false, true);
            showToast('Scan the QR code, then verify a current code.', 'success');
        } catch (error) { showToast(error.message, 'error'); }
    });

    document.getElementById('enableTwoFactor').addEventListener('click', async () => {
        const code = document.getElementById('twoFactorEnableCode').value;
        try {
            const result = await ApiClient.post('/me/2fa/enable', { code });
            ApiClient.setToken(result.token);
            renderTwoFactor(true);
            document.getElementById('twoFactorPassword').value = '';
            document.getElementById('twoFactorEnableCode').value = '';
            showToast('Authenticator 2FA is enabled.', 'success');
        } catch (error) { showToast(error.message, 'error'); }
    });

    document.getElementById('disableTwoFactor').addEventListener('click', async () => {
        const code = document.getElementById('twoFactorDisableCode').value;
        try {
            const result = await ApiClient.post('/me/2fa/disable', { code });
            ApiClient.setToken(result.token);
            renderTwoFactor(false);
            document.getElementById('twoFactorDisableCode').value = '';
            showToast('Authenticator 2FA is disabled.', 'success');
        } catch (error) { showToast(error.message, 'error'); }
    });
});
