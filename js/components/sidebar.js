import ApiClient from '../api/client.js';

export class AppSidebar extends HTMLElement {
    connectedCallback() {
        const activePage = this.getAttribute('active') || 'home';
        const mode = this.getAttribute('mode') || 'user';
        
        let navLinks = '';
        
        if (mode === 'admin') {
            navLinks = `
                <a href="/admin.html" class="nav-link ${activePage === 'home' ? 'active' : ''}"><i class="ph ph-house"></i> Dashboard</a>
                <a href="javascript:void(0)" class="nav-link"><i class="ph ph-users"></i> Users</a>
                <a href="javascript:void(0)" class="nav-link"><i class="ph ph-chart-line-up"></i> Markets</a>
                <a href="javascript:void(0)" class="nav-link"><i class="ph ph-identification-card"></i> KYC</a>
                <a href="javascript:void(0)" class="nav-link"><i class="ph ph-receipt"></i> Wallet Logs</a>
                <a href="javascript:void(0)" class="nav-link"><i class="ph ph-chart-pie-slice"></i> Analytics</a>
                <a href="javascript:void(0)" class="nav-link" style="opacity:0.6; cursor:not-allowed;" title="Coming Soon"><i class="ph ph-megaphone"></i> Advertiser Portal <span class="badge badge-warning" style="margin-left:auto;font-size:0.6rem;">Coming Soon</span></a>
                <a href="javascript:void(0)" class="nav-link" style="opacity:0.6; cursor:not-allowed;" title="Coming Soon"><i class="ph ph-star"></i> Sponsored Predictions <span class="badge badge-warning" style="margin-left:auto;font-size:0.6rem;">Coming Soon</span></a>
                
                <a href="/dashboard.html" class="nav-link text-primary" style="margin-top: 1rem;"><i class="ph ph-arrow-left"></i> Exit Admin</a>
            `;
        } else {
            navLinks = `
                <a href="/dashboard.html" class="nav-link ${activePage === 'home' ? 'active' : ''}"><i class="ph ph-house"></i> Dashboard</a>
                <a href="/dashboard.html?view=markets" class="nav-link ${activePage === 'markets' ? 'active' : ''}"><i class="ph ph-chart-line-up"></i> Markets</a>
                <a href="/portfolio.html" class="nav-link ${activePage === 'portfolio' ? 'active' : ''}"><i class="ph ph-chart-pie-slice"></i> Portfolio</a>
                <a href="/wallet.html" class="nav-link ${activePage === 'wallet' ? 'active' : ''}"><i class="ph ph-wallet"></i> Wallet</a>
                <a href="/rewards.html" class="nav-link ${activePage === 'rewards' ? 'active' : ''}"><i class="ph ph-gift"></i> Rewards</a>
                <a href="/dashboard.html?view=leaderboard" class="nav-link ${activePage === 'leaderboard' ? 'active' : ''}"><i class="ph ph-ranking"></i> Leaderboard</a>
                <a href="/notifications.html" class="nav-link ${activePage === 'notifications' ? 'active' : ''}"><i class="ph ph-bell"></i> Notifications</a>
                <a href="/profile.html" class="nav-link"><i class="ph ph-user"></i> Profile</a>
                <a href="/settings.html" class="nav-link ${activePage === 'settings' ? 'active' : ''}"><i class="ph ph-gear"></i> Settings</a>
                <a href="/support.html" class="nav-link ${activePage === 'support' ? 'active' : ''}"><i class="ph ph-headset"></i> Support</a>
                
                <a href="/propose-market.html" class="btn btn-outline w-full ${activePage === 'propose' ? 'active' : ''}" style="margin-top: 1rem; color: var(--color-gold); border-color: var(--color-gold); text-align: center; display: block; text-decoration: none;">
                    <i class="ph ph-plus-circle"></i> Propose Market
                </a>
            `;
        }

        this.innerHTML = `
            <aside class="sidebar">
                <div class="sidebar-header">
                    <div class="logo">
                        <i class="ph-fill ph-trend-up logo-icon text-primary"></i>
                        <span>PROPHIT ${mode === 'admin' ? 'ADMIN' : ''}</span>
                    </div>
                </div>
                
                <nav class="sidebar-nav">
                    ${navLinks}
                </nav>

                <div class="wallet-widget">
                    <div class="wallet-header">
                        <i class="ph-fill ph-wallet text-gold"></i> Quick Balance
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
