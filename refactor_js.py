import os
import shutil

# Move API client
with open('js/api.js', 'r', encoding='utf-8') as f:
    api_code = f.read()
api_code = api_code.replace("window.ApiClient = ApiClient;", "export default ApiClient;")
with open('js/api/client.js', 'w', encoding='utf-8') as f:
    f.write(api_code)

# Break components.js
with open('js/components.js', 'r', encoding='utf-8') as f:
    comp_code = f.read()

# We need to split into toast, sidebar, topbar
toast_code = """export class ToastManager extends HTMLElement {
    constructor() {
        super();
        this.attachShadow({ mode: 'open' });
        this.shadowRoot.innerHTML = `
            <style>
                :host {
                    position: fixed;
                    bottom: 20px;
                    right: 20px;
                    display: flex;
                    flex-direction: column;
                    gap: 10px;
                    z-index: 9999;
                    pointer-events: none;
                }
                .toast {
                    background-color: var(--bg-surface-elevated, #27272a);
                    color: var(--text-primary, #fff);
                    padding: 12px 20px;
                    border-radius: 8px;
                    border: 1px solid var(--border-strong, #3f3f46);
                    box-shadow: 0 10px 15px -3px rgba(0,0,0,0.5);
                    font-family: inherit;
                    font-size: 0.9rem;
                    transform: translateX(120%);
                    opacity: 0;
                    transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.3s ease;
                    pointer-events: auto;
                }
                .toast.show {
                    transform: translateX(0);
                    opacity: 1;
                }
                .toast.success { border-left: 4px solid var(--color-success, #10b981); }
                .toast.error { border-left: 4px solid var(--color-danger, #ef4444); }
            </style>
            <div id="container"></div>
        `;
    }

    show(message, type = 'default') {
        const container = this.shadowRoot.getElementById('container');
        const el = document.createElement('div');
        el.className = `toast ${type}`;
        el.textContent = message;
        
        container.appendChild(el);
        
        requestAnimationFrame(() => el.classList.add('show'));
        
        setTimeout(() => {
            el.classList.remove('show');
            setTimeout(() => el.remove(), 300);
        }, 3000);
    }
}
customElements.define('toast-manager', ToastManager);

export function showToast(msg, type) {
    let tm = document.querySelector('toast-manager');
    if (!tm) {
        tm = document.createElement('toast-manager');
        document.body.appendChild(tm);
    }
    tm.show(msg, type);
}
"""

with open('js/components/toast.js', 'w', encoding='utf-8') as f:
    f.write(toast_code)


sidebar_code = """import ApiClient from '../api/client.js';

export class AppSidebar extends HTMLElement {
    connectedCallback() {
        const activePage = this.getAttribute('active') || 'home';
        
        this.innerHTML = `
            <aside class="sidebar">
                <div class="sidebar-header">
                    <div class="logo">
                        <i class="fa-solid fa-chart-line logo-icon text-primary"></i>
                        <span>PROPHIT</span>
                    </div>
                </div>
                
                <nav class="sidebar-nav">
                    <a href="/dashboard.html" class="nav-link ${activePage === 'home' ? 'active' : ''}"><i class="fa-solid fa-house"></i> Dashboard</a>
                    <a href="/dashboard.html?view=markets" class="nav-link ${activePage === 'markets' ? 'active' : ''}"><i class="fa-solid fa-chart-simple"></i> Markets</a>
                    <a href="/profile.html" class="nav-link ${activePage === 'portfolio' ? 'active' : ''}"><i class="fa-solid fa-chart-pie"></i> Portfolio</a>
                    <a href="/wallet.html" class="nav-link ${activePage === 'wallet' ? 'active' : ''}"><i class="fa-solid fa-wallet"></i> Wallet</a>
                    <a href="/wallet.html#rewards" class="nav-link ${activePage === 'rewards' ? 'active' : ''}"><i class="fa-solid fa-gift"></i> Rewards</a>
                    <a href="/dashboard.html?view=leaderboard" class="nav-link ${activePage === 'leaderboard' ? 'active' : ''}"><i class="fa-solid fa-ranking-star"></i> Leaderboard</a>
                    <a href="#" class="nav-link"><i class="fa-solid fa-bell"></i> Notifications</a>
                    <a href="/profile.html" class="nav-link"><i class="fa-solid fa-user"></i> Profile</a>
                    <a href="#" class="nav-link"><i class="fa-solid fa-gear"></i> Settings</a>
                    <a href="#" class="nav-link"><i class="fa-solid fa-headset"></i> Support</a>
                    
                    <button class="btn btn-outline w-full" style="margin-top: 1rem; color: var(--color-gold); border-color: var(--color-gold);" onclick="window.dispatchEvent(new Event('openProposeModal'))">
                        <i class="fa-solid fa-plus-circle"></i> Propose Market
                    </button>
                </nav>

                <div class="wallet-widget">
                    <div class="wallet-header">
                        <i class="fa-solid fa-wallet text-gold"></i> Quick Balance
                    </div>
                    <div class="flex justify-between items-center" style="margin-top: 0.5rem">
                        <div class="balance-amount font-bold"><span id="sidebarBalance">--</span> PTS</div>
                        <a href="/wallet.html" class="btn btn-primary" style="padding: 0.2rem 0.5rem; font-size: 0.8rem;">Deposit</a>
                    </div>
                </div>
            </aside>
        `;
        
        this.fetchBalance();
    }
    
    async fetchBalance() {
        if (!ApiClient.isAuthenticated()) return;
        try {
            const data = await ApiClient.get('/me');
            const el = document.getElementById('sidebarBalance');
            if (el) el.textContent = data.points;
        } catch(e) {}
    }
}
customElements.define('app-sidebar', AppSidebar);
"""

