import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';

/**
 * PROPHIT - Authentication Logic
 */

let pendingGoogleCredential = '';

function requestTwoFactor(forGoogle = false) {
    const group = document.getElementById('twoFactorGroup');
    const input = document.getElementById('twoFactorCode');
    if (group) group.classList.remove('hidden');
    if (input) {
        input.required = true;
        input.focus();
    }
    document.getElementById('verifyGoogleTwoFactor')?.classList.toggle('hidden', !forGoogle);
    showToast('Enter the current 6-digit code from your authenticator app.', 'error');
}

async function completeGoogleLogin(credential) {
    try {
        const res = await ApiClient.post('/auth/google', { credential, two_factor_code: document.getElementById('twoFactorCode')?.value || '' });
        if (res && res.token) {
            ApiClient.setToken(res.token);
            pendingGoogleCredential = '';
            showToast('Google Sign-In successful! Welcome to PROPHIT.', 'success');
            setTimeout(() => window.location.href = 'dashboard.html', 1000);
        }
    } catch (err) {
        if (err.message === '2fa_required') {
            pendingGoogleCredential = credential;
            requestTwoFactor(true);
            return;
        }
        showToast(err.message || 'Google Sign-In failed', 'error');
    }
}

// Global Google OAuth callback handler
window.handleGoogleCredentialResponse = async (response) => {
    if (!response || !response.credential) {
        showToast('Google Sign-In failed', 'error');
        return;
    }
    await completeGoogleLogin(response.credential);
};

document.addEventListener('DOMContentLoaded', () => {
    const googleHost = document.getElementById('googleSignIn');
    if (googleHost) {
        ApiClient.get('/auth/config').then(config => {
            if (!config.google_client_id) { googleHost.textContent = 'Google Sign-In is not configured. Use email to continue.'; return; }
            const script = document.createElement('script'); script.src = 'https://accounts.google.com/gsi/client'; script.async = true;
            script.onload = () => { googleHost.replaceChildren(); window.google.accounts.id.initialize({client_id: config.google_client_id, callback: window.handleGoogleCredentialResponse}); window.google.accounts.id.renderButton(googleHost, {type:'standard', theme:'outline', size:'large'}); };
            script.onerror = () => { googleHost.textContent = 'Google Sign-In could not load. Use email or retry.'; };
            document.head.append(script);
        }).catch(() => { googleHost.textContent = 'Could not check Google Sign-In. Try email or refresh.'; });
    }
    // Password toggle visibility
    const toggles = document.querySelectorAll('.password-toggle');
    toggles.forEach(toggle => {
        toggle.addEventListener('click', (e) => {
            const input = e.currentTarget.previousElementSibling;
            if (input.type === 'password') {
                input.type = 'text';
                e.currentTarget.classList.replace('ph-eye', 'ph-eye-slash');
            } else {
                input.type = 'password';
                e.currentTarget.classList.replace('ph-eye-slash', 'ph-eye');
            }
        });
    });

    // Password strength meter
    const passInput = document.getElementById('regPassword');
    if (passInput) {
        passInput.addEventListener('input', (e) => {
            const val = e.target.value;
            const bars = document.querySelectorAll('.strength-bar');
            let strength = 0;
            if (val.length >= 8) strength++;
            if (/[A-Z]/.test(val)) strength++;
            if (/[0-9]/.test(val)) strength++;
            if (/[^A-Za-z0-9]/.test(val)) strength++;

            bars.forEach((bar, index) => {
                bar.style.backgroundColor = 'var(--border-strong)';
                if (index < strength) {
                    if (strength <= 1) bar.style.backgroundColor = 'var(--color-danger)';
                    else if (strength === 2) bar.style.backgroundColor = 'var(--color-warning)';
                    else bar.style.backgroundColor = 'var(--color-success)';
                }
            });
        });
    }

    // Handle Forms
    const loginForm = document.getElementById('loginForm');
    if (loginForm) {
        document.getElementById('verifyGoogleTwoFactor')?.addEventListener('click', async () => {
            if (pendingGoogleCredential) await completeGoogleLogin(pendingGoogleCredential);
        });
        loginForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const btn = loginForm.querySelector('button[type="submit"]');
            btn.disabled = true;
            btn.innerHTML = '<i class="ph ph-spinner ph-spin"></i> Logging in...';

            const email = document.getElementById('email').value;
            const password = document.getElementById('password').value;

            try {
                const res = await ApiClient.post('/auth/login', { email, password, two_factor_code: document.getElementById('twoFactorCode')?.value || '' });
                ApiClient.setToken(res.token);
                window.location.href = 'dashboard.html';
            } catch (err) {
                if (err.message === '2fa_required') requestTwoFactor(false);
                else showToast(err.message, 'error');
                btn.disabled = false;
                btn.innerHTML = 'Log in';
            }
        });
    }

    const regForm = document.getElementById('registerForm');
    if (regForm) {
        regForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const btn = regForm.querySelector('button[type="submit"]');
            btn.disabled = true;
            btn.innerHTML = '<i class="ph ph-spinner ph-spin"></i> Creating account...';

            const username = document.getElementById('username').value;
            const email = document.getElementById('email').value;
            const password = document.getElementById('regPassword').value;
            const passwordConfirm = document.getElementById('regPasswordConfirm').value;
            const referralCode = document.getElementById('referralCode')?.value || '';
            if (password !== passwordConfirm) {
                showToast('Passwords do not match.', 'error');
                btn.disabled = false;
                btn.innerHTML = 'Create Account';
                return;
            }

            // PDF §2.1: Optional demographic fields
            const phone = document.getElementById('phone')?.value || '';
            const date_of_birth = document.getElementById('dob')?.value || '';
            const city = document.getElementById('city')?.value || '';
            const country = document.getElementById('country')?.value || '';
            const interests = document.getElementById('interests')?.value || '';

            try {
                const res = await ApiClient.post('/auth/register', { 
                    username, 
                    email, 
                    password,
                    referral_code: referralCode,
                    phone,
                    date_of_birth,
                    city,
                    country,
                    interests,
                });
                if (res && res.token) {
                    ApiClient.setToken(res.token);
                    showToast('Account created successfully! Welcome to PROPHIT.', 'success');
                    setTimeout(() => window.location.href = 'dashboard.html', 1000);
                } else {
                    showToast('Registration successful! Please log in.', 'success');
                    setTimeout(() => window.location.href = 'login.html', 1500);
                }
            } catch (err) {
                showToast(err.message, 'error');
                btn.disabled = false;
                btn.innerHTML = 'Create Account';
            }
        });
    }

    const forgotForm = document.getElementById('forgotForm');
    if (forgotForm) {
        forgotForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const btn = forgotForm.querySelector('button[type="submit"]');
            btn.disabled = true;
            btn.innerHTML = '<i class="ph ph-spinner ph-spin"></i> Sending link...';

            const email = document.getElementById('email').value;

            try {
                await ApiClient.post('/auth/forgot-password', { email });
                showToast('If the email exists, a reset link has been sent.', 'success');
                setTimeout(() => window.location.href = 'login.html', 2000);
            } catch (err) {
                showToast(err.message, 'error');
                btn.disabled = false;
                btn.innerHTML = 'Send Reset Link';
            }
        });
    }
});
