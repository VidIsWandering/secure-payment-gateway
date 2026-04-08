const API_BASE = '/api/v1';

const spgAPI = {
  // Authentication & Settings
  getToken() {
    return localStorage.getItem('spg_token');
  },
  
  setToken(token, username) {
    localStorage.setItem('spg_token', token);
    localStorage.setItem('spg_username', username);
  },
  
  clearToken() {
    localStorage.removeItem('spg_token');
    localStorage.removeItem('spg_username');
  },

  getUsername() {
    return localStorage.getItem('spg_username') || 'Merchant';
  },

  // Base API call
  async call(endpoint, options = {}) {
    const headers = {
      'Content-Type': 'application/json',
      ...options.headers
    };

    const token = this.getToken();
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    try {
      const response = await fetch(`${API_BASE}${endpoint}`, {
        ...options,
        headers
      });

      const data = await response.json();

      if (!response.ok) {
        if (response.status === 401 || response.status === 403) {
           this.clearToken();
           // Only redirect if we are strictly requiring auth and not already on auth pages
           if (!window.location.pathname.includes('/login') && !window.location.pathname.includes('/register') && window.location.pathname !== '/') {
              window.location.href = '/login';
           }
        }
        throw new Error(data?.error?.message || data?.message || 'Transaction Failed');
      }

      return data.data; // Return only inner data payload
    } catch (error) {
      throw error;
    }
  },

  // Toast Notification System
  showToast(message, type = 'success') {
    let container = document.getElementById('toast-container');
    if (!container) {
      container = document.createElement('div');
      container.id = 'toast-container';
      container.className = 'toast-container';
      document.body.appendChild(container);
    }

    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;

    container.appendChild(toast);

    // Auto remove
    setTimeout(() => {
      toast.style.animation = 'slideIn 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275) reverse forwards';
      setTimeout(() => toast.remove(), 300);
    }, 3500);
  }
};
