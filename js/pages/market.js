import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import { showToast } from '../components/toast.js';
import { escapeHTML, safeURL } from '../utils/escape.js';

/**
 * PROPHIT - Market Detail Logic
 */

let currentMarketId = null;
let currentSelection = null;
let currentPayout = 0;
let currentMarket;
const tradeMarkup=document.querySelector(".trade-card").innerHTML;

document.addEventListener('DOMContentLoaded', () => {

    const urlParams = new URLSearchParams(window.location.search);
    currentMarketId = urlParams.get('id');

    if (!currentMarketId) {
        window.location.href = 'dashboard.html';
        return;
    }

    loadMarketDetails();
    if (urlParams.has('preview')) document.getElementById('commentsList').textContent='Discussion opens after publication.';
    else loadComments();
});

async function loadMarketDetails() {
    try {
        const market = await ApiClient.get(new URLSearchParams(location.search).has('preview') ? `/admin/markets/${encodeURIComponent(currentMarketId)}/preview` : `/markets/${encodeURIComponent(currentMarketId)}`);
        currentMarket=market;
        currentSelection = null;
        document.querySelector(".trade-card").innerHTML=tradeMarkup;
        currentPayout = Number(market.payout) * (market.weekly_challenge_id ? 2 : 1);
        document.getElementById('entryCost').textContent = `${Number(market.entry_coins)||0} Coins`;
        document.getElementById('marketFixedPayout').textContent = `${currentPayout} Coins`;
        document.getElementById('marketPredictionCount').textContent = participation(market);
        const options = JSON.parse(market.options);
        const optionsContainer = document.querySelector('.prediction-buttons');
        if (optionsContainer) {
            optionsContainer.replaceChildren();
            (['binary','winner','direction','multi_choice',''].includes(market.prediction_type||'') ? options : []).forEach(option => {
                optionsContainer.append(createPredictionButton(option));
            });
        }
        renderTypedAnswer(market);
        document.getElementById('potentialReturn').textContent = `${currentPayout} coins${market.weekly_challenge_id?' (weekly challenge: 2×)':''}`;

        
        document.getElementById('marketTitle').textContent = market.title;
        document.getElementById('marketDesc').textContent = market.description;
        const source = document.getElementById('marketSource');
        source.innerHTML = `Source: <a href="${escapeHTML(safeURL(market.news_url || market.resolution_source))}" target="_blank" rel="noopener noreferrer">${escapeHTML(market.news_source_name || 'Official source')}</a>`;
        document.getElementById('resolutionRules').textContent = market.resolution_rule || 'Resolution rules are being reviewed.';
        if (market.evidence_url) {
            const evidence = document.createElement('a'); evidence.href = safeURL(market.evidence_url); evidence.target = '_blank'; evidence.rel = 'noopener noreferrer'; evidence.textContent = 'View result evidence'; source.append(' · ', evidence);
        }
        document.getElementById('marketResolutionDate').textContent = displayDate(market.resolution_time);
        document.getElementById('backToMarkets').href = ApiClient.isAuthenticated() ? 'dashboard.html?view=markets' : 'index.html#liveFeed';
        document.getElementById('marketCategory').textContent = market.category;
        
        document.getElementById('marketCloseDate').textContent = displayDate(market.lock_time || market.end_date);

        const isClosed = new Date(market.lock_time || market.end_date) <= new Date();
        const notStarted = market.start_time && new Date(market.start_time) > new Date();
        const statusEl = document.getElementById('marketStatus');
        
        if (['Resolved', 'Archived'].includes(market.resolution_status) && market.correct_option) {
            statusEl.textContent = `Resolved: ${market.correct_option}`;
            statusEl.className = 'badge badge-success';
            document.querySelector('.trade-card').innerHTML = `<h3 class="text-success text-center">Market Resolved: ${escapeHTML(market.correct_option)}</h3>`;
        } else if (['Draft','Proposed'].includes(market.resolution_status)) {
            statusEl.textContent='Draft preview'; document.querySelector('.trade-card').innerHTML=tradeMarkup; document.getElementById('entryCost').textContent=`${market.entry_coins} Coins`; document.getElementById('marketFixedPayout').textContent=`${market.payout} Coins`; const choices=document.querySelector('.prediction-buttons'); choices.replaceChildren(); for(const option of options){choices.append(createPredictionButton(option, true));} document.getElementById('tradeForm').classList.add('hidden');
        } else if (['Paused','Voided'].includes(market.resolution_status)) {
            statusEl.textContent = market.resolution_status;
            document.querySelector('.trade-card').textContent = market.resolution_status === 'Voided' ? 'This prediction was cancelled. Entry Coins have been refunded.' : 'Predictions are temporarily paused.';
        } else if (market.resolution_status === 'Scheduled' || notStarted) {
            statusEl.textContent = 'Scheduled';
            document.querySelector('.trade-card').textContent = 'Predictions open when this market starts.';
        } else if (!['Open', 'Live'].includes(market.resolution_status) || isClosed) {
            statusEl.textContent = 'Resolving';
            statusEl.className = 'badge badge-warning';
            document.querySelector('.trade-card').innerHTML = `<h3 class="text-warning text-center">Market is closed. Awaiting resolution.</h3>`;
        } else {
            statusEl.textContent = market.resolution_status || 'Active';
            statusEl.className = 'badge badge-primary';
            if(!market.prediction_type || !market.resolution_rule || !market.resolution_source)document.querySelector('.trade-card').textContent='Predictions are currently unavailable for this market.';
        }

    } catch (err) {
        document.getElementById('marketCategory').textContent = 'Unavailable';
        document.getElementById('marketStatus').textContent = 'Error';
        document.getElementById('marketTitle').textContent = 'Market unavailable';
        document.getElementById('marketDesc').textContent = 'Could not load market details. Refresh to retry.';

        document.getElementById('resolutionRules').textContent = 'Resolution criteria are unavailable.';
        document.querySelector('.trade-card').textContent = 'Predictions are unavailable while market details cannot be loaded.';
        document.getElementById('marketPredictionCount').textContent = 'Unavailable';
        showToast('Failed to load market details.', 'error');
    }
}

