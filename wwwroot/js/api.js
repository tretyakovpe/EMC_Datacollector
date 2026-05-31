// Общий API-клиент
const API = {
    async get(endpoint) {
        const response = await fetch(`/api/${endpoint}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        return response.json();
    },
    
    async post(endpoint, data = {}) {
        const response = await fetch(`/api/${endpoint}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        return response.json();
    },
    
    // Специфичные методы
    getLines() {
        return this.get('lines');
    },
    
    getStats() {
        return this.get('stats');
    },
    
    getCurrentBox(lineName) {
        return this.get(`current-box/${encodeURIComponent(lineName)}`);
    },
    
    reprintLabel(labelId) {
        return this.post(`reprint-label/${encodeURIComponent(labelId)}`);
    }
};
