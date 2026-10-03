import '../components/sidebar.js';
import '../components/topbar.js';
import ApiClient from '../api/client.js';
import {escapeHTML} from '../utils/escape.js';
import {showToast} from '../components/toast.js';
if(!ApiClient.isAuthenticated())location.href='login.html';
async function load(){
 const list=document.getElementById('challengesList');
 try{
  const rows=await ApiClient.get('/challenges');
  list.innerHTML=rows?.length?rows.map(c=>`<article class="card"><h2>${escapeHTML(c.title)}</h2><p>${escapeHTML(c.status)} · closes ${new Date(c.end_date).toLocaleString()} · correct answer: ${Number(c.reward_pool)} coins</p>${c.market_id?`<a class="btn btn-primary" href="market.html?id=${Number(c.market_id)}">View prediction</a>`:'<p>Historical challenge awaiting reconciliation.</p>'}<button class="btn btn-outline" data-results="${Number(c.id)}">Results and participants</button></article>`).join(''):'No weekly challenges yet.';
  list.querySelectorAll('[data-results]').forEach(b=>b.addEventListener('click',()=>results(b.dataset.results)));
 }catch(err){list.textContent=`Challenges unavailable: ${err.message}`;}
}
async function results(id){const target=document.getElementById('challengeResults');target.textContent='Loading results…';try{const data=await ApiClient.get(`/challenges/${Number(id)}`);target.innerHTML=`<article class="card"><h2>${escapeHTML(data.challenge.title)} — results</h2>${data.leaderboard?.length?data.leaderboard.map((u,i)=>`<p>${i+1}. ${escapeHTML(u.username)} · ${Number(u.score)} correct · ${Number(u.reward_won)} coins</p>`).join(''):'No participants yet.'}</article>`;}catch(err){target.textContent=err.message;}}
document.getElementById('refreshChallenges').addEventListener('click',load);
document.getElementById('createChallenge').addEventListener('submit',async e=>{e.preventDefault();const button=e.target.querySelector('button');button.disabled=true;try{await ApiClient.post('/admin/challenges',{market_id:Number(document.getElementById('challengeMarket').value)});showToast('Challenge created.','success');load();}catch(err){showToast(err.message,'error');}finally{button.disabled=false;}});
ApiClient.get('/me').then(u=>{if(['admin','super_admin'].includes(u.role))document.getElementById('challengeAdmin').classList.remove('hidden');}).catch(()=>{});
window.addEventListener('prophit-live-status',e=>{document.getElementById('liveStatus').textContent=`Live updates: ${e.detail}`;});
window.addEventListener('prophit-live',e=>{if(['market_resolved','market_locked','leaderboard_updated'].includes(e.detail.event))load();});
load();
