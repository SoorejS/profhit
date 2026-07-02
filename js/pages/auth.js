import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';

/**
 * PROPHIT - Authentication Logic
 */

document.addEventListener('DOMContentLoaded', () => {
    // Password toggle visibility
    const toggles = document.querySelectorAll('.password-toggle');
    toggles.forEach(toggle => {
        toggle.addEventListener('click', (e) => {
            const input = e.target.previousElementSibling;
            if (input.type === 'password') {
                input.type = 'text';
                e.target.classList.replace('fa-eye', 'fa-eye-slash');
            } else {
                input.type = 'password';
                e.target.classList.replace('fa-eye-slash', 'fa-eye');
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
        loginForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const btn = loginForm.querySelector('button[type="submit"]');
            btn.disabled = true;
            btn.innerHTML = '<i class="fa-solid fa-circle-notch fa-spin"></i> Logging in...';

            const email = document.getElementById('email').value;
            const password = document.getElementById('password').value;

            try {
                const res = await ApiClient.post('/auth/login', { email, password });
                ApiClient.setToken(res.token);
                window.location.href = 'dashboard.html';
            } catch (err) {
                showToast(err.message, 'error');
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
            btn.innerHTML = '<i class="fa-solid fa-circle-notch fa-spin"></i> Creating account...';

            const username = document.getElementById('username').value;
            const email = document.getElementById('email').value;
            const password = document.getElementById('regPassword').value;
            const referralCode = document.getElementById('referralCode')?.value || '';

            try {
                await ApiClient.post('/auth/register', { 
                    username, 
                    email, 
                    password,
                    referral_code: referralCode
                });
                showToast('Registration successful! Please log in.', 'success');
                setTimeout(() => window.location.href = 'login.html', 1500);
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
            btn.innerHTML = '<i class="fa-solid fa-circle-notch fa-spin"></i> Sending link...';

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
