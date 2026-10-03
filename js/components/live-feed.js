import ApiClient from '../api/client.js';

import '../services/live.js';

import { escapeHTML, safeURL } from '../utils/escape.js';
import { age, remaining } from '../utils/time.js';

const categories=['Sports','Weather','Politics','Geopolitics','Technology','Entertainment','Financial Markets','Wild Card'];

function card(m){return `<article class="card live-opportunity" data-market-id="${Number(m.id)}"><div class="live-card-meta"><span class="badge badge-primary">${escapeHTML(m.category)}</span><span>${m.source_kind==='official_event'?'Official future event':escapeHTML(age(m.news_published_at))}</span></div><p class="live-event-title">${escapeHTML(m.news_event_title || (m.is_curated?'Manually curated opportunity':''))}</p><h3><a href="market.html?id=${Number(m.id)}">${escapeHTML(m.title)}</a></h3><p class="text-gold">${escapeHTML(m.difficulty)} · ${Number(m.entry_coins)||0} coins entry · +${Number(m.payout)} reward</p><p class="live-card-meta"><span>${Number(m.volume)} players</span><span>${escapeHTML(remaining(m.lock_time))}</span></p><p class="text-sm">Source: <a href="${escapeHTML(safeURL(m.news_url))}" target="_blank" rel="noopener noreferrer">${escapeHTML(m.news_source_name)}</a></p><details><summary>How this resolves</summary><p>${escapeHTML(m.resolution_rule)}</p><p>Result source: ${escapeHTML(m.resolution_source)}</p><p>Closes ${escapeHTML(new Date(m.lock_time).toLocaleString())}</p><p>${m.news_published_at?'Published '+escapeHTML(new Date(m.news_published_at).toLocaleString()):'Publication date not supplied by the official event source'}; checked ${escapeHTML(new Date(m.news_discovered_at).toLocaleString())}</p></details><a class="btn btn-primary" href="market.html?id=${Number(m.id)}">Predict</a></article>`;}

export function initLiveFeed(root){

 if(!root)return;

 root.innerHTML=`<div class="live-feed-heading"><div><h2>Predict what’s happening in the world.</h2><p>Virtual coins only · No real money</p></div><a href="propose-market.html" class="btn btn-outline">+ Create Prediction</a></div><p class="live-feed-status" role="status" aria-live="polite">Checking current opportunities…</p><div class="live-feed-controls"><div class="live-feed-tabs" aria-label="Feed sections">${[['latest','Latest'],['breaking','Breaking'],['trending','Trending'],['closing_soon','Closing Soon'],['for_you','For You']].map(([key,label])=>`<button type="button" class="btn btn-outline" data-section="${key}" aria-pressed="${key==='latest'}" ${key==='for_you'&&!ApiClient.isAuthenticated()?'disabled title="Sign in for recommendations"':''}>${label}</button>`).join('')}</div><form class="live-feed-search"><label>Search events, questions or sources<input name="search" class="input-control" type="search" maxlength="200" placeholder="Search current events…"></label><label>Category<select name="category" class="input-control"><option value="">All categories</option>${categories.map(c=>`<option>${escapeHTML(c)}</option>`).join('')}</select></label><button type="submit" class="btn btn-secondary">Search</button></form></div><div class="live-feed-items markets-grid" aria-busy="true"></div><button type="button" class="btn btn-outline live-feed-more" hidden>Load more</button>`;

 let section='latest',offset=0,version=0,timer;

 const items=root.querySelector('.live-feed-items'),status=root.querySelector('.live-feed-status'),more=root.querySelector('.live-feed-more'),form=root.querySelector('form'),params=new URLSearchParams(location.search);

 form.elements.search.value=params.get('search')||'';if(categories.includes(params.get('category')))form.elements.category.value=params.get('category');

 async function load(append=false,quiet=false){const pageSize=quiet?Math.min(100,Math.max(60,offset)):60;const current=++version;if(!append)offset=0;items.setAttribute('aria-busy','true');more.disabled=true;if(!append&&!quiet)items.innerHTML='<div class="card" role="status">Loading current opportunities…</div>';

  try{const data=await ApiClient.get('/live-feed?'+new URLSearchParams({section,limit:String(pageSize),offset:String(offset),search:form.elements.search.value.trim(),category:form.elements.category.value}));if(current!==version)return;status.textContent=`${Number(data.playable_count)} current playable predictions · ${Number(data.current_event_count)} fresh news events · Provider: ${data.provider.status}${data.provider.last_success_at?' · Last refreshed '+new Date(data.provider.last_success_at).toLocaleTimeString():''}`;if(!append)items.replaceChildren();items.insertAdjacentHTML('beforeend',data.items.map(card).join(''));offset+=data.items.length;if(!offset)items.innerHTML='<div class="card live-feed-empty"><h3>No current predictions match this feed.</h3><p>News needs a verifiable question, a future cutoff and editorial approval before it becomes playable.</p><button type="button" class="btn btn-outline" data-retry>Refresh feed</button></div>';more.hidden=offset>=Number(data.total);

  }catch(err){if(current!==version)return;status.textContent='Live feed unavailable. Counts cannot currently be verified.';if(!append)items.innerHTML=`<div class="card" role="alert"><h3>We couldn’t load the live feed.</h3><p>${escapeHTML(err.message)}</p><button type="button" class="btn btn-outline" data-retry>Try again</button></div>`;more.hidden=true;}finally{if(current===version){items.setAttribute('aria-busy','false');more.disabled=false;}}}

 root.querySelectorAll('[data-section]').forEach(button=>button.addEventListener('click',()=>{section=button.dataset.section;root.querySelectorAll('[data-section]').forEach(b=>b.setAttribute('aria-pressed',String(b===button)));load();}));form.addEventListener('submit',e=>{e.preventDefault();load();});form.elements.category.addEventListener('change',()=>load());items.addEventListener('click',e=>{if(e.target.closest('[data-retry]'))load();});more.addEventListener('click',()=>load(true));

 const live=e=>{if(['news_event_updated','market_live','market_locked','market_resolved','prediction_count_changed','market_activity_changed'].includes(e.detail.event)){clearTimeout(timer);timer=setTimeout(()=>load(false,true),250);}};window.addEventListener('prophit-live',live);const poll=setInterval(()=>{if(!document.hidden)load(false,true);},10000);window.addEventListener('pagehide',()=>{clearInterval(poll);clearTimeout(timer);window.removeEventListener('prophit-live',live);},{once:true});load();

}
