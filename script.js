// ============================================================
// PROPHIT — Frontend App Script
// ============================================================

// API URL from config.js (auto-switches local ↔ production)
const API_URL = CONFIG.API_URL;

// ---- Auth State ----
let authToken = localStorage.getItem('prophit_token');
let currentUser = JSON.parse(localStorage.getItem('prophit_user') || 'null');

// ---- Trade Modal State ----
const tradeSidebar    = document.getElementById('tradeSidebar');
const sidebarOverlay  = document.getElementById('sidebarOverlay');
const modalMarketName = document.getElementById('modalMarketName');
const inputAmount     = document.getElementById('investmentAmount');
const avgPriceDisplay = document.getElementById('avgPriceDisplay');
const sharesDisplay   = document.getElementById('sharesDisplay');
const expectedReturnDisplay = document.getElementById('expectedReturn');
const submitTradeBtn  = document.getElementById('submitTradeBtn');
const tabYes = document.getElementById('tabYes');
const tabNo  = document.getElementById('tabNo');

let currentSide      = 'yes';
let currentPriceYes  = 65;
let currentPriceNo   = 35;
let currentMarketId  = null;

// ============================================================
// AUTH FUNCTIONS
// ============================================================
function getAuthHeaders() {
    return {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${authToken}`
    };
}

function switchAuthTab(tab) {
    const loginForm    = document.getElementById('loginForm');
    const registerForm = document.getElementById('registerForm');
    const loginTab     = document.getElementById('loginTab');
    const registerTab  = document.getElementById('registerTab');

    if (tab === 'login') {
        loginForm.classList.remove('hidden');
        registerForm.classList.add('hidden');
        loginTab.classList.add('active');
        registerTab.classList.remove('active');
    } else {
        registerForm.classList.remove('hidden');
        loginForm.classList.add('hidden');
        registerTab.classList.add('active');
        loginTab.classList.remove('active');
    }
}

async function handleLogin(e) {
    e.preventDefault();
    const btn = document.getElementById('loginBtn');
    const errorEl = document.getElementById('loginError');
    errorEl.textContent = '';
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> Signing in...';
    btn.disabled = true;

    try {
        const res = await fetch(`${API_URL}/auth/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                email: document.getElementById('loginEmail').value,
                password: document.getElementById('loginPassword').value
            })
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Login failed');

        onAuthSuccess(data);
    } catch (err) {
        errorEl.textContent = err.message;
        btn.innerHTML = '<span>Sign In</span>';
        btn.disabled = false;
    }
}

async function handleRegister(e) {
    e.preventDefault();
    const btn = document.getElementById('registerBtn');
    const errorEl = document.getElementById('registerError');
    errorEl.textContent = '';
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> Creating...';
    btn.disabled = true;

    try {
        const res = await fetch(`${API_URL}/auth/register`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                username: document.getElementById('regUsername').value,
                email: document.getElementById('regEmail').value,
                password: document.getElementById('regPassword').value
            })
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Registration failed');

        onAuthSuccess(data);
    } catch (err) {
        errorEl.textContent = err.message;
        btn.innerHTML = '<span>Create Account</span>';
        btn.disabled = false;
    }
}

function onAuthSuccess(data) {
    authToken = data.token;
    currentUser = data.user;
    localStorage.setItem('prophit_token', authToken);
    localStorage.setItem('prophit_user', JSON.stringify(currentUser));

    document.getElementById('authOverlay').style.display = 'none';
    updateTopbarFromUser(currentUser);
    fetchMarkets();
    fetchUserStats();
}

function logout() {
    localStorage.removeItem('prophit_token');
    localStorage.removeItem('prophit_user');
    authToken = null;
    currentUser = null;
    window.location.reload();
}

function updateTopbarFromUser(user) {
    if (!user) return;
    const balanceEl     = document.querySelector('.points-value');
    const progressFill  = document.querySelector('.progress-fill-mini');
    const profileName   = document.querySelector('#profileDropdown .dropdown-header strong');
    const profileImg    = document.getElementById('profileImage');

    if (balanceEl)    balanceEl.textContent = user.points;
    if (progressFill) progressFill.style.width = `${Math.min(user.points, 100)}%`;
    if (profileName)  profileName.innerHTML = `@${user.username} <span class="badge" style="background: rgba(212, 175, 55, 0.15); color: var(--gold-light); width: fit-content; margin-top: 4px;">${user.tier} Tier</span>`;
    if (profileImg)   profileImg.src = `https://ui-avatars.com/api/?name=${user.username}&background=D4AF37&color=0A1128&bold=true`;

    const kycBtnText = document.getElementById('kycBtnText');
    if (kycBtnText) kycBtnText.textContent = user.tier;
    
    const sidebarBalance = document.querySelector('.sidebar-points-value');
    if (sidebarBalance) sidebarBalance.textContent = user.points;
    
    const sidebarKycBtnText = document.getElementById('sidebarKycBtnText');
    if (sidebarKycBtnText) sidebarKycBtnText.textContent = user.tier;

    const navAdminBtn = document.getElementById('navAdminBtn');
    if (navAdminBtn && user.username === 'You') {
        navAdminBtn.classList.remove('hidden');
    }
}

