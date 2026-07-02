export class ToastManager extends HTMLElement {
    constructor() {
        super();
        this.attachShadow({ mode: 'open' });
        this.shadowRoot.innerHTML = `
            <style>
                :host {
                    position: fixed;
                    bottom: 20px;
                    right: 20px;
                    display: flex;
                    flex-direction: column;
                    gap: 10px;
                    z-index: 9999;
                    pointer-events: none;
                }
                .toast {
                    background-color: var(--bg-surface-elevated, #27272a);
                    color: var(--text-primary, #fff);
                    padding: 12px 20px;
                    border-radius: 8px;
                    border: 1px solid var(--border-strong, #3f3f46);
                    box-shadow: 0 10px 15px -3px rgba(0,0,0,0.5);
                    font-family: inherit;
                    font-size: 0.9rem;
                    transform: translateX(120%);
                    opacity: 0;
                    transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.3s ease;
                    pointer-events: auto;
                }
                .toast.show {
                    transform: translateX(0);
                    opacity: 1;
                }
                .toast.success { border-left: 4px solid var(--color-success, #10b981); }
                .toast.error { border-left: 4px solid var(--color-danger, #ef4444); }
            </style>
            <div id="container"></div>
        `;
    }

    show(message, type = 'default') {
        const container = this.shadowRoot.getElementById('container');
        const el = document.createElement('div');
        el.className = `toast ${type}`;
        el.textContent = message;
        
        container.appendChild(el);
        
        requestAnimationFrame(() => el.classList.add('show'));
        
        setTimeout(() => {
            el.classList.remove('show');
            setTimeout(() => el.remove(), 300);
        }, 3000);
    }
}
customElements.define('toast-manager', ToastManager);

export function showToast(msg, type) {
    let tm = document.querySelector('toast-manager');
    if (!tm) {
        tm = document.createElement('toast-manager');
        document.body.appendChild(tm);
    }
    tm.show(msg, type);
}
