import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';
import { escapeHTML } from '../utils/escape.js';
import { askText, confirmAction } from '../components/dialog.js';

/**
 * PROPHIT - Admin Panel Logic
 */
let currentAdmin = null;

document.addEventListener('DOMContentLoaded', async () => {
    if (!ApiClient || !ApiClient.isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    // Verify Admin Role before rendering
    try {
        currentAdmin = await ApiClient.get('/me');
        if (currentAdmin.role !== 'admin' && currentAdmin.role !== 'super_admin') {
            window.location.href = 'dashboard.html';
            return;
        }
    } catch(err) {
        document.querySelector('.content-wrapper').innerHTML = '<div class="card text-danger" role="alert">Admin data could not load. Check the local API and refresh to retry.</div>';
        return;
    }

    setupMarketEditor();
    // Initial load is routed through the selected hash tab so each endpoint is
    // requested once.
    const openHash = () => { const name = window.location.hash.slice(1) || 'markets'; const button = document.querySelector('[data-tab="' + (['markets','kyc','withdrawals','analytics','moderation'].includes(name) ? name : 'markets') + '"]'); switchTab(button.dataset.tab, button); };
    window.addEventListener('hashchange', openHash); openHash();
    let searchTimer;
    document.getElementById('adminUserSearch').addEventListener('input', () => { clearTimeout(searchTimer); searchTimer = setTimeout(() => { usersPage = 0; fetchAdminUsers(); }, 300); });
});

function switchTab(tabName, element) {
    // UI Update
    document.querySelectorAll('.admin-tab').forEach(el => el.classList.remove('active'));
    element.classList.add('active');

    document.querySelectorAll('.admin-tab-content').forEach(el => el.classList.add('hidden'));
    document.getElementById(`tab-${tabName}`).classList.remove('hidden');
    if (window.location.hash !== `#${tabName}`) history.replaceState(null, '', `#${tabName}`);

    // Data Fetching
    if (tabName === 'markets') {
        fetchNewsControl();
        fetchProposedMarkets();
        fetchActiveMarkets();
    }
    if (tabName === 'analytics') fetchAnalytics();
    if (tabName === 'moderation') { fetchAdminUsers(); fetchReports(); }
    if (tabName === 'kyc') fetchAdminKyc();
    if (tabName === 'withdrawals') fetchWithdrawals();
}

async function fetchProposedMarkets() {
    const tbody = document.querySelector('#proposalsTable tbody');
    try {
        const markets = await ApiClient.get('/markets/proposed');
        if (!markets || markets.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No pending market proposals.</td></tr>';
            return;
        }

        tbody.innerHTML = markets.map(m => `
            <tr>
                <td class="font-semibold text-primary">${escapeHTML(m.title)}</td>
                <td><span class="badge badge-outline">${escapeHTML(m.category)}</span></td>
                <td>ID: ${m.creator_id}</td>
                <td>
                    <button class="btn btn-outline" onclick="editMarket(${m.id})">Edit draft</button><a class="btn btn-outline" href="market.html?id=${m.id}&preview=1">Preview</a><button class="btn btn-outline" onclick="reviewRules(${m.id})">Review rules</button>${m.category==='Weather'&&m.difficulty==='Easy'?`<button class="btn btn-outline" onclick="configureWeatherResult(${m.id})">Configure weather result</button>`:''}<button class="btn btn-yes" style="padding: 0.25rem 0.75rem; font-size: 0.8rem;" onclick="approveMarket(${m.id})">Publish reviewed rules</button>
                </td>
            </tr>
        `).join('');
    } catch (err) {
        tbody.innerHTML = '<tr><td colspan="4" class="text-center text-danger">Failed to load proposals.</td></tr>';
    }
}

let activeOffset=0, activeRequestVersion=0;
async function fetchActiveMarkets(append=false) {
    const tbody = document.querySelector('#activeMarketsTable tbody');
    if (!tbody) return;
    const more=document.getElementById('activeMarketsMore'), version=++activeRequestVersion;
    if(!append)activeOffset=0;more.disabled=true;
    try {
        const markets = await ApiClient.get(`/admin/markets?offset=${activeOffset}`);
        if(version!==activeRequestVersion)return;
        more.hidden=markets.length<100;
        activeOffset+=markets.length;
        if (!markets || markets.length === 0) {
            if(!append)tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No active markets.</td></tr>';
            return;
        }

        // Filter for Open/Closed (not Proposed)
        const activeOrClosed = markets.filter(m => m.resolution_status !== 'Proposed');

        if (activeOrClosed.length === 0) {
            if(!append)tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No active markets.</td></tr>';
            return;
        }

        const rows = activeOrClosed.map(m => {
            let statusBadge = m.resolution_status === 'Open' ? `<span class="badge badge-primary">Open</span>` : `<span class="badge badge-success">${escapeHTML(m.resolution_status)}</span>`;
            
            // If the market is open or closed but not resolved, we can resolve it
            let resolveBtn = '';
            if (['Locked', 'Awaiting Resolution'].includes(m.resolution_status)) {
                resolveBtn = `<button class="btn btn-yes" style="padding: 0.25rem 0.75rem; font-size: 0.8rem;" onclick="resolveMarket(${m.id})">Resolve</button>`;
            }

            return `
            <tr>
                <td class="font-semibold text-primary">${escapeHTML(m.title)}</td>
                <td><span class="badge badge-outline">${escapeHTML(m.category)}</span></td>
                <td>${statusBadge}</td>
                <td>
                    <a class="btn btn-outline" href="market.html?id=${m.id}&preview=1">Preview</a> ${['Live','Open','Scheduled'].includes(m.resolution_status)?`<button class="btn btn-outline" onclick="pauseMarket(${m.id},false)">Pause</button>`:''} ${m.resolution_status==='Paused'?`<button class="btn btn-outline" onclick="pauseMarket(${m.id},true)">Resume</button>`:''} ${m.resolution_status==='Live'?`<button class="btn btn-outline" onclick="featureMarket(${m.id},${!m.is_featured})">${m.is_featured?'Unfeature':'Feature'}</button>`:''} ${!['Resolved','Voided','Archived'].includes(m.resolution_status)?`<button class="btn btn-outline" onclick="voidMarket(${m.id})">Void & refund</button>`:''} ${['Live','Open','Paused'].includes(m.resolution_status)?`<button class="btn btn-outline" onclick="lockMarket(${m.id})">Lock</button>`:''} ${resolveBtn}
                </td>
            </tr>
            `;
        }).join('');
        if(append)tbody.insertAdjacentHTML('beforeend',rows);else tbody.innerHTML=rows;
    } catch (err) {
        if(version!==activeRequestVersion)return;
        if(append){showToast('Could not load more markets. Retry.','error');return;}
        tbody.innerHTML = '<tr><td colspan="4" class="text-center text-danger">Failed to load active markets.</td></tr>';
    } finally { if(version===activeRequestVersion)more.disabled=false; }
}

async function approveMarket(id) {
    const review=await editorialReview(); if(!review)return;
    try { await ApiClient.post(`/markets/${id}/approve`, review); showToast('Prediction published.','success'); fetchProposedMarkets();fetchActiveMarkets(); }
    catch(err){showToast(err.message,'error');}
}
function editorialReview() {
    return new Promise(resolve=>{
        const dialog=document.createElement('dialog');dialog.className='modal-content';
        const checks=[['current','Current event and real context verified'],['future_outcome','Outcome is still in the future'],['objective','Resolution rule is objective and measurable'],['trusted_source','Trusted source and evidence checked'],['sensible_cutoff','Cutoff leaves no outcome known before entry'],['interesting','Question is worth predicting now']];
        dialog.innerHTML=`<form style="display:grid;gap:12px"><h2>Publication review</h2>${checks.map(([name,label])=>`<label><input type="checkbox" name="${name}" required> ${label}</label>`).join('')}<label>Why does this matter now?<textarea class="input-control" name="rationale" minlength="30" maxlength="2000" required></textarea></label><button type="submit" class="btn btn-primary">Publish reviewed prediction</button><button type="button" class="btn btn-outline" data-cancel>Cancel</button></form>`;
        const close=value=>{dialog.close();dialog.remove();resolve(value);};
        dialog.querySelector('[data-cancel]').onclick=()=>close(null);dialog.addEventListener('cancel',event=>{event.preventDefault();close(null);});
        dialog.querySelector('form').onsubmit=event=>{event.preventDefault();const fields=new FormData(event.currentTarget);const review={rationale:fields.get('rationale')};for(const [name] of checks)review[name]=fields.has(name);close(review);};
        document.body.append(dialog);dialog.showModal();
    });
}

async function resolveMarket(id) {
    const outcome = await askText("Enter the measured outcome in the published format (option, number, score array or three nominees):");
    if (!outcome) return;

    if (!await confirmAction(`Are you sure you want to resolve Market ${id} with winner: ${outcome}? This will trigger payouts and cannot be undone.`)) return;

    try {
        const evidence_url=await askText("Approved result-source evidence URL:");if(!evidence_url)return;const observed=await askText("Observation time (ISO date/time with timezone):",new Date().toISOString());if(!observed || Number.isNaN(Date.parse(observed)))throw new Error("Enter a valid observation timestamp");
        const res = await ApiClient.post(`/markets/${id}/resolve`, { winner: outcome,evidence_url,observed_at:new Date(observed).toISOString() });
        showToast(`Market resolved! ${res.winners_paid} winners paid.`, "success");
        fetchActiveMarkets();
    } catch (err) {
        showToast(err.message, "error");
    }
}

async function fetchAdminKyc() {
    const tbody = document.querySelector('#kycTable tbody');
    try {
        const res = await ApiClient.get('/admin/kyc');
        const reqs = Array.isArray(res) ? res : (res?.items || []);
        if (!reqs || reqs.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="text-center text-muted">No KYC verification attempts found.</td></tr>';
            return;
        }

        tbody.innerHTML = reqs.map(r => {
            let badge = '';
            if (r.status === 'Verified') badge = `<span class="badge badge-success">Verified</span>`;
            else if (r.status === 'Rejected') badge = `<span class="badge badge-danger">Rejected</span>`;
            else badge = `<span class="badge badge-warning">${escapeHTML(r.status)}</span>`;

            return `
                <tr>
                    <td><div class="font-semibold">${escapeHTML(r.username)}</div><div class="text-muted" style="font-size: 0.75rem;">ID: ${r.user_id}</div></td>
                    <td>${badge}</td>
                    <td class="font-mono text-muted" style="font-size: 0.85rem;">${escapeHTML(r.provider_reference)}</td>
                    <td class="text-muted" style="font-size: 0.85rem;">${new Date(r.created_at).toLocaleString()}</td>
                    <td class="text-danger" style="font-size: 0.85rem;">${escapeHTML(r.failure_reason || '--')}</td>
                </tr>
            `;
        }).join('');
    } catch (err) {
        tbody.innerHTML = '<tr><td colspan="5" class="text-center text-danger">Failed to load KYC logs.</td></tr>';
    }
}

async function fetchWithdrawals() {
    const tbody = document.querySelector('#withdrawalsTable tbody');
    try {
        const reqs = await ApiClient.get('/admin/withdrawals');
        if (!reqs || reqs.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No pending withdrawals.</td></tr>';
            return;
        }

        tbody.innerHTML = reqs.map(w => `
            <tr>
                <td>User ID: ${w.user_id}</td>
                <td class="font-bold text-gold">INR ${w.amount}</td>
                <td><span class="badge badge-warning">${escapeHTML(w.status)}</span></td>
                <td>
                    ${w.status==='Pending'?`<button class="btn btn-outline" onclick="processWithdrawal(${w.id}, 'Approve')">Start processing</button><button class="btn btn-outline" onclick="processWithdrawal(${w.id}, 'Reject')">Reject and refund valid coins</button>`:''}
                    ${w.status==='Processing'?`<button class="btn btn-primary" onclick="fulfillVoucher(${w.id})">Supply sourced voucher</button>`:''}
                    ${w.status==='Fulfilled'?`<button class="btn btn-primary" onclick="deliverVoucher(${w.id})">Send voucher email</button>`:''}
                    ${w.status==='Failed'?'<p>Reconcile delivery attempt before retrying.</p>':''}
                </td>
            </tr>
        `).join('');
    } catch (err) {
        tbody.innerHTML = '<tr><td colspan="4" class="text-center text-danger">Failed to load withdrawals.</td></tr>';
    }
}

async function processWithdrawal(id, action) {
    if (!await confirmAction(`${action} this withdrawal?`)) return;
    try {
        await ApiClient.post(`/admin/withdrawals/${id}/${action.toLowerCase()}`);
        showToast(`Withdrawal ${action.toLowerCase()}d successfully`, "success");
        fetchWithdrawals();
    } catch (err) {
        showToast(err.message, "error");
    }
}

window.switchTab = switchTab;
window.fetchProposedMarkets = fetchProposedMarkets;
window.fetchActiveMarkets = fetchActiveMarkets;
window.approveMarket = approveMarket;
window.resolveMarket = resolveMarket;
window.fetchAdminKyc = fetchAdminKyc;
window.fetchWithdrawals = fetchWithdrawals;
window.processWithdrawal = processWithdrawal;

async function fetchAnalytics() {
    try {
        const stats = await ApiClient.get('/admin/stats');
        document.getElementById('adminTotalUsers').textContent = stats.users.total;
        document.getElementById('adminTotalPredictions').textContent = stats.trades.total;
        const health = await ApiClient.get('/health');
        document.getElementById('adminHealth').textContent = health.message;
    } catch (error) {
        document.getElementById('adminHealth').textContent = error.message;
    }
}

let usersRequest = 0, usersPage = 0;
window.changeUsersPage = dir => { usersPage = Math.max(0, usersPage + dir); fetchAdminUsers(); };
async function fetchAdminUsers() {
    const request = ++usersRequest;
    const list = document.getElementById('adminUsers');
    document.getElementById('usersPrev').disabled=true;
    document.getElementById('usersNext').disabled=true;
    list.textContent='Loading users...';
    try {
        const response = await ApiClient.get('/admin/users?limit=100&offset=' + (usersPage * 100) + '&search=' + encodeURIComponent(document.getElementById('adminUserSearch').value));
        if (request !== usersRequest) return;
        document.getElementById("usersPrev").disabled = usersPage === 0;
        document.getElementById("usersNext").disabled = response.users.length < 100;
        list.replaceChildren();
        if (!response.users.length) list.textContent = 'No matching users.';
        for (const user of response.users) {
            const row = document.createElement('div');
            row.className = 'flex justify-between items-center mb-2';
            const name = document.createElement('span');
            name.textContent = `${user.username} · ${user.role} · ${user.is_active ? 'Active' : 'Banned'}`;
            const button = document.createElement('button');
            button.className = 'btn btn-outline';
            button.textContent = user.is_active ? 'Ban' : 'Unban';
            const protectedTarget = user.id === currentAdmin?.id || user.role === 'super_admin' ||
                (user.role === 'admin' && currentAdmin?.role !== 'super_admin');
            if (protectedTarget) {
                button.disabled = true;
                button.title = user.id === currentAdmin?.id ? 'You cannot moderate your own account' : 'This account cannot be moderated by your role';
            }
            button.addEventListener('click', async () => {
                if (!await confirmAction(`${button.textContent} ${user.username}?`)) return;
                try { await ApiClient.post(`/admin/users/${user.id}/${user.is_active ? 'ban' : 'unban'}`); await fetchAdminUsers(); }
                catch (error) { showToast(error.message, 'error'); }
            });
            row.append(name, button); list.append(row);
        }
    } catch (error) { if (request === usersRequest) list.textContent = error.message; }
}

async function fetchReports() {
    const list = document.getElementById('adminReports');
    try {
        const reports = await ApiClient.get('/admin/reports?status=Pending&limit=100');
        list.replaceChildren();
        if (!reports.length) list.textContent = 'No pending reports.';
        for (const report of reports) {
            const row = document.createElement('article');
            const text = document.createElement('p');
            text.textContent = `${report.target_type} #${report.target_id}: ${report.reason} — ${report.description}`;
            const button = document.createElement('button');
            button.className = 'btn btn-outline'; button.textContent = 'Review report';
            button.addEventListener('click', async () => {
                const actions = report.target_type === 'User' ? 'Dismiss, Mute, Suspend, Ban' : report.target_type === 'Comment' ? 'Dismiss, DeleteComment' : 'Dismiss';
                const action = await askText(`Choose an action: ${actions}`);
                if (!action || !await confirmAction(`Apply ${action} to report #${report.id}? Suspensions last 7 days.`)) return;
                try { await ApiClient.post(`/admin/reports/${report.id}/resolve`, {action, duration_days: 7}); await fetchReports(); }
                catch (error) { showToast(error.message, 'error'); }
            });
            row.append(text, button); list.append(row);
        }
    } catch (error) { list.textContent = error.message; }
}
window.fetchAdminUsers = fetchAdminUsers;

window.reviewRules=async id=>{try{const rows=await ApiClient.get('/markets/proposed');const market=rows.find(m=>Number(m.id)===Number(id));const edited=await askText('Review source event, title, category, difficulty, options, measurable resolution_rule, approved resolution_source, range_width and future lock_time:',JSON.stringify(market,null,2));if(!edited)return;await ApiClient.put(`/markets/${Number(id)}/rules`,JSON.parse(edited));showToast('Rules saved for publication review.','success');fetchProposedMarkets();}catch(err){showToast(err.message,'error');}};

async function fetchNewsControl(){
 const status=document.getElementById('newsControlStatus'),events=document.getElementById('newsControlEvents');
 try{const data=await ApiClient.get('/admin/news'),state=data.ingestion;status.textContent=`Provider: ${state.status} · Last success: ${state.last_success_at?new Date(state.last_success_at).toLocaleString():'None'} · Fetched: ${state.articles_fetched} · Duplicates: ${state.duplicates_removed} · Events: ${state.events_detected} · Rejected: ${state.articles_rejected} · Drafts: ${data.markets.Draft} · Live: ${data.markets.Live} · Resolved: ${data.markets.Resolved}${state.error?' · '+state.error:''}`;events.replaceChildren();for(const event of data.events){const entry=document.createElement('details'),summary=document.createElement('summary');summary.textContent=`${event.title} · ${event.category} · ${event.status}`;entry.append(summary);for(const source of event.sources||[]){const line=document.createElement('p'),link=document.createElement('a');link.href=source.url;link.target='_blank';link.rel='noopener noreferrer';link.textContent=`${source.name||source.provider} · ${new Date(source.published_at).toLocaleString()}`;line.append(link);entry.append(line);}const generate=document.createElement('button');generate.type='button';generate.className='btn btn-outline';generate.textContent='Generate draft question';generate.addEventListener('click',async()=>{generate.disabled=true;try{await ApiClient.post(`/admin/news/${Number(event.id)}/generate`);await fetchNewsControl();fetchProposedMarkets();}catch(err){showToast(err.message,'error');}finally{generate.disabled=false;}});entry.append(generate);if(event.rejection_reason){const reason=document.createElement('p');reason.textContent=event.rejection_reason;entry.append(reason);}events.append(entry);}}
 catch(err){status.textContent=`News control unavailable: ${err.message}`;events.replaceChildren();}
}
document.getElementById('refreshNews')?.addEventListener('click',async e=>{e.currentTarget.disabled=true;try{await ApiClient.post('/admin/news/refresh');await fetchNewsControl();fetchProposedMarkets();}catch(err){showToast(err.message,'error');}finally{e.target.disabled=false;}});

window.configureWeatherResult=async id=>{try{const text=await askText('Enter the actual location, metric and threshold from the reviewed question. Observation must start after the cutoff and last exactly 15 minutes. All fields must be reviewed:',JSON.stringify({provider:'openweather',latitude:null,longitude:null,metric:'temperature_c',threshold:null,observation_from:'',observation_until:''},null,2));if(!text)return;const spec=JSON.parse(text);if(![spec.latitude,spec.longitude,spec.threshold].every(Number.isFinite))throw new Error('Fill in actual coordinates and threshold.');await ApiClient.put(`/markets/${Number(id)}/result-provider`,spec);showToast('Result rule saved. Review the question and rule before publication.','success');fetchProposedMarkets();}catch(err){showToast(err.message,'error');}};
window.fulfillVoucher=async id=>{const voucher_code=await askText('Actual officially sourced voucher code:');if(!voucher_code)return;const source_reference=await askText('Official supplier and invoice/order reference:');if(!source_reference)return;try{await ApiClient.post(`/admin/withdrawals/${Number(id)}/fulfill`,{voucher_code,source_reference});showToast('Voucher fulfilled. Email delivery is separate.','success');fetchWithdrawals();}catch(err){showToast(err.message,'error');}};
window.deliverVoucher=async id=>{if(!await confirmAction('Send this sourced voucher to the account email now?'))return;try{const result=await ApiClient.post(`/admin/withdrawals/${Number(id)}/deliver`);showToast(result.message,'success');}catch(err){showToast(err.message,'error');}finally{fetchWithdrawals();}};

function setupMarketEditor() {
    const form=document.getElementById('adminMarketForm');
    form.addEventListener('submit',async event=>{
        event.preventDefault();
        const fields=new FormData(form), button=form.querySelector('[type="submit"]');
        const market={title:fields.get('title'), description:fields.get('description'), category:fields.get('category'), difficulty:fields.get('difficulty'), options:JSON.stringify(String(fields.get('options')).split('\n').map(s=>s.trim()).filter(Boolean)), entry_coins:Number(fields.get('entry_coins')), range_width:((['Weather','Sports'].includes(fields.get('category'))&&fields.get('difficulty')==='Medium')||(fields.get('category')==='Entertainment'&&fields.get('difficulty')==='Hard'))?Number(fields.get('range_width')):0, resolution_rule:fields.get('resolution_rule'), resolution_source:fields.get('resolution_source'), lock_time:new Date(fields.get('lock_time')).toISOString(), resolution_time:new Date(fields.get('resolution_time')).toISOString(), news_url:fields.get('news_url'), news_source_name:fields.get('news_source_name'), source_kind:fields.get('source_kind'), news_published_at:fields.get('news_published_at')?new Date(fields.get('news_published_at')).toISOString():null, resolution_status:'Draft'};
        if(market.difficulty==='Easy' && market.options===JSON.stringify(['Yes','No']))market.prediction_type='binary';
        if(fields.get('start_time'))market.start_time=new Date(fields.get('start_time')).toISOString();
        const id=form.dataset.marketId;
        button.disabled=true;
        try { if(id)await ApiClient.put(`/markets/${id}/rules`,market);else await ApiClient.post('/markets',market); form.reset();delete form.dataset.marketId;showToast('Draft saved. Review and publish below.','success');fetchProposedMarkets(); }
        catch(error){showToast(error.message,'error');}
        finally {button.disabled=false;}
    });
}
async function editMarket(id) {
    try {
        const markets=await ApiClient.get('/markets/proposed'), m=markets.find(item=>Number(item.id)===Number(id));
        if(!m)throw new Error('This market is no longer editable.');
        const form=document.getElementById('adminMarketForm');
        form.dataset.marketId=String(id);
        for(const name of ['title','description','category','difficulty','entry_coins','resolution_rule','resolution_source','news_url','news_source_name','source_kind'])form.elements[name].value=m[name]??'';
        form.elements.range_width.value=m.range_width||5;
        form.elements.options.value=JSON.parse(m.options).join('\n');
        for(const name of ['start_time','lock_time','resolution_time','news_published_at']) {
            const date=m[name]?new Date(m[name]):null;
            form.elements[name].value=date?new Date(date.getTime()-date.getTimezoneOffset()*60000).toISOString().slice(0,16):'';
        }
        document.getElementById('adminMarketEditor').open=true;form.scrollIntoView({behavior:'smooth'});
    }catch(error){showToast(error.message,'error');}
}
async function lockMarket(id) {
    if(!await confirmAction('Lock this market and stop new predictions?'))return;
    try{await ApiClient.put(`/markets/${id}/transition`,{status:'Locked'});fetchActiveMarkets();showToast('Market locked.','success');}catch(error){showToast(error.message,'error');}
}
window.editMarket=editMarket;window.lockMarket=lockMarket;

window.pauseMarket=async(id,resume)=>{try{await ApiClient.put(`/markets/${id}/transition`,{status:resume?'Live':'Paused'});fetchActiveMarkets();showToast(resume?'Predictions resumed.':'Predictions paused.','success');}catch(err){showToast(err.message,'error');}};
window.featureMarket=async(id,featured)=>{try{await ApiClient.put(`/admin/markets/${id}/feature`,{featured});fetchActiveMarkets();}catch(err){showToast(err.message,'error');}};
window.voidMarket=async id=>{const reason=await askText('Why should this prediction be cancelled? Entries will be refunded.');if(!reason)return;try{await ApiClient.post(`/admin/markets/${id}/void`,{reason});fetchActiveMarkets();showToast('Cancelled. Entry Coins refunded.','success');}catch(err){showToast(err.message,'error');}};
