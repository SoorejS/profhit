/**
 * PROPHIT - Unified API Client
 * Handles HTTP requests, JWT injection, and error catching.
 */

const localAPIHost = window.location.hostname === '127.0.0.1' ? '127.0.0.1' : 'localhost';
const API_URL = document.querySelector('meta[name="api-base"]')?.content || (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
    ? `http://${localAPIHost}:8080/api`
    : 'https://profhit-1.onrender.com/api');

class ApiClient {
    static profileRequest = null;
    static getToken() {
        return localStorage.getItem('token');
    }

    static setToken(token) {
        localStorage.setItem('token', token);
    }

    static removeToken() {
        localStorage.removeItem('token');
    }

    static isAuthenticated() {
        return !!this.getToken();
    }

    static async request(endpoint, options = {}) {
        const headers = {
            'Content-Type': 'application/json',
            ...options.headers
        };

        const token = this.getToken();
        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 25000); // 25s timeout for Render cold-starts

        const config = {
            ...options,
            headers,
            signal: controller.signal
        };

        if (config.body && typeof config.body === 'object') {
            config.body = JSON.stringify(config.body);
        }

        try {
            const response = await fetch(`${API_URL}${endpoint}`, config);
            clearTimeout(timeoutId);
            
            // Handle 401 Unauthorized globally (but not for login itself to prevent loops)
            if (response.status === 401 && !endpoint.startsWith('/auth/')) {
                this.removeToken();
                window.location.href = '/login.html';
                const sessionError=new Error('Session expired. Please log in again.');
                sessionError.status=401;
                throw sessionError;
            }

            let data;
            const textResponse = await response.text();
            try {
                data = JSON.parse(textResponse);
            } catch (parseError) {
                // If it's not JSON, throw a standard HTTP error instead of a JSON SyntaxError
                if (!response.ok) {
                    const error = new Error(`Server Error: ${response.status} ${response.statusText}`);
                    error.status = response.status;
                    throw error;
                }
                data = { message: textResponse };
            }
            
            if (!response.ok) {
                const error = new Error(data.error || data.message || `Request failed (${response.status})`);
                error.status = response.status;
                error.data = data;
                throw error;
            }
            
            return data;
        } catch (error) {
            clearTimeout(timeoutId);
            if (error.name === 'AbortError') {
                console.error(`[API Timeout] ${endpoint}`);
                throw new Error('Server connection timed out. The server may be waking up, please try again in a few seconds.');
            }
            // Expected HTTP failures are rendered by each page. Log only
            // transport/runtime failures so handled validation does not pollute
            // the browser console.
            if (!error.status) console.error(`[API Error] ${endpoint}:`, error);
            throw error;
        }
    }

    static get(endpoint, options = {}) {
        // Sidebar, topbar and page mount together; share only an in-flight read.
        // Never retain the balance after the request completes.
        if (endpoint === '/me') {
            if (!this.profileRequest) {
                this.profileRequest = this.request(endpoint, { ...options, method: 'GET' })
                    .finally(() => { this.profileRequest = null; });
            }
            return this.profileRequest;
        }
        return this.request(endpoint, { ...options, method: 'GET' });
    }

    static post(endpoint, body, options = {}) {
        return this.request(endpoint, { ...options, method: 'POST', body });
    }

    static put(endpoint, body, options = {}) {
        return this.request(endpoint, { ...options, method: 'PUT', body });
    }

    static delete(endpoint, options = {}) {
        return this.request(endpoint, { ...options, method: 'DELETE' });
    }
}

// Export for module usage, or attach to window for global usage
export default ApiClient;
