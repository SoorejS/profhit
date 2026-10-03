import '../services/live.js';
import ApiClient from '../api/client.js';

export async function logout() {
    try {
        await ApiClient.post('/auth/logout');
    } catch {
        // The local credential must still be removed when the API is down.
        // Server-side revocation is best-effort until connectivity returns.
    } finally {
        ApiClient.removeToken();
        window.location.href = '/login.html';
    }
}

export class AppTopbar extends HTMLElement {
    connectedCallback() {
        this.innerHTML = `
            <header class="topbar flex justify-between items-center" style="padding: var(--spacing-4) var(--spacing-6); border-bottom: 1px solid var(--border-subtle); background: var(--bg-base);">
                <button class="btn btn-outline menu-toggle" type="button" aria-label="Toggle navigation" aria-expanded="false">Menu</button>
                <div class="search-container" style="flex: 1; max-width: 400px; position: relative;">
                    <i class="ph ph-magnifying-glass" style="position: absolute; left: 15px; top: 50%; transform: translateY(-50%); color: var(--text-muted);"></i>
                    <input type="text" class="input-control" aria-label="Search markets" placeholder="Search markets..." style="padding-left: 40px; border-radius: var(--radius-full); background: var(--bg-surface);">
                </div>
                
                <div class="topbar-actions flex items-center gap-4">
                    <button class="btn btn-outline" onclick="window.location.href='/wallet.html'" style="border-radius: var(--radius-full);">
                        <i class="ph-fill ph-coins text-gold"></i> <span id="topbarBalance">--</span> PTS
                    </button>
                    
                    <div class="profile-menu" style="position: relative; cursor: pointer;">
                        <button type="button" class="btn btn-outline" id="profileToggle" aria-label="Open profile menu" aria-expanded="false"><i class="ph ph-user"></i></button>
                        
                        <div id="profileDropdown" class="hidden" style="position: absolute; top: 50px; right: 0; background: var(--bg-surface-elevated); border: 1px solid var(--border-strong); border-radius: var(--radius-md); width: 200px; box-shadow: var(--shadow-lg); z-index: 100;">
                            <a href="/profile.html" class="nav-link" style="padding: 10px 15px; display: block; color: var(--text-primary);"><i class="ph ph-user"></i> Profile</a>
                            <a href="/wallet.html" class="nav-link" style="padding: 10px 15px; display: block; color: var(--text-primary);"><i class="ph ph-wallet"></i> Wallet & KYC</a>
                            <div style="height: 1px; background: var(--border-strong); margin: 5px 0;"></div>
                            <button type="button" id="logoutBtn" class="nav-link text-danger" style="padding: 10px 15px; display: block;"><i class="ph ph-sign-out"></i> Logout</button>
                        </div>
                    </div>
                </div>
            </header>
        `;
        
        this.querySelector('#profileToggle').addEventListener('click', e => { const hidden = this.querySelector('#profileDropdown').classList.toggle('hidden'); e.currentTarget.setAttribute('aria-expanded', String(!hidden)); });
        this.onOutsideClick = (e) => {
            const sidebar = document.querySelector('app-sidebar');
            if (!sidebar?.contains(e.target) && !this.querySelector('.menu-toggle').contains(e.target)) {
                this.closeNavigation();
            }
            if (!this.contains(e.target)) {
                const drop = document.getElementById('profileDropdown');
                if(drop) drop.classList.add('hidden');
                this.querySelector('#profileToggle').setAttribute('aria-expanded', 'false');
            }
        };
        document.addEventListener('click', this.onOutsideClick);
        this.onEscape = (e) => {
            if (e.key === 'Escape' && document.querySelector('app-sidebar')?.classList.contains('open')) {
                this.closeNavigation();
                this.querySelector('.menu-toggle').focus();
            }
        };
        document.addEventListener('keydown', this.onEscape);
        this.querySelector('.menu-toggle').addEventListener('click', e => { const sidebar = document.querySelector('app-sidebar'); const open = sidebar.classList.toggle('open'); e.currentTarget.setAttribute('aria-expanded', String(open)); });
        this.querySelector('input').addEventListener('keydown', e => { if (e.key === 'Enter') window.location.href = '/dashboard.html?search=' + encodeURIComponent(e.target.value); });
        
        const logoutBtn = this.querySelector('#logoutBtn');
        if(logoutBtn) {
            logoutBtn.addEventListener('click', (e) => {
                e.preventDefault();
                logout();
            });
        }
        
        this.onLive=e=>{if(e.detail.event==='wallet_updated')this.fetchBalance();};window.addEventListener('prophit-live',this.onLive);
        this.fetchBalance();
		if(ApiClient.isAuthenticated())ApiClient.post('/me/daily-login').then(()=>this.fetchBalance()).catch(()=>{});
    }
    
    closeNavigation() {
        document.querySelector('app-sidebar')?.classList.remove('open');
        this.querySelector('.menu-toggle')?.setAttribute('aria-expanded', 'false');
    }

    disconnectedCallback() { window.removeEventListener('prophit-live',this.onLive); document.removeEventListener('click', this.onOutsideClick); document.removeEventListener('keydown', this.onEscape); }
    async fetchBalance() {
        if (!ApiClient.isAuthenticated()) return;
        try {
            const data = await ApiClient.get('/me');
            const el = document.getElementById('topbarBalance');
            if (el) el.textContent = data.points;
            
            if(data.role === 'admin' || data.role === 'super_admin') {
                const drop = document.getElementById('profileDropdown');
                if (drop && !document.getElementById('adminLinkDrop')) {
                    const adminL = document.createElement('a');
                    adminL.id = 'adminLinkDrop';
                    adminL.href = '/admin.html';
                    adminL.className = 'nav-link text-primary';
                    adminL.style.cssText = 'padding: 10px 15px; display: block; font-weight: bold;';
                    adminL.innerHTML = '<i class="ph-bold ph-lock-key"></i> Admin Panel';
                    drop.insertBefore(adminL, drop.firstChild);
                }
            }
        } catch(e) {}
    }
}
customElements.define('app-topbar', AppTopbar);
