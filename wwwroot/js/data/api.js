const API = {
    async get(endpoint) {
        const response = await fetch(`/api/${endpoint}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        return response.json();
    },
    
    getLines() {
        return this.get('lines');
    },
    
    getStats() {
        return this.get('stats');
    },
    
    getCurrentBox(lineName) {
        return this.get(`current-box/${encodeURIComponent(lineName)}`);
    },
    
    async reprintLabel(labelId) {
        const response = await fetch(`/api/reprint-label/${labelId}`, { method: 'POST' });
        return response.json();
    }
};