function createPredictionButton(option, disabled = false) {
    const button = document.createElement('button');
    const label = String(option).trim().toLowerCase();
    const tone = label === 'yes' ? 'btn-yes' : label === 'no' ? 'btn-no' : 'btn-outline';
    button.className = `btn ${tone}`;
    button.type = 'button';
    button.textContent = option;
    button.dataset.option = option;
    button.disabled = disabled;
    button.setAttribute('aria-pressed', 'false');
    if (!disabled) button.addEventListener('click', () => selectPrediction(option));
    return button;
}

function selectPrediction(outcome) {
    currentSelection = outcome;
    document.getElementById('tradeForm').classList.remove('hidden');
    
    // Update button styles
    document.querySelectorAll('[data-option]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.option === outcome)));
}

async function executeTrade() {
    if (!ApiClient.isAuthenticated()) { window.location.href='login.html'; return; }
    if(!currentMarket) return;
    const choice=readTypedAnswer();
    if(!choice){showToast('Enter your prediction.','error');return;}
    const button=document.getElementById('confirmPrediction');button.disabled=true;
    try {
        const result=await ApiClient.post('/predictions',{market_id:Number(currentMarketId),choice,amount:Number(currentMarket.entry_coins)||0});
        showToast(`Prediction recorded. ${result.staked} virtual coins spent. Correct-answer reward: ${result.potential_payout} coins.`,'success');
        document.getElementById('tradeForm').classList.add('hidden');currentSelection=null;
        document.querySelectorAll('[data-option]').forEach(option => option.setAttribute('aria-pressed', 'false'));
        document.querySelector('app-topbar')?.fetchBalance();document.querySelector('app-sidebar')?.fetchBalance();
        await refreshActivity();
    }catch(err){showToast(err.message,'error');}finally{button.disabled=false;}
}
function renderTypedAnswer(m){
 const area=document.getElementById('typedAnswer');
 if(['binary','winner','direction','multi_choice',''].includes(m.prediction_type||''))return;
 document.getElementById('tradeForm').classList.remove('hidden');
 if(m.prediction_type==='range'){
  area.innerHTML='<label for="answerMin">Range minimum</label><input id="answerMin" type="number" step="any" class="input-control"><label for="answerMax">Range maximum</label><input id="answerMax" type="number" step="any" class="input-control">';
 }else if(m.prediction_type==='top3'){
  area.textContent='Select exactly three nominees';
  for(const option of JSON.parse(m.options)){const label=document.createElement('label');label.style.display='block';const input=document.createElement('input');input.type='checkbox';input.name='nominee';input.value=option;label.append(input,document.createTextNode(' '+option));area.append(label);}
 }else{
  const label=document.createElement('label');label.htmlFor='numericAnswer';label.textContent=m.prediction_type==='percent_range'?'Your percentage-change guess (within ±1 percentage point wins)':m.prediction_type==='score'?'Run total or score [home,away], following the published rule':'Your numerical answer in the published units';
  const input=document.createElement('input');input.id='numericAnswer';input.className='input-control';input.type=m.prediction_type==='score'?'text':'number';input.step='any';area.append(label,input);
 }
}
function readTypedAnswer(){
 if(currentMarket.prediction_type==='range'){const min=document.getElementById('answerMin').value,max=document.getElementById('answerMax').value;return min!==''&&max!==''?JSON.stringify([Number(min),Number(max)]):'';}
 if(currentMarket.prediction_type==='top3')return JSON.stringify([...document.querySelectorAll('[name="nominee"]:checked')].map(el=>el.value));
 return document.getElementById('numericAnswer')?.value.trim() || currentSelection;
}
async function refreshActivity(){try{const m=await ApiClient.get(new URLSearchParams(location.search).has('preview') ? `/admin/markets/${encodeURIComponent(currentMarketId)}/preview` : `/markets/${encodeURIComponent(currentMarketId)}`);document.getElementById('marketPredictionCount').textContent=participation(m);if(currentMarket&&m.resolution_status!==currentMarket.resolution_status)await loadMarketDetails();}catch{}}
window.addEventListener('prophit-live',e=>{const {event,payload}=e.detail;if(payload?.market_id && Number(payload.market_id)!==Number(currentMarketId))return;if(['prediction_count_changed','market_activity_changed','market_state_changed'].includes(event))refreshActivity();if(['market_locked','market_resolved'].includes(event))loadMarketDetails();});

async function loadComments() {
    try {
        const res = await ApiClient.get(`/markets/${encodeURIComponent(currentMarketId)}/comments`);
        const comments = Array.isArray(res) ? res : (res?.items || []);
        const list = document.getElementById('commentsList');
        
        if (!comments || comments.length === 0) {
            list.innerHTML = `<div class="text-muted text-center" style="padding: var(--spacing-4);">No comments yet. Be the first to share your thoughts!</div>`;
            return;
        }

        list.innerHTML = comments.map(c => `
            <div class="comment-item">
                <div class="flex justify-between items-center" style="margin-bottom: 4px;">
                    <div class="font-bold text-primary">@${escapeHTML(c.username || 'Unknown')}</div>
                    <div class="text-muted" style="font-size: 0.8rem;">${new Date(c.created_at).toLocaleString()}</div>
                </div>
                <div class="text-secondary" style="font-size: 0.95rem;">${escapeHTML(c.content)}</div>
            </div>
        `).join('');

    } catch (err) {
        document.getElementById('commentsList').textContent = 'Comments could not load. Refresh to retry.';
    }
}

async function postComment() {
    if (!ApiClient.isAuthenticated()) { window.location.href='login.html'; return; }
    const input = document.getElementById('commentInput');
    const content = input.value.trim();
    if (!content) return;

    try {
        await ApiClient.post(`/markets/${encodeURIComponent(currentMarketId)}/comments`, { content });
        input.value = '';
        showToast('Comment posted.', 'success');
        loadComments();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

function participation(m) { return Number(m.volume) > 0 ? `${Number(m.volume)} ${Number(m.volume) === 1 ? 'player' : 'players'}` : 'Be the first to predict'; }
function displayDate(raw) { return raw ? new Date(raw).toLocaleString(undefined, {month:'short', day:'numeric', hour:'numeric', minute:'2-digit', timeZoneName:'short'}) : 'To be announced'; }
window.selectPrediction = selectPrediction;
window.executeTrade = executeTrade;
window.postComment = postComment;
