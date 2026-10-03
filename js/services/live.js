import ApiClient from '../api/client.js';

let socket;
let retry;
let stopped = false;
const events = new Set(['prediction_count_changed','market_activity_changed','market_locked','market_resolved','notification_created','leaderboard_updated','wallet_updated','market_live','news_event_updated','market_state_changed']);
function connect() {
    if(stopped || !ApiClient.isAuthenticated()) return;
    const base = document.querySelector('meta[name="api-base"]')?.content || '/api';
    const url = new URL(`${base}/ws`, location.origin); url.protocol=url.protocol==='https:'?'wss:':'ws:';url.searchParams.set('token',ApiClient.getToken());
    socket = new WebSocket(url);
    socket.onopen=()=>window.dispatchEvent(new CustomEvent('prophit-live-status',{detail:'Connected'}));
    socket.onmessage=message=>{try{const data=JSON.parse(message.data);if(events.has(data.event))window.dispatchEvent(new CustomEvent('prophit-live',{detail:data}));}catch{}};
    socket.onclose=()=>{window.dispatchEvent(new CustomEvent('prophit-live-status',{detail:'Reconnecting'}));if(!stopped)retry=setTimeout(connect,5000);};
}
let previous;
let polling = false;
const emit = event => window.dispatchEvent(new CustomEvent('prophit-live', {detail:{event, payload:{}}}));
async function pollState() {
    if(stopped || document.hidden || polling) return;
    polling=true;
    try {
        const next=await ApiClient.get('/live-state');
        if(previous) {
            if(next.public!==previous.public) ['prediction_count_changed','market_activity_changed','news_event_updated','leaderboard_updated'].forEach(emit);
            if(next.lifecycle!==previous.lifecycle)emit('market_state_changed');
            if(next.wallet!==previous.wallet) { emit('wallet_updated'); emit('leaderboard_updated'); }
            if(next.notifications!==previous.notifications) emit('notification_created');
        }
        previous=next;
        window.dispatchEvent(new CustomEvent('prophit-live-status',{detail:'Live updates active'}));
    } catch { window.dispatchEvent(new CustomEvent('prophit-live-status',{detail:'Updates reconnecting'})); }
    finally { polling=false; }
}
const pollTimer=setInterval(pollState,10000);
window.addEventListener('visibilitychange',()=>{if(!document.hidden)pollState();});
window.addEventListener('pagehide',()=>{stopped=true;clearTimeout(retry);clearInterval(pollTimer);socket?.close();});
pollState();
connect();