async function forgotPassword() {
    const email = prompt("Enter your registered email address:");
    if (!email) return;

    try {
        const res = await fetch(`${API_URL}/auth/forgot-password`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email })
        });
        const data = await res.json();
        alert(data.message);
    } catch (err) {
        console.error(err);
    }
}

async function upgradeKyc() {
    if (!authToken) return;
    document.getElementById('kycOverlay').style.display = 'flex';
}

async function handleKycSubmit(event) {
    event.preventDefault();
    const docId = document.getElementById('kycDocId').value;
    const fileInput = document.getElementById('kycFile');
    
    if (!fileInput.files[0]) return alert("Please select an image");

    const formData = new FormData();
    formData.append('document_id', docId);
    formData.append('document_image', fileInput.files[0]);

    try {
        const res = await fetch(`${API_URL}/me/kyc`, {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${authToken}`
                // Note: Don't set Content-Type header for FormData, browser does it with boundary
            },
            body: formData
        });
        const data = await res.json();
        
        if (res.ok) {
            authToken = data.token;
            localStorage.setItem('prophit_token', authToken);
            showToast("✅ Verification Submitted Successfully!");
            document.getElementById('kycOverlay').style.display = 'none';
            fetchUserStats();
        } else {
            alert(data.error || 'Verification failed');
        }
    } catch (err) {
        console.error(err);
    }
}

// ============================================================
// PAYMENTS (RAZORPAY)
// ============================================================
function openDepositModal() {
    document.getElementById('depositOverlay').style.display = 'flex';
}

async function initiateDeposit() {
    if (!authToken) return alert("Please log in first.");
    const amount = parseFloat(document.getElementById('depositAmount').value);
    if (isNaN(amount) || amount < 100) return alert("Minimum deposit is ₹100");

    try {
        // 1. Create order
        const res = await fetch(`${API_URL}/payments/create-order`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ amount })
        });
        const orderData = await res.json();
        
        if (!res.ok) throw new Error(orderData.error);

        // 2. Open Razorpay Widget
        const options = {
            "key": orderData.key,
            "amount": orderData.amount,
            "currency": orderData.currency,
            "name": "PROPHIT",
            "description": "Wallet Deposit",
            "order_id": orderData.order_id,
            "handler": async function (response) {
                // 3. Verify payment
                const verifyRes = await fetch(`${API_URL}/payments/verify`, {
                    method: 'POST',
                    headers: getAuthHeaders(),
                    body: JSON.stringify({
                        razorpay_payment_id: response.razorpay_payment_id || "mock_payment_id",
                        razorpay_order_id: response.razorpay_order_id || orderData.order_id,
                        razorpay_signature: response.razorpay_signature || "mock_signature",
                        points: amount
                    })
                });
                const verifyData = await verifyRes.json();
                if (verifyRes.ok) {
                    showToast(`✅ ${verifyData.message}`);
                    document.getElementById('depositOverlay').style.display = 'none';
                    fetchUserStats();
                } else {
                    alert(`❌ Verification failed: ${verifyData.error}`);
                }
            },
            "prefill": {
                "name": currentUser.username,
                "email": currentUser.email
            },
            "theme": { "color": "#10B981" }
        };
        
        if (orderData.key === "mock_key_123") {
            // Mock mode simulation
            console.log("Mock Payment Mode active. Simulating success...");
            options.handler({});
        } else {
            const rzp = new Razorpay(options);
            rzp.on('payment.failed', function (response){
                alert("Payment Failed: " + response.error.description);
            });
            rzp.open();
        }
        
    } catch (err) {
        alert("Error initiating payment: " + err.message);
        console.error(err);
    }
}

async function withdrawFunds() {
    if (!authToken) return alert("Please log in first.");
    const amountStr = prompt(`Enter amount to withdraw (Max: ${currentUser?.points || 0} PTS)`);
    if (!amountStr) return;
    
    const amount = parseFloat(amountStr);
    if (isNaN(amount) || amount <= 0) return alert("Invalid amount");

    try {
        const res = await fetch(`${API_URL}/payments/withdraw`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ amount })
        });
        const data = await res.json();
        
        if (res.ok) {
            showToast(`✅ ${data.message}`);
            fetchUserStats();
        } else {
            alert(`❌ Failed: ${data.error}`);
        }
    } catch (err) {
        console.error(err);
    }
}

// ============================================================
// API FETCH FUNCTIONS
// ============================================================
async function fetchUserStats() {
    if (!authToken) return;
    try {
        const res = await fetch(`${API_URL}/me`, { headers: getAuthHeaders() });
        if (res.status === 401) {
            logout();
            return;
        }
        if (!res.ok) return;
        const user = await res.json();
        currentUser = user;
        localStorage.setItem('prophit_user', JSON.stringify(user));
        updateTopbarFromUser(user);
    } catch (err) {
        console.error('Failed to fetch user stats:', err);
    }
}

async function fetchMarkets(category = '') {
    try {
        let url = `${API_URL}/markets`;
        if (category) url += `?category=${encodeURIComponent(category)}`;
        const res = await fetch(url);
        const data = await res.json();
        renderMarkets(Array.isArray(data) ? data : []);
    } catch (err) {
        console.error('Failed to fetch markets:', err);
    }
}

async function fetchPortfolio() {
    if (!authToken) return;
    try {
        const res = await fetch(`${API_URL}/portfolio`, { headers: getAuthHeaders() });
        const data = await res.json();
        renderPortfolio(Array.isArray(data) ? data : []);
    } catch (err) {
        console.error('Failed to fetch portfolio:', err);
    }
}

async function fetchLeaderboard() {
    try {
        const res = await fetch(`${API_URL}/stats/leaderboard`);
        const data = await res.json();
        renderLeaderboard(Array.isArray(data) ? data : []);
    } catch (err) {
        console.error('Failed to fetch leaderboard:', err);
    }
}

async function fetchActivity() {
    try {
        const res = await fetch(`${API_URL}/stats/activity`);
        const data = await res.json();
        renderActivity(Array.isArray(data) ? data : []);
    } catch (err) {
        console.error('Failed to fetch activity:', err);
    }
}

// ============================================================
// RENDER FUNCTIONS
// ============================================================
function renderMarkets(markets) {
    const activeView = document.querySelector('.view-section.active');
    if (!activeView) return;
    const grid = activeView.querySelector('.markets-grid');
    if (!grid) return;

    if (markets.length === 0) {
        grid.innerHTML = `<div style="grid-column:1/-1; text-align:center; padding:3rem; color:var(--text-secondary);">
            <i class="fa-solid fa-chart-line" style="font-size:2rem; margin-bottom:1rem; display:block;"></i>
            No markets found in this category yet.
        </div>`;
        return;
    }

    const iconMap = {
        'Politics': 'fa-landmark',
        'Finance': 'fa-money-bill-trend-up',
        'Technology': 'fa-microchip',
        'Global News': 'fa-globe'
    };

    grid.innerHTML = '';
    markets.forEach(m => {
        const icon = iconMap[m.category] || 'fa-chart-line';
        const vol = m.volume > 1000000 ? (m.volume/1000000).toFixed(1)+'M'
                  : m.volume > 1000    ? (m.volume/1000).toFixed(1)+'K'
                  : m.volume;
        const trendColor = m.yes_price >= 50 ? 'var(--color-yes)' : 'var(--color-no)';
        const endDate = m.end_date ? new Date(m.end_date).toLocaleDateString('en-IN', {day:'numeric', month:'short'}) : 'TBD';

        grid.innerHTML += `
        <div class="market-card" onclick="trade('yes', '${m.title.replace(/'/g,"\\'")}', ${m.yes_price}, ${m.no_price}, ${m.id})">
            <div class="card-top">
                <div class="market-icon"><i class="fa-solid ${icon}"></i></div>
                <div class="market-vol"><i class="fa-solid fa-chart-simple"></i> ₹${vol} Vol.</div>
            </div>
            <h4 class="card-title">${m.title}</h4>
            <div class="card-probability">
                <div class="prob-number" style="color:${trendColor}">${m.yes_price}%</div>
                <svg class="sparkline" viewBox="0 0 100 30">
                    <path d="M0 20 Q 25 5, 50 15 T 100 ${m.yes_price > 50 ? '5' : '25'}" fill="none" stroke="${trendColor}" stroke-width="2"/>
                </svg>
            </div>
            <div class="card-actions">
                <button class="trade-btn btn-yes" onclick="event.stopPropagation(); trade('yes', '${m.title.replace(/'/g,"\\'")}', ${m.yes_price}, ${m.no_price}, ${m.id})">
                    <span>Yes</span><span>${m.yes_price}¢</span>
                </button>
                <button class="trade-btn btn-no" onclick="event.stopPropagation(); trade('no', '${m.title.replace(/'/g,"\\'")}', ${m.yes_price}, ${m.no_price}, ${m.id})">
                    <span>No</span><span>${m.no_price}¢</span>
                </button>
            </div>
            <div class="card-footer">
                <span><i class="fa-regular fa-clock"></i> Closes ${endDate}</span>
                <a href="#" onclick="event.stopPropagation(); showResearch('${m.title.replace(/'/g,"\\'")}', '${(m.description||'').replace(/'/g,"\\'")}', '${m.resolution_source||''}')">
                    <i class="fa-solid fa-book-open"></i> Research
                </a>
            </div>
        </div>`;
    });
}

function showResearch(title, description, source) {
    alert(`📊 Research Report\n\n${title}\n\n${description || 'No description available.'}\n\n🔍 Resolution Source: ${source || 'TBD'}`);
}

function renderPortfolio(trades) {
    const tbody = document.getElementById('portfolioTrades');
    if (!tbody) return;

    if (!trades || trades.length === 0) {
        tbody.innerHTML = `<tr><td colspan="5" style="text-align:center; padding:2rem; color:var(--text-secondary);">
            No trades yet. Go buy some shares! 🚀
        </td></tr>`;
        return;
    }

    tbody.innerHTML = '';
    trades.forEach(t => {
        const sideClass = t.outcome === 'Yes' ? 'badge-yes' : 'badge-no';
        let payout = t.payout > 0 ? `<span class="text-green">+${t.payout} pts</span>` : `<span style="color:var(--text-secondary)">Pending</span>`;
        let action = '';
        if (t.shares > 0) {
            action = `<button class="auth-submit" style="padding:0.3rem 0.6rem; font-size:0.8rem;" onclick="sellShares(${t.id})">Sell</button>`;
        } else {
            action = `<span style="color:var(--text-secondary)">Sold/Resolved</span>`;
        }

        tbody.innerHTML += `
        <tr>
            <td>${t.market_title || `Market #${t.market_id}`}</td>
            <td><span class="badge ${sideClass}">${t.outcome}</span></td>
            <td>${parseFloat(t.shares).toFixed(2)}</td>
            <td>${t.price}¢</td>
            <td>${payout}</td>
            <td>${action}</td>
        </tr>`;
    });
}

async function fetchLimitOrders() {
    if (!authToken) return;
    try {
        const res = await fetch(`${API_URL}/trades/limit`, { headers: getAuthHeaders() });
        const data = await res.json();
        renderLimitOrders(Array.isArray(data) ? data : []);
    } catch (err) {
        console.error('Failed to fetch limit orders:', err);
    }
}

function renderLimitOrders(orders) {
    const tbody = document.getElementById('portfolioLimitOrders');
    if (!tbody) return;

    if (!orders || orders.length === 0) {
        tbody.innerHTML = `<tr><td colspan="5" style="text-align:center; padding:2rem; color:var(--text-secondary);">
            No pending limit orders.
        </td></tr>`;
        return;
    }

    tbody.innerHTML = '';
    orders.forEach(o => {
        const sideClass = o.outcome === 'Yes' ? 'badge-yes' : 'badge-no';
        tbody.innerHTML += `
        <tr>
            <td><span class="badge ${sideClass}">${o.outcome}</span></td>
            <td>${o.target_price} pts</td>
            <td>${o.points} pts</td>
            <td><span style="color:var(--text-secondary)">${o.status}</span></td>
            <td>
                <button class="auth-submit" style="padding:0.3rem 0.6rem; font-size:0.8rem; background:var(--color-no);" onclick="cancelLimitOrder(${o.id})">Cancel</button>
            </td>
        </tr>`;
    });
}

async function cancelLimitOrder(id) {
    if (!confirm("Cancel this limit order? Your locked points will be refunded.")) return;
    try {
        const res = await fetch(`${API_URL}/trades/limit/${id}`, {
            method: 'DELETE',
            headers: getAuthHeaders()
        });
        const data = await res.json();
        if (res.ok) {
            showToast(`✅ ${data.message}`);
            fetchUserStats();
            fetchPortfolio();
            fetchLimitOrders();
        } else {
            alert(`❌ Failed: ${data.error}`);
        }
    } catch (err) {
        console.error(err);
    }
}

function renderLeaderboard(users) {
    const tbody = document.querySelector('#view-leaderboard .data-table tbody');
    if (!tbody) return;

    tbody.innerHTML = '';
    if (!users || users.length === 0) return;

    users.forEach((u, i) => {
        const medal = i === 0 ? '🥇' : i === 1 ? '🥈' : i === 2 ? '🥉' : `#${i+1}`;
        const isMe = currentUser && u.username === currentUser.username;
        tbody.innerHTML += `
        <tr class="${isMe ? 'current-user-row' : ''}">
            <td><strong>${medal}</strong></td>
            <td class="user-cell">
                <img src="https://ui-avatars.com/api/?name=${u.username}&background=random" alt="${u.username}">
                @${u.username} ${isMe ? '<span class="badge" style="background:rgba(212,175,55,0.2);color:var(--gold-light);font-size:0.7rem;">You</span>' : ''}
            </td>
            <td><span class="badge" style="background:var(--bg-hover);color:var(--gold-light);">${u.tier}</span></td>
            <td class="text-green"><strong>${u.points} pts</strong></td>
        </tr>`;
    });
}