with open('js/components/sidebar.js', 'w', encoding='utf-8') as f:
    f.write(sidebar_code)

topbar_code = """import ApiClient from '../api/client.js';

export function logout() {
    ApiClient.removeToken();
    window.location.href = '/login.html';
}

export class AppTopbar extends HTMLElement {
    connectedCallback() {
        this.innerHTML = `
            <header class="topbar flex justify-between items-center" style="padding: var(--spacing-4) var(--spacing-6); border-bottom: 1px solid var(--border-subtle); background: var(--bg-base);">
                <div class="search-container" style="flex: 1; max-width: 400px; position: relative;">
                    <i class="fa-solid fa-search" style="position: absolute; left: 15px; top: 50%; transform: translateY(-50%); color: var(--text-muted);"></i>
                    <input type="text" class="input-control" placeholder="Search markets, categories..." style="padding-left: 40px; border-radius: var(--radius-full); background: var(--bg-surface);">
                </div>
                
                <div class="topbar-actions flex items-center gap-4">
                    <button class="btn btn-outline" onclick="window.location.href='/wallet.html'" style="border-radius: var(--radius-full);">
                        <i class="fa-solid fa-coins text-gold"></i> <span id="topbarBalance">--</span> PTS
                    </button>
                    
                    <div class="profile-menu" style="position: relative; cursor: pointer;">
                        <img src="https://ui-avatars.com/api/?name=User&background=D4AF37&color=0A1128" alt="Profile" style="width: 40px; height: 40px; border-radius: 50%; border: 2px solid var(--border-subtle);" onclick="document.getElementById('profileDropdown').classList.toggle('hidden')">
                        
                        <div id="profileDropdown" class="hidden" style="position: absolute; top: 50px; right: 0; background: var(--bg-surface-elevated); border: 1px solid var(--border-strong); border-radius: var(--radius-md); width: 200px; box-shadow: var(--shadow-lg); z-index: 100;">
                            <a href="/profile.html" class="nav-link" style="padding: 10px 15px; display: block; color: var(--text-primary);"><i class="fa-solid fa-user"></i> Profile</a>
                            <a href="/wallet.html" class="nav-link" style="padding: 10px 15px; display: block; color: var(--text-primary);"><i class="fa-solid fa-wallet"></i> Wallet & KYC</a>
                            <div style="height: 1px; background: var(--border-strong); margin: 5px 0;"></div>
                            <a href="#" id="logoutBtn" class="nav-link text-danger" style="padding: 10px 15px; display: block;"><i class="fa-solid fa-right-from-bracket"></i> Logout</a>
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
                    adminL.innerHTML = '<i class="fa-solid fa-lock"></i> Admin Panel';
                    drop.insertBefore(adminL, drop.firstChild);
                }
            }
        } catch(e) {}
    }
}
customElements.define('app-topbar', AppTopbar);
"""

with open('js/components/topbar.js', 'w', encoding='utf-8') as f:
    f.write(topbar_code)

# Clean up old files
os.remove('js/api.js')
os.remove('js/components.js')
print("Successfully split components and API client.")
