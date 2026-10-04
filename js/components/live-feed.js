import ApiClient from '../api/client.js';

import '../services/live.js';

import { escapeHTML } from '../utils/escape.js';
import { categories, marketCard } from './market-card.js';

export function initLiveFeed(root){

 if(!root)return;

 root.innerHTML=`<div class="live-feed-heading"><div><h2>Predict what’s happening in the world.</h2><p>Virtual coins only · No real money</p></div><a href="propose-market.html" class="btn btn-outline">+ Create Prediction</a></div><p class="live-feed-status" role="status" aria-live="polite">Checking current opportunities…</p><div class="live-feed-controls"><div class="live-feed-tabs" aria-label="Feed sections">${[['all','Explore'],['latest','New'],['trending','Trending'],['closing_soon','Closing Soon'],['today','Today’s News'],['for_you','For You']].map(([key,label])=>`<button type="button" class="btn btn-outline" data-section="${key}" aria-pressed="${key==='all'}" ${key==='for_you'&&!ApiClient.isAuthenticated()?'disabled title="Sign in for recommendations"':''}>${label}</button>`).join('')}</div><form class="live-feed-search"><label>Search events, questions or sources<input name="search" class="input-control" type="search" maxlength="200" placeholder="Search current events…"></label><label>Category<select name="category" class="input-control"><option value="">All categories</option>${categories.map(c=>`<option>${escapeHTML(c)}</option>`).join('')}</select></label><button type="submit" class="btn btn-secondary">Search</button></form></div><div class="live-feed-items markets-grid" aria-busy="true"></div><button type="button" class="btn btn-outline live-feed-more" hidden>Load more</button>`;

 let section='all',offset=0,version=0,timer,signature='',loading=false;

 const items=root.querySelector('.live-feed-items'),status=root.querySelector('.live-feed-status'),more=root.querySelector('.live-feed-more'),form=root.querySelector('form'),params=new URLSearchParams(location.search);

 form.elements.search.value=params.get('search')||'';if(categories.includes(params.get('category')))form.elements.category.value=params.get('category');

 async function load(append=false,quiet=false){
  if(quiet && loading)return;
  const pageSize=quiet?Math.min(100,Math.max(24,offset)):24, requestOffset=append?offset:0, current=++version;
  loading=true; items.setAttribute('aria-busy','true'); more.disabled=true;
  if(!append&&!quiet)items.innerHTML='<div class="card" role="status">Loading predictions…</div>';
  try {
   const data=await ApiClient.get('/live-feed?'+new URLSearchParams({section,limit:String(pageSize),offset:String(requestOffset),search:form.elements.search.value.trim(),category:form.elements.category.value}));
   if(current!==version)return;
   status.textContent=`${Number(data.playable_count)} open predictions · Virtual Coins only`;
   const next=JSON.stringify(data.items);
   if(append)items.insertAdjacentHTML('beforeend',data.items.map(marketCard).join(''));
   else if(!quiet || next!==signature) { items.innerHTML=data.items.map(marketCard).join(''); signature=next; }
   offset=requestOffset+data.items.length;
   if(!offset)items.innerHTML='<div class="card live-feed-empty"><h3>No predictions here yet.</h3><p>Try another category or explore the newest predictions.</p><button type="button" class="btn btn-outline" data-retry>Refresh</button></div>';
   more.hidden=offset>=Number(data.total);
  } catch(err) {
   if(current!==version)return;
   status.textContent='We couldn’t refresh predictions. Please try again.';
   if(!quiet&&!append)items.innerHTML='<div class="card" role="alert"><h3>Predictions couldn’t load.</h3><button type="button" class="btn btn-outline" data-retry>Try again</button></div>';
  } finally { if(current===version){loading=false;items.setAttribute('aria-busy','false');more.disabled=false;} }
 }

 root.querySelectorAll('[data-section]').forEach(button=>button.addEventListener('click',()=>{section=button.dataset.section;root.querySelectorAll('[data-section]').forEach(b=>b.setAttribute('aria-pressed',String(b===button)));load();}));form.addEventListener('submit',e=>{e.preventDefault();load();});form.elements.category.addEventListener('change',()=>load());items.addEventListener('click',e=>{if(e.target.closest('[data-retry]'))load();});more.addEventListener('click',()=>load(true));

 const live=e=>{if(['news_event_updated','market_live','market_locked','market_resolved','prediction_count_changed','market_activity_changed','market_state_changed'].includes(e.detail.event)){clearTimeout(timer);timer=setTimeout(()=>load(false,true),250);}};window.addEventListener('prophit-live',live);const poll=setInterval(()=>{if(!document.hidden)load(false,true);},10000);window.addEventListener('pagehide',()=>{clearInterval(poll);clearTimeout(timer);window.removeEventListener('prophit-live',live);},{once:true});load();

}