function renderActivity(trades) {
    const container = document.querySelector('#view-activity .activity-feed');
    if (!container) return;

    if (!trades || trades.length === 0) {
        container.innerHTML = `<p style="color:var(--text-secondary);text-align:center;padding:3rem;">
            No recent activity yet. Be the first to trade!
        </p>`;
        return;
    }

    container.innerHTML = '';
    trades.forEach(t => {
        const color = t.outcome === 'Yes' ? 'var(--color-yes)' : 'var(--color-no)';
        // Format duration
        const timeAgo = t.time_ago.replace('h', 'h ').replace('m', 'm ').replace('s', 's ').trim();
        container.innerHTML += `
        <div class="activity-item" style="display:flex;align-items:center;padding:1rem;border-bottom:1px solid var(--border-color);gap:1rem;">
            <img src="https://ui-avatars.com/api/?name=${t.username}&background=random" style="width:40px;height:40px;border-radius:50%;flex-shrink:0;">
            <div style="flex:1;min-width:0;">
                <div><strong>@${t.username}</strong> bought <span style="color:${color};font-weight:bold;">${t.outcome}</span></div>
                <div style="font-size:0.83rem;color:var(--text-secondary);margin-top:3px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">${t.market}</div>
            </div>
            <div style="text-align:right;flex-shrink:0;">
                <div style="font-weight:bold;">${t.points} pts</div>
                <div style="font-size:0.75rem;color:var(--text-secondary);">${timeAgo} ago</div>
            </div>
        </div>`;
    });
}

