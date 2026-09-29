export function escapeHTML(value) {
 return String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
}
export function safeURL(value) {
 try { const url = new URL(value, window.location.origin); return ['http:','https:'].includes(url.protocol) ? url.href : ''; } catch { return ''; }
}
window.escapeHTML = escapeHTML;
