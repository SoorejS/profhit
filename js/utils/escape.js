/**
 * Escape HTML to prevent XSS vulnerabilities when inserting user content into the DOM.
 */
export function escapeHTML(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

window.escapeHTML = escapeHTML;