// ============================================================
// TRADE MODAL
// ============================================================
function switchTradeType(type) {
    currentTradeType = type;
    const tabs = document.querySelectorAll('#tradeTypeTabs .auth-tab');
    tabs.forEach(t => t.classList.toggle('active', t.textContent.toLowerCase() === type));
    
    document.getElementById('limitPriceSection').style.display = type === 'limit' ? 'block' : 'none';
    document.getElementById('priceLabel').textContent = type === 'limit' ? 'Target Price' : 'Avg Price';
    
    updateTradeDetails();
}

function trade(side, marketTitle, yesPrice, noPrice, marketId) {
    if (!authToken) {
        document.getElementById('authOverlay').style.display = 'flex';
        return;
    }

    currentTradeMarketId = marketId;
    currentSide = side;
    
    // Update UI
    document.getElementById('modalMarketName').textContent = marketTitle;
    selectOutcome(side);
    
    if (typeof switchTradeSide === 'function') {
        switchTradeSide('buy');
    }
    
    // Open sidebar
    tradeSidebar.classList.add('active');
    sidebarOverlay.classList.add('active');
    
    fetchComments(marketId);
}

async function fetchComments(marketId) {
    try {
        const res = await fetch(`${API_URL}/markets/${marketId}/comments`);
        const comments = await res.json();
        renderComments(comments);
    } catch (err) {
        console.error("Failed to fetch comments", err);
    }
}

