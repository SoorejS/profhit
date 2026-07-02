import ApiClient from '../api/client.js';

export class AppSidebar extends HTMLElement {
    connectedCallback() {
        const activePage = this.getAttribute('active') || 'home';
        const mode = this.getAttribute('mode') || 'user';
        
        let navLinks = '';
        
        if (mode === 'admin') {
            navLinks = `
                <a href="/admin.html" class="nav-link ${activePage === 'home' ? 'active' : ''}"><i class="fa-solid fa-house"></i> Dashboard</a>
                <a href="#" class="nav-link"><i class="fa-solid fa-users"></i> Users</a>
                <a href="#" class="nav-link"><i class="fa-solid fa-chart-simple"></i> Markets</a>
                <a href="#" class="nav-link"><i class="fa-solid fa-id-card"></i> KYC</a>
                <a href="#" class="nav-link"><i class="fa-solid fa-file-invoice-dollar"></i> Wallet Logs</a>
                <a href="#" class="nav-link"><i class="fa-solid fa-chart-pie"></i> Analytics</a>
                <a href="#" class="nav-link"><i class="fa-solid fa-rectangle-ad"></i> Advertiser Portal (Placeholder)</a>
                <a href="#" class="nav-link"><i class="fa-solid fa-star"></i> Sponsored Predictions (Placeholder)</a>
                
                <a href="/dashboard.html" class="nav-link text-primary" style="margin-top: 1rem;"><i class="fa-solid fa-arrow-left"></i> Exit Admin</a>
            `;
        } else {
            navLinks = `
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
            `;
        }

        this.innerHTML = `
            <aside class="sidebar">
                <div class="sidebar-header">
                    <div class="logo">
                        <i class="fa-solid fa-chart-line logo-icon text-primary"></i>
                        <span>PROPHIT ${mode === 'admin' ? 'ADMIN' : ''}</span>
                    </div>
                </div>
                
                <nav class="sidebar-nav">
                    ${navLinks}
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
