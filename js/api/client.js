/**
 * PROPHIT - Unified API Client
 * Handles HTTP requests, JWT injection, and error catching.
 */

const API_URL = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
    ? 'http://localhost:8080/api'
    : 'https://profhit.onrender.com/api';

class ApiClient {
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
            if (response.status === 401 && !endpoint.includes('/auth/login')) {
                this.removeToken();
                window.location.href = '/login.html';
                return null;
            }

            let data;
            const textResponse = await response.text();
            try {
                data = JSON.parse(textResponse);
            } catch (parseError) {
                // If it's not JSON, throw a standard HTTP error instead of a JSON SyntaxError
                if (!response.ok) {
                    throw new Error(`Server Error: ${response.status} ${response.statusText}`);
                }
                data = { message: textResponse };
            }
            
            if (!response.ok) {
                throw new Error(data.error || data.message || `Request failed (${response.status})`);
            }
            
            return data;
        } catch (error) {
            clearTimeout(timeoutId);
            if (error.name === 'AbortError') {
                console.error(`[API Timeout] ${endpoint}`);
                throw new Error('Server connection timed out. The server may be waking up, please try again in a few seconds.');
            }
            console.error(`[API Error] ${endpoint}:`, error);
            throw error;
        }
    }

    static get(endpoint, options = {}) {
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