function renderComments(comments) {
    const list = document.getElementById('commentsList');
    list.innerHTML = comments.map(c => `
        <div class="comment-item" style="background: rgba(255,255,255,0.05); padding: 0.8rem; border-radius: 8px;">
            <div style="font-size: 0.75rem; color: var(--gold-light); margin-bottom: 0.2rem; font-weight: bold;">@${c.username}</div>
            <div style="font-size: 0.85rem; color: white;">${c.content}</div>
        </div>
    `).join('');
    list.scrollTop = list.scrollHeight;
}

async function postComment() {
    const input = document.getElementById('newCommentInput');
    const content = input.value.trim();
    if (!content || !currentTradeMarketId) return;

    try {
        const res = await fetch(`${API_URL}/markets/${currentTradeMarketId}/comments`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ content })
        });
        if (res.ok) {
            input.value = '';
            fetchComments(currentTradeMarketId);
        }
    } catch (err) {
        console.error(err);
    }
}

function closeSidebar() {
    tradeSidebar.classList.remove('active');
    sidebarOverlay.classList.remove('active');
}

function selectOutcome(side) {
    currentSide = side;
    if (side === 'yes') {
        tabYes.classList.add('active');
        tabNo.classList.remove('active');
        submitTradeBtn.style.backgroundColor = 'var(--color-yes)';
        submitTradeBtn.innerText = 'Buy Yes';
    } else {
        tabNo.classList.add('active');
        tabYes.classList.remove('active');
        submitTradeBtn.style.backgroundColor = 'var(--color-no)';
        submitTradeBtn.innerText = 'Buy No';
    }
    calculateTrade();
    if (currentTradeMode === 'sell') {
        fetchMarketPositions();
    }
}

function setAmount(val) {
    inputAmount.value = val;
    calculateTrade();
}

let currentTradeMode = 'buy';
function switchTradeSide(mode) {
    currentTradeMode = mode;
    document.getElementById('tabBuySide').classList.toggle('active', mode === 'buy');
    document.getElementById('tabSellSide').classList.toggle('active', mode === 'sell');
    
    const tradeTypeTabs = document.getElementById('tradeTypeTabs');
    const limitPriceSec = document.getElementById('limitPriceSection');
    const buySec = document.getElementById('buySection');
    const sellSec = document.getElementById('sellSection');
    
    if (mode === 'buy') {
        if(tradeTypeTabs) tradeTypeTabs.style.display = 'flex';
        buySec.style.display = 'block';
        sellSec.style.display = 'none';
        submitTradeBtn.innerText = currentSide === 'yes' ? 'Buy Yes' : 'Buy No';
    } else {
        if(tradeTypeTabs) tradeTypeTabs.style.display = 'none';
        if(limitPriceSec) limitPriceSec.style.display = 'none';
        buySec.style.display = 'none';
        sellSec.style.display = 'block';
        fetchMarketPositions();
    }
}

let currentTradeType = 'market';
function switchTradeType(type) {
    currentTradeType = type;
    const tabs = document.querySelectorAll('#tradeTypeTabs button');
    tabs[0].classList.toggle('active', type === 'market');
    tabs[1].classList.toggle('active', type === 'limit');
    
    document.getElementById('limitPriceSection').style.display = type === 'limit' ? 'block' : 'none';
}

