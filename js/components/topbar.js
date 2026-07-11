import ApiClient from '../api/client.js';

export function logout() {
    ApiClient.removeToken();
    window.location.href = '/login.html';
}

export class AppTopbar extends HTMLElement {
    connectedCallback() {
        this.innerHTML = `
            <header class="topbar flex justify-between items-center" style="padding: var(--spacing-4) var(--spacing-6); border-bottom: 1px solid var(--border-subtle); background: var(--bg-base);">
                <div class="search-container" style="flex: 1; max-width: 400px; position: relative;">
                    <i class="ph ph-magnifying-glass" style="position: absolute; left: 15px; top: 50%; transform: translateY(-50%); color: var(--text-muted);"></i>
                    <input type="text" class="input-control" placeholder="Search markets, categories..." style="padding-left: 40px; border-radius: var(--radius-full); background: var(--bg-surface);">
                </div>
                
                <div class="topbar-actions flex items-center gap-4">
                    <button class="btn btn-outline" onclick="window.location.href='/wallet.html'" style="border-radius: var(--radius-full);">
                        <i class="ph-fill ph-coins text-gold"></i> <span id="topbarBalance">--</span> PTS
                    </button>
                    
                    <div class="profile-menu" style="position: relative; cursor: pointer;">
                        <img src="https://ui-avatars.com/api/?name=User&background=D4AF37&color=0A1128" alt="Profile" style="width: 40px; height: 40px; border-radius: 50%; border: 2px solid var(--border-subtle);" onclick="document.getElementById('profileDropdown').classList.toggle('hidden')">
                        
                        <div id="profileDropdown" class="hidden" style="position: absolute; top: 50px; right: 0; background: var(--bg-surface-elevated); border: 1px solid var(--border-strong); border-radius: var(--radius-md); width: 200px; box-shadow: var(--shadow-lg); z-index: 100;">
                            <a href="/profile.html" class="nav-link" style="padding: 10px 15px; display: block; color: var(--text-primary);"><i class="ph ph-user"></i> Profile</a>
                            <a href="/wallet.html" class="nav-link" style="padding: 10px 15px; display: block; color: var(--text-primary);"><i class="ph ph-wallet"></i> Wallet & KYC</a>
                            <div style="height: 1px; background: var(--border-strong); margin: 5px 0;"></div>
                            <a href="javascript:void(0)" id="logoutBtn" class="nav-link text-danger" style="padding: 10px 15px; display: block;"><i class="ph ph-sign-out"></i> Logout</a>
                        </div>
                    </div>
                </div>
            </header>
        `;
        
        document.addEventListener('click', (e) => {
            if (!this.contains(e.target)) {
                const drop = document.getElementById('profileDropdown');
                if(drop) drop.classList.add('hidden');
            }
        });
        
        const logoutBtn = this.querySelector('#logoutBtn');
        if(logoutBtn) {
            logoutBtn.addEventListener('click', (e) => {
                e.preventDefault();
                logout();
            });
        }
        
        this.fetchBalance();
    }
    
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
