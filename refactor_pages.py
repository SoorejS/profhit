import os
import re
import shutil

pages = ['auth.js', 'dashboard.js', 'market.js', 'profile.js', 'wallet.js']

for page in pages:
    if os.path.exists(f'js/{page}'):
        with open(f'js/{page}', 'r', encoding='utf-8') as f:
            content = f.read()
        
        # Replace global window calls
        content = content.replace("window.ApiClient", "ApiClient")
        content = content.replace("window.showToast", "showToast")
        
        # We need to prepend imports
        imports = "import ApiClient from '../api/client.js';\nimport { showToast } from '../components/toast.js';\n\n"
        
        content = imports + content
        
        # Export functions that are attached to window for HTML onclick compatibility
        # Or attach them to window inside the module so the HTML inline onclicks still work!
        # Actually, if we use <script type="module">, inline onclicks won't work unless they are on window.
        # So we should explicitly attach them to window inside the module.
        # E.g. window.claimDailyReward = async () => ... (already there)
        # E.g. async function openWithdraw -> window.openWithdraw = openWithdraw
        
        # Let's just make sure functions called from HTML are on window
        if page == 'wallet.js':
            content += "\nwindow.openWithdraw = openWithdraw;\nwindow.deposit = deposit;\nwindow.startKYC = startKYC;\n"
        elif page == 'market.js':
            content += "\nwindow.submitPrediction = submitPrediction;\nwindow.postComment = postComment;\n"
        elif page == 'admin.js':
            content += "\nwindow.switchTab = switchTab;\nwindow.fetchProposedMarkets = fetchProposedMarkets;\nwindow.approveMarket = approveMarket;\nwindow.resolveMarket = resolveMarket;\nwindow.fetchKycRequests = fetchKycRequests;\nwindow.reviewKyc = reviewKyc;\nwindow.fetchWithdrawals = fetchWithdrawals;\nwindow.approveWithdrawal = approveWithdrawal;\nwindow.rejectWithdrawal = rejectWithdrawal;\n"
            
        with open(f'js/pages/{page}', 'w', encoding='utf-8') as f:
            f.write(content)
        
        os.remove(f'js/{page}')

if os.path.exists('admin.js'):
    with open('admin.js', 'r', encoding='utf-8') as f:
        content = f.read()
    content = content.replace("window.ApiClient", "ApiClient")
    content = content.replace("window.showToast", "showToast")
    imports = "import ApiClient from './api/client.js';\nimport { showToast } from './components/toast.js';\n\n"
    content = imports + content
    content += "\nwindow.switchTab = switchTab;\nwindow.fetchProposedMarkets = fetchProposedMarkets;\nwindow.approveMarket = approveMarket;\nwindow.resolveMarket = resolveMarket;\nwindow.fetchKycRequests = fetchKycRequests;\nwindow.reviewKyc = reviewKyc;\nwindow.fetchWithdrawals = fetchWithdrawals;\nwindow.approveWithdrawal = approveWithdrawal;\nwindow.rejectWithdrawal = rejectWithdrawal;\n"
    with open('js/pages/admin.js', 'w', encoding='utf-8') as f:
        f.write(content)
    os.remove('admin.js')

print("Page scripts refactored.")
