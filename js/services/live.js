import ApiClient from '../api/client.js';

let socket;
let retry;
let stopped = false;
const events = new Set(['prediction_count_changed','market_activity_changed','market_locked','market_resolved','notification_created','leaderboard_updated','wallet_updated','market_live']);
function connect() {
    if(stopped || !ApiClient.isAuthenticated()) return;
    const host = location.hostname === '127.0.0.1' ? '127.0.0.1' : 'localhost';
    const base = document.querySelector('meta[name="api-base"]')?.content || (['localhost','127.0.0.1'].includes(location.hostname) ? `http://${host}:8080/api` : 'https://profhit.onrender.com/api');
    const url = new URL(`${base}/ws`, location.origin); url.protocol=url.protocol==='https:'?'wss:':'ws:';url.searchParams.set('token',ApiClient.getToken());
    socket = new WebSocket(url);
    socket.onopen=()=>window.dispatchEvent(new CustomEvent('prophit-live-status',{detail:'Connected'}));
    socket.onmessage=message=>{try{const data=JSON.parse(message.data);if(events.has(data.event))window.dispatchEvent(new CustomEvent('prophit-live',{detail:data}));}catch{}};
    socket.onclose=()=>{window.dispatchEvent(new CustomEvent('prophit-live-status',{detail:'Reconnecting'}));if(!stopped)retry=setTimeout(connect,5000);};
}
window.addEventListener('pagehide',()=>{stopped=true;clearTimeout(retry);socket?.close();});
connect();