async function fetchMarketPositions() {
    const list = document.getElementById('activePositionsList');
    list.innerHTML = '<p style="color:var(--text-secondary);font-size:0.9rem;">Loading positions...</p>';
    if (!authToken || !currentTradeMarketId) return;

    try {
        const res = await fetch(`${API_URL}/portfolio`, { headers: getAuthHeaders() });
        const data = await res.json();
        const trades = Array.isArray(data) ? data : [];
        const marketTrades = trades.filter(t => t.market_id === currentTradeMarketId && t.shares > 0);
        
        if (marketTrades.length === 0) {
            list.innerHTML = '<p style="color:var(--text-secondary);font-size:0.9rem;">You have no active shares for this market.</p>';
            return;
        }

        list.innerHTML = '';
        marketTrades.forEach(t => {
            const isYes = t.outcome.toLowerCase() === 'yes';
            const colorClass = isYes ? 'text-green' : 'text-red';
            list.innerHTML += `
            <div style="background:var(--bg-card); padding:1rem; border-radius:8px; border:1px solid var(--border-color); display:flex; justify-content:space-between; align-items:center;">
                <div>
                    <strong class="${colorClass}">${t.outcome} Shares</strong>
                    <div style="font-size:0.8rem; color:var(--text-secondary); margin-top:4px;">
                        ${parseFloat(t.shares).toFixed(2)} shares @ ${t.price}¢
                    </div>
                </div>
                <button class="auth-submit" style="width:auto; padding:0.4rem 1rem; font-size:0.8rem;" onclick="sellSharesSidebar(${t.id})">
                    Sell All
                </button>
            </div>
            `;
        });
    } catch (err) {
        list.innerHTML = '<p style="color:var(--text-secondary);font-size:0.9rem;">Failed to load positions.</p>';
    }
}

async function sellSharesSidebar(tradeId) {
    if (!confirm("Are you sure you want to sell these shares at the current market price?")) return;
    
    try {
        const res = await fetch(`${API_URL}/trades/${tradeId}/sell`, {
            method: 'POST',
            headers: getAuthHeaders()
        });
        const data = await res.json();
        
        if (res.ok) {
            showToast(`✅ ${data.message} (Payout: ${data.payout} pts)`);
            fetchUserStats();
            fetchPortfolio();
            fetchMarketPositions();
            
            // Also refresh market volume/prices
            fetchMarkets();
        } else {
            alert(`❌ Failed: ${data.error}`);
        }
    } catch (err) {
        console.error(err);
        alert('Network error while selling.');
    }
}

function calculateTrade() {
    let amount = parseInt(inputAmount.value) || 0;
    if (amount > 100) { amount = 100; inputAmount.value = 100; }
    if (amount < 1 && inputAmount.value !== '') { amount = 1; inputAmount.value = 1; }

    const price = currentSide === 'yes' ? currentPriceYes : currentPriceNo;
    const priceInPts = price / 100;
    const shares = amount > 0 ? amount / priceInPts : 0;

    avgPriceDisplay.innerText     = `${price.toFixed(2)} pts`;
    sharesDisplay.innerText       = shares.toFixed(2);
    expectedReturnDisplay.innerText = `${shares.toFixed(2)} pts`;
}

inputAmount.addEventListener('input', calculateTrade);

async function confirmTrade() {
    const amount = parseInt(inputAmount.value) || 0;
    if (amount === 0) return;
    if (!authToken) {
        document.getElementById('authOverlay').style.display = 'flex';
        return;
    }

    submitTradeBtn.innerText = 'Processing...';
    submitTradeBtn.disabled  = true;

    try {
        let endpoint = `${API_URL}/trades`;
        let payload = {
            market_id: currentTradeMarketId,
            outcome: currentSide,
            points: amount
        };

        const tradeTypeTabs = document.getElementById('tradeTypeTabs');
        const isLimitOrder = tradeTypeTabs && tradeTypeTabs.style.display !== 'none' && currentTradeType === 'limit';
        
        if (isLimitOrder) {
            endpoint = `${API_URL}/trades/limit`;
            payload.target_price = parseInt(document.getElementById('limitPriceInput').value);
        }

        const res = await fetch(endpoint, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(payload)
        });
        const data = await res.json();

        if (res.ok) {
            if (isLimitOrder) {
                showToast(`✅ Limit Order placed! Waiting for price to reach ${payload.target_price}pts.`);
                fetchLimitOrders();
            } else {
                showToast(`✅ Trade confirmed! You bought ${sharesDisplay.innerText} ${currentSide.toUpperCase()} shares.`);
            }

            // Refresh wallet
            if (data.balance !== undefined) {
                currentUser.points = data.balance;
                localStorage.setItem('prophit_user', JSON.stringify(currentUser));
                updateTopbarFromUser(currentUser);
            }

            closeSidebar();
            fetchUserStats();
            fetchPortfolio();
        } else {
            alert(`❌ Failed: ${data.error}`);
        }
    } catch (err) {
        console.error(err);
        alert('Network error while placing trade.');
    } finally {
        submitTradeBtn.disabled  = false;
        submitTradeBtn.innerText = currentSide === 'yes' ? 'Buy Yes' : 'Buy No';
    }
}

