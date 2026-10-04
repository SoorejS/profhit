import { escapeHTML, safeURL } from '../utils/escape.js';
import { remaining } from '../utils/time.js';

export const categories = ['Politics','Geopolitics','Sports','Technology','Financial Markets','Entertainment','Weather','Wild Card'];
export function marketStatus(m, now = Date.now()) {
    if (m.resolution_status === 'Scheduled' || (['Live','Open'].includes(m.resolution_status) && m.start_time && Date.parse(m.start_time) > now)) return 'Scheduled';
    if (['Live','Open'].includes(m.resolution_status) && Date.parse(m.lock_time || m.end_date) > now) return 'LIVE';
    return ['Live','Open'].includes(m.resolution_status) ? 'Locked' : m.resolution_status;
}
export function formatMarketDate(value) {
    return value ? new Date(value).toLocaleString(undefined, {month:'short', day:'numeric', hour:'numeric', minute:'2-digit', timeZoneName:'short'}) : 'To be announced';
}
export function marketCard(m) {
    const state = marketStatus(m), players = Number(m.volume) || 0;
    return `<article class="card market-card live-opportunity" data-market-id="${Number(m.id)}" data-category="${escapeHTML(m.category)}">
        <div class="market-card-header"><span class="market-card-category">${escapeHTML(m.category)}</span><div class="flex gap-2">${m.is_featured ? '<span class="badge badge-outline">Featured</span>' : ''}<span class="badge ${state==='LIVE'?'badge-primary':'badge-outline'}">${escapeHTML(state)}</span></div></div>
        <h3 class="market-card-title"><a href="market.html?id=${Number(m.id)}">${escapeHTML(m.title)}</a></h3>
        <p class="market-context">${escapeHTML(m.description)}</p>
        <p class="market-source">Source: <a href="${escapeHTML(safeURL(m.news_url || m.resolution_source))}" target="_blank" rel="noopener noreferrer">${escapeHTML(m.news_source_name || 'Official source')}</a></p>
        <dl class="market-dates"><div><dt>Closes</dt><dd>${escapeHTML(formatMarketDate(m.lock_time || m.end_date))}</dd></div><div><dt>Resolution</dt><dd>${escapeHTML(formatMarketDate(m.resolution_time))}</dd></div></dl>
        <div class="market-card-stats"><span>${players ? `${players} ${players===1?'player':'players'}` : 'Be the first to predict'}</span><span>${state==='LIVE'?escapeHTML(remaining(m.lock_time)):''}</span></div>
        <div class="market-coin-row"><span>Cost <strong>${Number(m.entry_coins)||0} Coins</strong></span><span>Reward <strong>${Number(m.payout)} Coins</strong></span></div>
        <a class="btn btn-primary" href="market.html?id=${Number(m.id)}">${state==='LIVE'?'Make prediction':'View prediction'}</a>
    </article>`;
}