function showToast(message) {
    let toast = document.getElementById('prophit-toast');
    if (!toast) {
        toast = document.createElement('div');
        toast.id = 'prophit-toast';
        toast.style.cssText = `
            position:fixed; bottom:2rem; right:2rem; z-index:9999;
            background:var(--bg-card); border:1px solid var(--color-yes);
            color:var(--text-primary); padding:1rem 1.5rem; border-radius:12px;
            font-weight:600; box-shadow:0 8px 32px rgba(0,0,0,0.4);
            transition:all 0.3s; max-width:360px;
        `;
        document.body.appendChild(toast);
    }
    toast.textContent = message;
    toast.style.opacity = '1';
    toast.style.transform = 'translateY(0)';
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateY(20px)';
    }, 4000);
}

// ============================================================
// NAVIGATION
// ============================================================
const navLinks = document.querySelectorAll('.nav-link');
const views    = document.querySelectorAll('.view-section');

const categoryMap = {
    'view-category-politics':   'Politics',
    'view-category-finance':    'Finance',
    'view-category-technology': 'Technology',
    'view-category-global':     'Global News'
};

navLinks.forEach(link => {
    link.addEventListener('click', e => {
        const targetId = link.getAttribute('data-target');
        if (!targetId) return;
        e.preventDefault();

        navLinks.forEach(l => l.classList.toggle('active', l.getAttribute('data-target') === targetId));
        views.forEach(v => v.classList.toggle('active', v.id === targetId));

        const targetView = document.getElementById(targetId);
        if (targetView) {
            document.querySelector('.content-wrapper')?.scrollTo(0, 0);

            if (categoryMap[targetId]) {
                fetchMarkets(categoryMap[targetId]);
            } else if (targetId === 'view-markets') {
                fetchMarkets();
            } else if (targetId === 'view-portfolio') {
                fetchPortfolio();
                fetchLimitOrders();
            } else if (targetId === 'view-leaderboard') {
                fetchLeaderboard();
            } else if (targetId === 'view-activity') {
                fetchActivity();
            } else if (targetId === 'view-admin') {
                fetchAdminMarkets();
            }
        }
    });
});

// ============================================================
// SEARCH
// ============================================================
const searchInput = document.querySelector('.search-container input');
if (searchInput) {
    searchInput.addEventListener('input', e => {
        const q = e.target.value.toLowerCase();
        document.querySelectorAll('.market-card').forEach(card => {
            const title = card.querySelector('.card-title')?.innerText.toLowerCase() || '';
            card.style.display = title.includes(q) ? '' : 'none';
        });
    });
}

// ============================================================
// PROFILE DROPDOWN
// ============================================================
const profileImage    = document.getElementById('profileImage');
const profileDropdown = document.getElementById('profileDropdown');

if (profileImage && profileDropdown) {
    profileImage.addEventListener('click', e => {
        e.stopPropagation();
        profileDropdown.classList.toggle('active');
    });

    document.addEventListener('click', e => {
        if (!profileDropdown.contains(e.target) && e.target !== profileImage) {
            profileDropdown.classList.remove('active');
        }
    });

    profileDropdown.querySelectorAll('.dropdown-item').forEach(item => {
        item.addEventListener('click', e => {
            e.preventDefault();
            const text = item.innerText.trim();
            if (text.includes('Logout')) {
                logout();
            } else if (text.includes('Profile')) {
                alert(`@${currentUser?.username}\nTier: ${currentUser?.tier}\nPoints: ${currentUser?.points}`);
            } else {
                alert(`${text} — coming soon!`);
            }
        });
    });
}

// ============================================================
// WEBSOCKETS
// ============================================================
let ws;
function initWebSocket() {
    const wsUrl = API_URL.replace('http', 'ws') + '/ws';
    ws = new WebSocket(wsUrl);

    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        if (data.event === 'trade_placed' || data.event === 'market_update') {
            fetchMarkets(); 
        } else if (data.event === 'new_comment') {
            if (currentTradeMarketId == data.payload.market_id) {
                fetchComments(currentTradeMarketId);
            }
        } else if (data.event === 'limit_order_executed') {
            if (data.payload.user_id === currentUser?.id) {
                showToast(`🚀 ${data.payload.message}`);
                fetchUserStats();
                fetchPortfolio();
                fetchLimitOrders();
            }
        }
    };
    
    ws.onclose = () => {
        setTimeout(initWebSocket, 5000); // Reconnect after 5s
    };
}

// ============================================================
// SELL SHARES
// ============================================================
async function sellShares(tradeId) {
    if (!confirm("Are you sure you want to sell these shares at the current market price?")) return;
    
    try {
        const res = await fetch(`${API_URL}/trades/${tradeId}/sell`, {
            method: 'POST',
            headers: getAuthHeaders()
        });
        const data = await res.json();
        
        if (res.ok) {
            showToast(`✅ ${data.message} (+${data.payout} pts)`);
            fetchUserStats();
            fetchPortfolio();
        } else {
            alert(`❌ Failed: ${data.error}`);
        }
    } catch (err) {
        console.error(err);
    }
}

// ============================================================
// ADMIN DASHBOARD
// ============================================================
async function fetchAdminMarkets() {
    try {
        const res = await fetch(`${API_URL}/markets`);
        const markets = await res.json();
        const tbody = document.querySelector('#adminResolveTable tbody');
        if (!tbody) return;

        tbody.innerHTML = '';
        markets.forEach(m => {
            if (m.resolution_status === 'Open') {
                tbody.innerHTML += `
                <tr>
                    <td>${m.id}</td>
                    <td>${m.title}</td>
                    <td><span class="badge" style="background:var(--bg-hover);">${m.resolution_status}</span></td>
                    <td>
                        <button onclick="resolveMarket(${m.id}, 'Yes')" class="auth-submit" style="padding:0.3rem 0.6rem; font-size:0.8rem; background:var(--color-yes);">Resolve Yes</button>
                        <button onclick="resolveMarket(${m.id}, 'No')" class="auth-submit" style="padding:0.3rem 0.6rem; font-size:0.8rem; background:var(--color-no); margin-left:5px;">Resolve No</button>
                    </td>
                </tr>`;
            }
        });
    } catch (err) {
        console.error(err);
    }
}

async function resolveMarket(id, outcome) {
    if (!confirm(`Are you sure you want to resolve Market #${id} to ${outcome}?`)) return;
    try {
        const res = await fetch(`${API_URL}/markets/${id}/resolve`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ outcome })
        });
        const data = await res.json();
        if (res.ok) {
            showToast(`✅ Market resolved to ${outcome}!`);
            fetchAdminMarkets();
        } else {
            alert(`❌ Failed: ${data.error}`);
        }
    } catch (err) {
        console.error(err);
    }
}

const adminCreateForm = document.getElementById('adminCreateForm');
if (adminCreateForm) {
    adminCreateForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        try {
            const res = await fetch(`${API_URL}/markets`, {
                method: 'POST',
                headers: getAuthHeaders(),
                body: JSON.stringify({
                    title: document.getElementById('adminTitle').value,
                    description: document.getElementById('adminDesc').value,
                    category: document.getElementById('adminCategory').value,
                    end_date: document.getElementById('adminDate').value + 'T23:59:59Z'
                })
            });
            const data = await res.json();
            if (res.ok) {
                showToast(`✅ Market created successfully!`);
                adminCreateForm.reset();
                fetchAdminMarkets();
            } else {
                alert(`❌ Failed: ${data.error}`);
            }
        } catch (err) {
            console.error(err);
        }
    });
}


function openProposeModal() {
    if (!authToken) return alert("Please log in first.");
    document.getElementById('proposeOverlay').style.display = 'flex';
}

async function handleProposeSubmit(e) {
    e.preventDefault();
    const payload = {
        title: document.getElementById('proposeTitle').value,
        description: document.getElementById('proposeDesc').value,
        category: document.getElementById('proposeCategory').value,
        end_date: document.getElementById('proposeDate').value + 'T23:59:59Z'
    };

    try {
        const res = await fetch(`${API_URL}/markets/propose`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(payload)
        });
        const data = await res.json();
        if (res.ok) {
            showToast("✅ Proposal submitted for review!");
            document.getElementById('proposeOverlay').style.display = 'none';
            document.getElementById('proposeForm').reset();
        } else {
            alert(data.error);
        }
    } catch (err) {
        console.error(err);
    }
}

function switchAdminTab(tab) {
    document.querySelectorAll('.admin-tab-content').forEach(c => c.style.display = 'none');
    document.querySelectorAll('.admin-dashboard .auth-tab').forEach(t => t.classList.remove('active'));
    
    if (tab === 'markets') {
        document.getElementById('admin-markets-tab').style.display = 'block';
        fetchAdminMarkets();
    } else if (tab === 'proposals') {
        document.getElementById('admin-proposals-tab').style.display = 'block';
        fetchProposedMarkets();
    }
}

async function fetchProposedMarkets() {
    try {
        const res = await fetch(`${API_URL}/markets/proposed`, { headers: getAuthHeaders() });
        const markets = await res.json();
        const tbody = document.querySelector('#adminProposalsTable tbody');
        if (!tbody) return;

        tbody.innerHTML = markets.map(m => `
            <tr>
                <td>${m.title}</td>
                <td>${m.description}</td>
                <td>${m.category}</td>
                <td>
                    <button onclick="approveMarket(${m.id})" class="auth-submit" style="padding:0.3rem 0.6rem; font-size:0.8rem; background:var(--color-yes);">Approve</button>
                </td>
            </tr>
        `).join('');
    } catch (err) {
        console.error(err);
    }
}

async function approveMarket(id) {
    if (!confirm("Approve this market and make it live?")) return;
    try {
        const res = await fetch(`${API_URL}/markets/${id}/approve`, {
            method: 'POST',
            headers: getAuthHeaders()
        });
        if (res.ok) {
            showToast("✅ Market is now live!");
            fetchProposedMarkets();
        }
    } catch (err) {
        console.error(err);
    }
}


// ============================================================
// STARTUP
// ============================================================
(function init() {
    const authOverlay = document.getElementById('authOverlay');

    if (!authToken || !currentUser) {
        // Show auth modal if not logged in
        authOverlay.style.display = 'flex';
    } else {
        authOverlay.style.display = 'none';
        updateTopbarFromUser(currentUser);
        fetchMarkets();
        fetchUserStats(); // refresh balance from server
        initWebSocket();  // Start live updates
    }
})();
